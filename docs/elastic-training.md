# Elastic training (PyTorch, run-to-completion jobs)

**Status: run on kind with a real `torchrun` on CPU; never on GPUs.** The kind e2e (`e2e-ml.yml`, "Elastic training") runs [`elastic_train.py`](../examples/training/elastic_train.py) in an elastic AIJob (`nodes: 2`, `minNodes: 1`, gloo), deletes worker index 1 after a committed checkpoint and expects torchrun to restart the workers, every rank to resume from the committed step and the job to succeed. NCCL, GPUs, multi-node kind clusters and Kueue are not covered.

## What it is

```yaml
spec:
  type: training
  distributed:
    enabled: true
    framework: pytorch
    nodes: 4            # the most workers wanted (and what quota / admission count)
    gpusPerNode: 8
    elastic:
      minNodes: 2       # the fewest the training can run with
```

For such a job the operator:

- creates the Indexed batch Job with `completions = parallelism = nodes`;
- gives the launcher `NNODES=2:4` (the form `torchrun --nnodes` accepts) plus `GRYVIA_ELASTIC=true`, `GRYVIA_ELASTIC_MIN_NODES` and `GRYVIA_ELASTIC_MAX_NODES`. `WORLD_SIZE` stays the upper bound; the launcher recomputes it after each rendezvous;
- sets a Job `successPolicy` with `succeededCount: minNodes`, so the Job completes when `minNodes` workers succeeded even if the others never got scheduled (without it a Pending worker would keep the Job from ever completing). Needs Kubernetes with `batch/v1` Job success policy (beta and on by default in 1.31, GA in 1.33). With `minNodes == nodes` no policy is set;
- places the job as soon as `minNodes` nodes qualify and takes as many as are free up to `nodes`.

Validation (webhook and controller): `1 <= minNodes <= nodes`, PyTorch only, batch Job only (an inference StatefulSet has a fixed replica set). A bad spec fails the job with `InvalidElasticConfig`.

Your training script must be elastic itself: launch it with `torchrun --nnodes=$NNODES --nproc-per-node=$NPROC_PER_NODE --rdzv-backend=c10d --rdzv-endpoint=$MASTER_ADDR:$MASTER_PORT ...`, restart from a checkpoint on each membership change, and use [coordinated checkpoints](../examples/training/coordinated_checkpoint.py) so all ranks resume the same step.

## Reference trainer

[`examples/training/elastic_train.py`](../examples/training/elastic_train.py) (image: `examples/training/Dockerfile.elastic`, CPU torch) is a small data-parallel trainer that does all three:

- every `CHECKPOINT_EVERY` steps all ranks call `save_global`; a step counts only once rank 0 wrote its COMMIT record;
- on (re)start rank 0 deletes half-written steps above the committed one (`discard_uncommitted`), then every rank loads rank 0's blob of the committed step (`load_replicated`), so the group may resume with another world size;
- the process group gets a short timeout (`COLLECTIVE_TIMEOUT`, default 60s). A peer that disappears in the middle of an all-reduce otherwise only surfaces after gloo's default of 30 minutes, and torchrun cannot restart the workers before that.

```yaml
spec:
  type: training
  gpus: 0
  image: ghcr.io/zyvorai/gryvia-elastic-train:dev
  retryLimit: 3                     # the replacement of a lost worker counts as a retry
  distributed: {enabled: true, framework: pytorch, backend: gloo, nodes: 2, elastic: {minNodes: 1}}
  command: [torchrun]
  args: [--nnodes=$(NNODES), --nproc_per_node=$(NPROC_PER_NODE), --rdzv_backend=c10d,
         "--rdzv_endpoint=$(MASTER_ADDR):$(MASTER_PORT)", --rdzv_id=my-job, --max_restarts=3, /app/elastic_train.py]
  env: [{name: CHECKPOINT_DIR, value: /ckpt}]
  volumes: [{name: ckpt, persistentVolumeClaim: {claimName: my-checkpoints}}]   # ReadWriteMany across nodes
  volumeMounts: [{name: ckpt, mountPath: /ckpt}]
```

What the kind e2e checks: after worker 1 is deleted, the survivor's all-reduce fails, torchrun restarts its worker, the group re-forms (alone, or with the replacement pod the Job creates for index 1) and training continues from the last committed step. Rank 0 writes `DONE` (steps, final loss, world size, restarts, resumed step) next to the checkpoints.

## What it does not do

- **It does not add or remove workers while the job runs.** The Indexed Job's `completions` is fixed at creation. Workers lost to a node failure are replaced by the Job controller (same index, same DNS name) when capacity exists; if it does not, the others carry on only if the launcher's rendezvous accepts a smaller group. There is no controller loop that resizes the Job.
- **Losing the rendezvous host (index 0) is not tolerated.** `MASTER_ADDR` is pod 0. Use an external rendezvous (for example etcd) if you need that.
- **The Job can finish early.** Once `minNodes` indexes succeed the remaining pods are removed. In a healthy elastic run all workers finish together; if some finish a moment later they may be stopped mid-exit.
- No resharding of optimizer or data-loader state, no scale-up of a running job, no interaction with Kueue's resize (a Kueue-managed Job cannot change `nodes`).
- Unverified: Kueue with a success policy, workers spread over several nodes, and any NCCL behaviour on a resized group.

## Tests

`controllers/gryviaaijob_elastic_test.go` (Job shape, env, success policy, defaults, validation), `pkg/scheduler/holds_test.go` (placement between min and max), `pkg/webhook/validator_test.go`, `examples/training/test_coordinated_checkpoint.py` (commit protocol, `load_replicated`, `discard_uncommitted`), and the "Elastic training" step of `e2e-ml.yml`.
