"""A small elastic data-parallel trainer for `torchrun` on CPU (gloo), used by the kind e2e.

Launch it the way docs/elastic-training.md says, from every pod of an elastic GryviaAIJob:

  torchrun --nnodes=$NNODES --nproc_per_node=$NPROC_PER_NODE --rdzv_backend=c10d \
    --rdzv_endpoint=$MASTER_ADDR:$MASTER_PORT --rdzv_id=$JOB --max_restarts=3 elastic_train.py

It fits y = x @ W on a fixed batch of 8 samples per step, split evenly across the ranks, so the averaged gradient
is the same whatever the world size. Every CHECKPOINT_EVERY steps all ranks commit a coordinated checkpoint
(coordinated_checkpoint.save_global) under CHECKPOINT_DIR. When torchrun restarts the workers after a member was
lost, or the group re-forms with another size, each rank resumes from the last committed step. Rank 0 writes
DONE (JSON: steps, final loss, world size, restarts) next to the checkpoints when training finishes.

Environment: CHECKPOINT_DIR (required), TOTAL_STEPS (80), CHECKPOINT_EVERY (5), STEP_SECONDS (0.5),
COLLECTIVE_TIMEOUT (60 seconds).
"""
from datetime import timedelta
import io
import json
import os
from pathlib import Path
import time

import torch
import torch.distributed as dist

import coordinated_checkpoint as cc

BATCH = 8
TRUE_W = torch.tensor([[1.0, -2.0, 3.0, 0.5]])


def batch(step):
    g = torch.Generator().manual_seed(step)
    x = torch.randn(BATCH, 4, generator=g)
    return x, x @ TRUE_W.T


def main():
    root = os.environ['CHECKPOINT_DIR']
    total = int(os.environ.get('TOTAL_STEPS', '80'))
    every = int(os.environ.get('CHECKPOINT_EVERY', '5'))
    pause = float(os.environ.get('STEP_SECONDS', '0.5'))
    restarts = int(os.environ.get('TORCHELASTIC_RESTART_COUNT', '0'))
    # a peer that disappears mid all-reduce only surfaces as a collective timeout (30 min by default)
    collective_timeout = int(os.environ.get('COLLECTIVE_TIMEOUT', '60'))

    dist.init_process_group('gloo', timeout=timedelta(seconds=collective_timeout))
    rank, world = dist.get_rank(), dist.get_world_size()
    if BATCH % world:
        raise SystemExit('world size %d does not divide the batch of %d' % (world, BATCH))
    Path(root).mkdir(parents=True, exist_ok=True)
    if rank == 0:
        dropped = cc.discard_uncommitted(root)
        if dropped:
            print('discarded uncommitted steps %s' % dropped, flush=True)
    dist.barrier()

    torch.manual_seed(0)
    model = torch.nn.Linear(4, 1, bias=False)
    opt = torch.optim.SGD(model.parameters(), lr=0.1)
    start = 0
    found = cc.load_replicated(root)
    if found:
        payload, start = found
        state = torch.load(io.BytesIO(payload))
        model.load_state_dict(state['model'])
        opt.load_state_dict(state['opt'])
    print('rank %d of %d: starting after step %d (restart %d)' % (rank, world, start, restarts), flush=True)

    loss = None
    for step in range(start + 1, total + 1):
        x, y = batch(step)
        xs, ys = x.chunk(world)[rank], y.chunk(world)[rank]
        opt.zero_grad()
        torch.nn.functional.mse_loss(model(xs), ys).backward()
        for p in model.parameters():
            dist.all_reduce(p.grad)
            p.grad /= world
        opt.step()
        with torch.no_grad():
            loss = torch.nn.functional.mse_loss(model(x), y).item()
        if step % every == 0:
            buf = io.BytesIO()
            torch.save({'model': model.state_dict(), 'opt': opt.state_dict()}, buf)
            cc.save_global(root, buf.getvalue(), step, rank, world, timeout=60)
            if rank == 0:
                cc.prune(root, keep=2)
                print('committed step %d (world %d, loss %.6f)' % (step, world, loss), flush=True)
        time.sleep(pause)

    dist.barrier()
    if rank == 0:
        done = {'steps': total, 'loss': loss, 'world': world, 'restarts': restarts, 'resumedFrom': start}
        Path(root, 'DONE').write_text(json.dumps(done))
        print('done: %s' % json.dumps(done), flush=True)
    dist.destroy_process_group()


if __name__ == '__main__':
    main()
