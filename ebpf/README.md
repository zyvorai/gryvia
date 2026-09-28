# Gryvia eBPF Programs

Kernel-space eBPF programs that form the data-plane of the Gryvia
network intelligence layer.  They run inside the Linux kernel and feed
structured events to the userspace flow collector.

## Programs

| Program | Hook Type | Purpose |
|---------|-----------|---------|
| `tcp_trace.c` | kprobe (`tcp_v4_connect`, `inet_csk_accept`, `tcp_close`, `tcp_retransmit_skb`) | TCP connection lifecycle tracking -- establishment latency, bytes transferred, retransmits |
| `packet_filter.c` | XDP | Ultra-fast inline packet filtering with dynamic rule maps and per-rule hit counters |
| `latency_probe.c` | kprobe (`tcp_sendmsg`, `tcp_recvmsg`) | Per-connection round-trip latency measurement with histogram buckets |
| `syscall_monitor.c` | raw_tracepoint (`sys_enter`) | Tracks which processes make `connect`, `sendto`, `recvfrom` syscalls; alerts on first-time network activity |
| `dns_tracker.c` | XDP | Passive DNS query/response correlation, resolution latency, NXDOMAIN/SERVFAIL tracking |
| `nccl_trace.c` | uprobe (`ncclAllReduce`, `ncclAllGather`, `ncclBroadcast`, `ncclReduce`, `ncclReduceScatter`, `ncclSend`, `ncclRecv`, `ncclGroupStart`, `ncclGroupEnd`) | NCCL collective communication tracing with per-op histograms and latency distribution |
| `gpu_mem_trace.c` | uprobe (`cudaMemcpy`, `cudaMemcpyAsync`, `cudaMalloc`, `cudaFree`, `cudaLaunchKernel`, `cudaDeviceSynchronize`) | GPU memory transfer monitoring with direction-aware byte counters and allocation tracking |
| `rdma_trace.c` | kprobe (`ib_post_send`, `ib_post_recv`, `ib_poll_cq`), tracepoint (`rdma/rdma_create_qp`, `rdma/rdma_destroy_qp`) | RDMA/InfiniBand traffic analysis with per-QP statistics and completion tracking |
| `training_pattern.c` | uprobe (`ncclAllReduce`), kprobe (`tcp_sendmsg`, `tcp_recvmsg`) | AI training communication pattern detection -- rank communication matrix and compute/comm cycle analysis |
| `datapipe_bottleneck.c` | tracepoint (`block/block_rq_complete`), kprobe (`tcp_recvmsg`), uprobe (`cudaLaunchKernel`, `cudaDeviceSynchronize`) | Data pipeline bottleneck detection -- correlates storage I/O, network ingestion, and GPU busy/idle phases |
| `gradient_compress.c` | uprobe (`ncclAllReduce`) | Gradient compression analysis -- compares expected vs actual bytes per collective to detect compression ratios |

## Prerequisites

- **clang** >= 12 (with BPF target support)
- **llvm** >= 12
- **libbpf-dev** (headers and shared library)
- **bpftool** (optional, for vmlinux.h generation)
- **linux-headers** for the running kernel

On Fedora/RHEL:

```bash
sudo dnf install clang llvm libbpf-devel bpftool kernel-devel
```

On Ubuntu/Debian:

```bash
sudo apt install clang llvm libbpf-dev linux-tools-common linux-headers-$(uname -r)
```

## Building

```bash
make            # compile all programs to .o ELF objects
make clean      # remove compiled objects
```

The Makefile auto-detects the host architecture (`x86` or `arm64`).

## Shared Headers

`headers/common.h` defines structures shared between kernel and userspace for
network tracing:

- `struct flow_event` -- the primary event structure emitted over perf ring buffers
- `struct conn_info` -- per-connection state stored in BPF hash maps

`headers/gpu_common.h` defines structures shared between kernel and userspace
for GPU/AI workload tracing:

- `struct gpu_event` -- GPU event structure emitted via ring buffers (NCCL ops, memory transfers, RDMA, CUDA launches)
- `struct nccl_inflight` -- in-flight NCCL operation tracking for entry/exit latency correlation
- `struct cuda_mem_inflight` -- in-flight CUDA memory operation tracking
- `struct rdma_qp_info` -- per-RDMA queue pair statistics (bytes sent/received, retransmits, completions)
- Enums: `gpu_event_type`, `mem_direction`, `nccl_op_type`

## How It Works

1. The userspace collector (see `../collector/`) loads the compiled `.o` files
   using the cilium/ebpf Go library.
2. Programs are attached to their respective hooks (kprobes, tracepoints, XDP).
3. Events flow from kernel to userspace via `BPF_MAP_TYPE_PERF_EVENT_ARRAY` maps
   (network programs) or `BPF_MAP_TYPE_RINGBUF` maps (GPU/AI programs).
4. BPF hash/array maps provide shared state that the collector can read for
   metrics (histograms, counters, connection tables).

## GPU/AI Program Architecture

The GPU and AI training eBPF programs use **uprobes** to hook into userspace
libraries (NCCL, CUDA runtime) rather than kernel functions.  This requires
special handling in containerized environments:

### Uprobe Resolution for Container Libraries

- NCCL libraries (`libnccl.so`) and CUDA runtime (`libcudart.so`) live inside
  container filesystems, not on the host.
- The userspace collector must resolve the library path within the container's
  mount namespace (e.g., `/proc/<pid>/root/usr/lib/x86_64-linux-gnu/libnccl.so.2`).
- Symbol offsets are determined by reading the ELF symbol table of the target
  library using `bpftool` or the cilium/ebpf library.
- When containers are recreated, uprobe attachments must be refreshed since PIDs
  and library paths change.

### Event Flow

```
  NCCL/CUDA call (userspace)
        |
        v
  uprobe entry --> record start time + params in BPF hash map
        |
  function executes
        |
        v
  uretprobe exit --> compute latency, emit gpu_event via ring buffer
        |
        v
  userspace collector --> Prometheus metrics / alerts
```

### Ring Buffer vs Perf Buffer

GPU/AI programs use `BPF_MAP_TYPE_RINGBUF` instead of `BPF_MAP_TYPE_PERF_EVENT_ARRAY`
for event output.  Ring buffers provide:

- Single shared buffer across all CPUs (simpler userspace consumption)
- `bpf_ringbuf_reserve`/`bpf_ringbuf_submit` for zero-copy event emission
- Better performance for high-frequency GPU events (no per-CPU wakeups)

### Program Interactions

Several GPU programs hook the same functions for different purposes:

- `nccl_trace.c` and `training_pattern.c` both hook `ncclAllReduce`, but
  `nccl_trace` focuses on per-op latency while `training_pattern` analyzes
  rank communication matrices and compute/comm cycles.
- `gpu_mem_trace.c` and `datapipe_bottleneck.c` both track CUDA kernel launches,
  but `datapipe_bottleneck` correlates them with storage I/O and network
  ingestion to detect pipeline stalls.
- `gradient_compress.c` hooks `ncclAllReduce` specifically to compare expected
  vs actual data volumes for compression ratio analysis.
