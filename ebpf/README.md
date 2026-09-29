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
| `ucx_gloo.c` | uprobe/uretprobe (`ucp_tag_send_nb`, `ucp_tag_send_nbx` in `libucp.so`) | UCX tag-send calls that blocked for >= 5 ms as `FABRIC_SIG_UCX_SLOW` (span of the posting call, not of the transfer). Skipped when `libucp.so` is not found (`-ucx-lib`, `-uprobe-pid`, standard dirs). **Gloo is not probed**: its C++ entry points are mangled, version-specific and usually linked statically into `libtorch_cpu.so`, so a fixed-symbol probe could never attach |
| `pfc_pause.c` | XDP | 802.1Qbb PFC pause frames (EtherType 0x8808, opcode 0x0101; per-priority counts from the enable vector and quanta) and 802.3x pause frames in per-CPU counters (`pause_count`); always `XDP_PASS`, attached only with `-iface`. **One XDP program per interface**: conflicts with `roce_cnp`, `packet_filter` and `dns_tracker`; the collector attaches the first and skips the rest with a logged reason. Many NICs consume pause frames in the MAC and never show them to XDP |
| `weight_exfil.c` | kprobe (`vfs_read`, `tcp_v4_connect`) | Observe only. A read request >= 8 MiB from a file named `*.safetensors/.gguf/.ckpt/.onnx/.pt/.pth/.bin/.h5`, then a `tcp_v4_connect` by the same process within 30 s to a destination that is not loopback, RFC1918, link-local or 0.0.0.0/8, emits one `FABRIC_SIG_EXFIL`. A read alone never fires. IPv4 only, `read()` only (not mmap), name-based |
| `quota_pace.c` | sockops (cgroup v2) | **The only mutating program. Off by default.** Lowers `SO_MAX_PACING_RATE` of an outbound TCP connection (`TCP_CONNECT_CB`) when `pace_rate[<full 64-bit cgroup id>]` has an entry; no entry means the socket is untouched. Rate clamped up to 1 Mbit/s, never raised, fail open. Attached only with `-quota-pace` **and** `-cgroup-path`; entries are lease-gated by `collector/pkg/fabric` (`Pacer`), but nothing grants leases yet |

## Portability (CO-RE)

The programs are **CO-RE** (compile once, run everywhere): they are built without a generated `vmlinux.h`.
`headers/gryvia_core.h` declares the few kernel structs the programs read as minimal local definitions marked
`preserve_access_index`, and every field read goes through `BPF_CORE_READ`. The loader (cilium/ebpf) relocates
those accesses against the running kernel's BTF (`/sys/kernel/btf/vmlinux`) at load time, so one object file works
across kernel versions, and the same sources build for `x86_64` and `arm64` (`make ARCH=arm64`). Per-architecture
syscall numbers are `GRYVIA_NR_*` in the same header.

Requirements on the node: a kernel with BTF (`CONFIG_DEBUG_INFO_BTF=y`, standard on Ubuntu 22.04+, RHEL 9, Debian 12+);
`tcx/*` programs (`cost_tracker`, `trace_correlator`) need Linux 6.6+.

Verified: all 34 programs compile with `-Wall -Werror` (clang 21; the original 24 also with clang 18) and pass the
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

`pfc_pause` was exercised with `BPF_PROG_TEST_RUN` (PFC frames with per-priority counts, XON quanta, legacy pause, other
ethertypes, truncated frames, 20000 random frames: all `XDP_PASS`, bytes unchanged). `ucx_gloo` ran on a private
stand-in `libucp.so` with the real uprobe/uretprobe attach path. `weight_exfil` was attached for a few seconds to the
live `vfs_read` / `tcp_v4_connect` hooks in a test process (read alone silent, internal destinations silent, external
destination fires once, name and threshold edges) and detached again. `quota_pace` was run on a throwaway cgroup tree
(never the root cgroup): no entry leaves the socket unchanged, an entry clamps it, a low rate is raised to the floor, 0 and
out-of-range values change nothing, an already lower limit is kept, a cgroup without an entry is untouched next to one
with an entry, an entry for the parent does not cover its children, and a deleted entry leaves the next connection
unchanged. None of this has run on GPU, RDMA or RoCE hardware; whether pause frames reach XDP depends on the NIC.

`quota_pace` is the only program that changes a socket. Its map (`pace_rate`) is empty until userspace inserts a key, so
loading it alone changes nothing; the loader skips it unless `-quota-pace` and `-cgroup-path` are both set. Limits: only
outbound connections are paced (the ESTABLISHED callbacks run in softirq where the current task is unrelated, and
sockops programs cannot ask for a socket's cgroup); the key is the exact cgroup id (the directory's inode number), not
its descendants; the kernel takes the rate as 32-bit bytes per second (max about 4.29 GB/s); a socket that is already
paced keeps its rate until it closes, deleting the entry only unpaces new connections.

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

`headers/fabric_signal.h` defines the side channel used by `straggler.c`, `rdma_health.c`, `gds_trace.c`, `overlap.c`,
`infer_latency.c`, `ucx_gloo.c` and `weight_exfil.c` (`roce_cnp.c` and `pfc_pause.c` only keep counters)
(`gpu_event` is frozen at 72 bytes and is not extended):

- `struct fabric_signal` -- 80-byte scheduler-facing signal emitted on each program's own `fabric_events` ring buffer
  (straggler, RDMA retry/RNR, GDS, overlap, inference wait). Mirrored by `collector/pkg/fabric/signal.go`; both sides are checked against the same
  offsets (`_Static_assert` in C, a unit test in Go)
- `struct rank_span`, `struct rdma_health_val`, `struct gds_inflight`, `struct overlap_state` -- per-program map values
- `CNP_SLOT_*` -- slots of `roce_cnp`'s per-CPU `cnp_count` array (`FABRIC_SIG_CNP` is synthesised by the collector from it and never appears on a ring)
- `PFC_SLOT_*` -- slots of `pfc_pause`'s per-CPU `pause_count` array (frames, legacy pause, one per paused priority; `FABRIC_SIG_PFC` is synthesised by the collector and never appears on a ring)
- `struct exfil_mark` -- `weight_exfil`'s per-process record of recent large model-file reads
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
