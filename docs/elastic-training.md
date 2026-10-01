# Elastic training (PyTorch, run-to-completion jobs)

**Status: unit-tested only.** Nothing here has run on a cluster, with a real `torchrun`, or on GPUs. Treat every statement about runtime behaviour as intended behaviour until a kind or GPU run says otherwise.

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

## What it does not do

- **It does not add or remove workers while the job runs.** The Indexed Job's `completions` is fixed at creation. Workers lost to a node failure are replaced by the Job controller (same index, same DNS name) when capacity exists; if it does not, the others carry on only if the launcher's rendezvous accepts a smaller group. There is no controller loop that resizes the Job.
- **Losing the rendezvous host (index 0) is not tolerated.** `MASTER_ADDR` is pod 0. Use an external rendezvous (for example etcd) if you need that.
- **The Job can finish early.** Once `minNodes` indexes succeed the remaining pods are removed. In a healthy elastic run all workers finish together; if some finish a moment later they may be stopped mid-exit.
- No resharding of optimizer or data-loader state, no scale-up of a running job, no interaction with Kueue's resize (a Kueue-managed Job cannot change `nodes`).
- Unverified: whether kube-scheduler/Kueue behave as described with a success policy, and any NCCL behaviour on a resized group.

## Tests

`controllers/gryviaaijob_elastic_test.go` (Job shape, env, success policy, defaults, validation), `pkg/scheduler/holds_test.go` (placement between min and max), `pkg/webhook/validator_test.go`.
