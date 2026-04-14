# TensorReaper eBPF Programs

Kernel-space eBPF programs that form the data-plane of the TensorReaper
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

## Shared Header

`headers/common.h` defines structures shared between kernel and userspace:

- `struct flow_event` -- the primary event structure emitted over perf ring buffers
- `struct conn_info` -- per-connection state stored in BPF hash maps

## How It Works

1. The userspace collector (see `../collector/`) loads the compiled `.o` files
   using the cilium/ebpf Go library.
2. Programs are attached to their respective hooks (kprobes, tracepoints, XDP).
3. Events flow from kernel to userspace via `BPF_MAP_TYPE_PERF_EVENT_ARRAY` maps.
4. BPF hash/array maps provide shared state that the collector can read for
   metrics (histograms, counters, connection tables).
