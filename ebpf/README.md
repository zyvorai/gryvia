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
| `straggler.c` | uprobe (`ncclAllReduce`) | Per-rank NCCL skew -- emits a `fabric_signal` when a collective takes >2x the fastest recent span of the same size (and >5 ms) |
| `rdma_health.c` | kprobe (`ib_post_send`, `mlx5_ib_post_send`, `ib_poll_cq`, `mlx5_ib_poll_cq`) | Per-QP send counts and retry-exceeded / RNR-exceeded completions; emits a `fabric_signal` above operator-set thresholds (all four probes optional) |
| `gds_trace.c` | uprobe (`cuFileRead`, `cuFileWrite`), kprobe (`nvidia_fs_read`, optional) | GPU Direct Storage vs bounce-buffer reads -- byte counters, and a `fabric_signal` for slow (>2 ms) calls |
| `overlap.c` | uprobe/uretprobe (`ncclAllReduce`, `cudaDeviceSynchronize`) | GPU sync while a collective is in flight -- emits `FABRIC_SIG_OVERLAP` when a `cudaDeviceSynchronize` (>= 1 ms) runs nested inside an `ncclAllReduce` on the same thread; per-thread nesting counters, state deleted on every exit path |
| `roce_cnp.c` | XDP | RoCEv2 congestion notification packets (UDP 4791, BTH opcode 0x81; Ethernet, up to 2 VLAN tags, IPv4 or IPv6). Per-CPU counters only (`cnp_count`), always `XDP_PASS`; attached only with `-iface` |
| `infer_latency.c` | kretprobe (`inet_csk_accept`), kprobe (`tcp_recvmsg`) | Accept -> first recv wait of connections to the ports in `infer_ports` (collector `-infer-ports`, default 8000 vLLM and 8001 Triton) as `FABRIC_SIG_INFER_WAIT`; other ports are never recorded |

## Portability (CO-RE)

The programs are **CO-RE** (compile once, run everywhere): they are built without a generated `vmlinux.h`.
`headers/gryvia_core.h` declares the few kernel structs the programs read as minimal local definitions marked
`preserve_access_index`, and every field read goes through `BPF_CORE_READ`. The loader (cilium/ebpf) relocates
those accesses against the running kernel's BTF (`/sys/kernel/btf/vmlinux`) at load time, so one object file works
across kernel versions, and the same sources build for `x86_64` and `arm64` (`make ARCH=arm64`). Per-architecture
syscall numbers are `GRYVIA_NR_*` in the same header.

Requirements on the node: a kernel with BTF (`CONFIG_DEBUG_INFO_BTF=y`, standard on Ubuntu 22.04+, RHEL 9, Debian 12+);
`tcx/*` programs (`cost_tracker`, `trace_correlator`) need Linux 6.6+.

Verified: all 30 programs compile with `-Wall -Werror` (clang 21; the original 24 also with clang 18) and pass the
kernel verifier on Linux 7.0 x86_64; the same sources cross-compile for arm64 (clang 21) but have not been loaded on
an arm64 kernel. Runtime behaviour of the GPU/NCCL/RDMA/GDS programs (including `straggler`, `rdma_health` and
`gds_trace`, `overlap`, `infer_latency`) has not been exercised on GPU, RDMA or GPUDirect Storage hardware; the kprobe symbols they use
(`ib_post_send`, `ib_poll_cq`, `mlx5_ib_*`, `nvidia_fs_read`) are absent on the test host and are skipped by the
collector when missing.

`roce_cnp` was exercised with `BPF_PROG_TEST_RUN` on crafted packets (CNP counted for IPv4, IPv4 with options, VLAN
and IPv6; non-CNP RoCE, other UDP ports, TCP, fragments and truncated frames left alone; always `XDP_PASS`, packet
bytes unchanged; 20000 random frames all `XDP_PASS`), and `overlap` on a private stand-in library with the real
uprobe/uretprobe attach path (nested sync reported, sync alone and sub-millisecond sync silent, no state left behind).
`infer_latency` is load-verified only: attaching it needs the live `inet_csk_accept` / `tcp_recvmsg` hooks.

## Prerequisites

- **clang** >= 12 (with BPF target support), **llvm**, **libbpf-dev**, **linux-libc-dev** (for the `asm/*.h` headers)
- **bpftool** only for `make check`

On Ubuntu/Debian:

```bash
sudo apt install clang llvm libbpf-dev linux-libc-dev make bpftool
```

## Building

```bash
make            # compile all programs to .o ELF objects
make check      # load every object through the verifier with bpftool (needs root; cleans up after itself)
make ARCH=arm64 # cross-compile for arm64
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

`headers/fabric_signal.h` defines the side channel used by `straggler.c`, `rdma_health.c`, `gds_trace.c`, `overlap.c` and
`infer_latency.c` (`roce_cnp.c` only keeps counters)
(`gpu_event` is frozen at 72 bytes and is not extended):

- `struct fabric_signal` -- 80-byte scheduler-facing signal emitted on each program's own `fabric_events` ring buffer
  (straggler, RDMA retry/RNR, GDS, overlap, inference wait). Mirrored by `collector/pkg/fabric/signal.go`; both sides are checked against the same
  offsets (`_Static_assert` in C, a unit test in Go)
- `struct rank_span`, `struct rdma_health_val`, `struct gds_inflight`, `struct overlap_state` -- per-program map values
- `CNP_SLOT_*` -- slots of `roce_cnp`'s per-CPU `cnp_count` array (`FABRIC_SIG_CNP` is synthesised by the collector from it and never appears on a ring)
- Enum: `fabric_signal_type`

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
