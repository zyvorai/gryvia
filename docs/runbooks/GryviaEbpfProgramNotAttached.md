# GryviaEbpfProgramNotAttached

Severity: warning. Rule: [`monitoring/prometheus-rules.yaml`](../../monitoring/prometheus-rules.yaml).

## What fired

`gryvia_ebpf_program_attached == 0` for 5 minutes.

## What it means

An eBPF program in the collector failed to attach on a node, so the signals it feeds are missing there (they read as absent or zero, which can look healthy).

## How to check

```bash
kubectl -n gryvia-network get pods -l app.kubernetes.io/component=collector -o wide
kubectl -n gryvia-network logs <collector-pod> | grep -i -E 'attach|verifier|btf|failed'
gryvia_ebpf_program_attached == 0   # labels object, program, kind say which one
# Same list from the collector:  GET /api/v1/ebpf/status   (signed; see the README)
uname -r; ls /sys/kernel/btf/vmlinux   # on the node
```

To query the collector directly see [Calling a collector](README.md#calling-a-collector).

## Likely causes

- Kernel without BTF or too old for the program; missing kernel config.
- Missing capabilities (CAP_BPF, CAP_PERFMON) or seccomp/AppArmor blocking bpf().
- Program needs hardware absent on the node (RDMA, NVIDIA userspace libraries for uprobes): expected on such nodes.
- Interface/driver without XDP support (PFC/CNP programs).

## Mitigation

- Fix the kernel/capability cause shown in the log, then restart the collector pod.
- If the program is not applicable to the node class, disable it via the chart (`ebpf.*` flags) so it stops alerting.

## Limits (honest)

The gauge shows attach state, not whether the program sees traffic. Verified on one x86 Linux 7.0 test host only; other kernels are untested.
