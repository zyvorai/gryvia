# Slurm batch scripts

`gryvia submit --sbatch` runs a Slurm batch script as a `GryviaAIJob`. The `#SBATCH` directives become spec fields,
the script becomes the container command, and the `SLURM_*` variables a script usually reads are set in every pod.
There is no Slurm controller involved: the job is scheduled by Gryvia (and Kueue, when the integration is on) like
any other `GryviaAIJob`.

```bash
gryvia submit --sbatch train.sh --image pytorch/pytorch:2.4.0-cuda12.1-cudnn9-runtime --dry-run   # print the YAML
gryvia submit --sbatch train.sh --image pytorch/pytorch:2.4.0-cuda12.1-cudnn9-runtime --wait --logs
gryvia -n tenant-alpha submit --sbatch sweep.sh --image ghcr.io/acme/train:1.2
```

`--dry-run` prints the jobs as YAML on stdout and the warnings on stderr, so
`gryvia submit --sbatch train.sh --image ... --dry-run | kubectl apply -f -` works too. Slurm has no container image,
so pass `--image` or add `#SBATCH --container-image=<image>` (the pyxis directive; `nvcr.io#nvidia/pytorch:24.01-py3`
becomes `nvcr.io/nvidia/pytorch:24.01-py3`). `--image` wins when both are given.

## Directive mapping

| `#SBATCH` | GryviaAIJob |
| --- | --- |
| `-J`, `--job-name` | `metadata.name` (lower case, other characters become `-`); the original is `SLURM_JOB_NAME`. Without it, the file name |
| `-p`, `--partition` | `spec.queueName`, the Kueue LocalQueue (used only with the Kueue integration, see [Kueue integration](kueue-integration.md)) |
| `-N`, `--nodes=N` | `distributed: {enabled: true, nodes: N}`, one pod per node; CPU-only jobs get `backend: gloo` |
| `--nodes=min-max` | `distributed.nodes: max` with `elastic.minNodes: min` ([elastic training](elastic-training.md)) |
| `-n`, `--ntasks` without `--nodes` | one pod per task (`ceil(ntasks / ntasks-per-node)` pods) |
| `--gres=gpu[:type][:N]`, `--gpus-per-node=[type:]N` | `gpus: N` per pod, `gpuType` upper case (`a100`, `tesla_a100` become `A100`) |
| `-G`, `--gpus=[type:]N` | `gpus: ceil(N / nodes)` per pod |
| `--gpus-per-task=N` | `gpus: N × ntasks-per-node` |
| `-c`, `--cpus-per-task`, `--cpus-per-gpu` | `resources.requests.cpu` (per task × tasks per node, or per GPU × GPUs) |
| `--mem`, `--mem-per-cpu`, `--mem-per-gpu` | `resources.requests.memory` and `limits.memory` (Slurm's MB default; K/M/G/T become Ki/Mi/Gi/Ti) |
| `-t`, `--time` | `timeout` (`MM`, `MM:SS`, `HH:MM:SS`, `D-HH[:MM[:SS]]`; `UNLIMITED` and `0` set none) |
| `-a`, `--array=0-3,7,10-20:5` | one job per index, named `<name>-<index>`, labelled `gryvia.io/sbatch-array=<name>`, at most 256 |
| `-D`, `--chdir` | `workingDir` |
| `--container-image` | `image` (unless `--image` is given) |

Directive parsing stops at the first command line, like `sbatch`. Every job is labelled
`gryvia.io/imported-from: sbatch`.

These print a warning and are otherwise ignored: `--output`/`--error` (read the output with `gryvia logs`),
`--dependency` (chain jobs with a [GryviaWorkflow](../website/docs/guides/ML_WORKFLOWS.md)), `--signal` (pods get
SIGTERM on eviction; see the checkpoint hooks in [admission and recovery](admission-recovery.md)), `--exclusive`,
`--export`, an array's `%limit` (all indexes are submitted at once; a Kueue queue can hold them), `--gres` resources
other than GPUs, `--mem=0`, and any directive without an equivalent (`--account`, `--qos`, `--constraint`,
`--nodelist`, `--mail-*`, ...). A malformed value (`--nodes=two`, `--time=later`) is an error and nothing is
submitted.

## What the script sees

| Variable | Value |
| --- | --- |
| `SLURM_PROCID`, `SLURM_NODEID` | The pod index (`RANK` of a distributed job, else `JOB_COMPLETION_INDEX`) |
| `SLURM_LOCALID` | `0` |
| `SLURM_NTASKS`, `SLURM_NPROCS`, `SLURM_NNODES`, `SLURM_JOB_NUM_NODES` | The number of pods |
| `SLURM_NTASKS_PER_NODE`, `SLURM_TASKS_PER_NODE` | `1`, `1(x<pods>)` |
| `SLURM_JOB_ID` | The GryviaAIJob name (Slurm's is a number; scripts that only use it in file names are fine) |
| `SLURM_JOB_NAME`, `SLURM_JOB_PARTITION` | From the directives |
| `SLURM_ARRAY_TASK_ID`, `SLURM_ARRAY_JOB_ID`, `SLURM_ARRAY_TASK_COUNT`, `_MIN`, `_MAX` | Array jobs only |
| `SLURM_CPUS_PER_TASK`, `SLURM_CPUS_ON_NODE`, `SLURM_GPUS_ON_NODE`, `SLURM_MEM_PER_NODE` | When requested |
| `SLURM_JOB_NODELIST`, `SLURM_NODELIST` | The rendezvous host (pod 0); `MASTER_ADDR` and `MASTER_PORT` are set too |
| `SLURM_SUBMIT_DIR` | The working directory |

Two commands are defined as shell functions before the script runs:

- `srun [options] cmd ...` drops its options and runs `cmd` once in the pod. Each pod already is one task, so
  `srun python train.py` in a 4-node script runs `train.py` once on each of the 4 pods, as Slurm would with one task
  per node.
- `scontrol show hostnames` prints the rendezvous host, so the common
  `MASTER_ADDR=$(scontrol show hostnames "$SLURM_JOB_NODELIST" | head -n1)` gives pod 0. Other `scontrol` calls fail
  with a message.

The script runs with its own interpreter from the `#!` line (a POSIX shell such as `sh` or `bash`, or `python`;
others are refused). For a python script the variables are set by `/bin/sh` first, so the image needs `/bin/sh`.

## Limits

- One task per pod. With `--ntasks-per-node` above 1 (and not equal to the GPUs per node), start the processes
  yourself, for example `torchrun --nproc-per-node=$SLURM_GPUS_ON_NODE`; the importer warns.
- No job steps, `sacct`, `squeue`, `scancel`, heterogeneous jobs or `--dependency`. Use `gryvia list jobs`,
  `gryvia logs`, `gryvia cancel` and workflows.
- `SLURM_JOB_NODELIST` holds one host (pod 0), not every node.
- Partitions are not created; a partition name is only used as the Kueue LocalQueue name.

## What is verified

| Verified | How |
| --- | --- |
| Each directive's mapping, time, memory, GPU and array formats, name sanitizing, warnings for ignored and unknown directives, errors for malformed values, interpreters, and the shims run under `/bin/sh` (`SLURM_PROCID` from `RANK` or `JOB_COMPLETION_INDEX`, `srun` option stripping, `scontrol show hostnames`) | Unit tests in `cli/src/commands/sbatch.rs` |
| A 2-node busybox script submitted with `gryvia submit --sbatch --wait` on kind: it Succeeds, the logs show `SLURM_PROCID` 0 and 1, `SLURM_NTASKS=2`, `srun` running once per pod and `scontrol show hostnames` returning pod 0; a 2-index array runs `sweep-0` and `sweep-1` with their `SLURM_ARRAY_*` values | `scripts/e2e-jobs.sh sbatch` in `.github/workflows/e2e-jobs.yml` |

Not verified: GPU scripts (CI has no GPU) and real Slurm job scripts from a site. This importer does not run Slurm;
the jobs never reach a Slurm controller.
