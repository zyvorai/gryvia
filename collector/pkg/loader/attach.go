package loader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

// AttachKind identifies which link.* call is used for a program.
type AttachKind string

const (
	KindKprobe        AttachKind = "kprobe"
	KindKretprobe     AttachKind = "kretprobe"
	KindUprobe        AttachKind = "uprobe"
	KindUretprobe     AttachKind = "uretprobe"
	KindTracepoint    AttachKind = "tracepoint"
	KindRawTracepoint AttachKind = "raw_tracepoint"
	KindTPBTF         AttachKind = "tp_btf"
	KindXDP           AttachKind = "xdp"
	KindTCXIngress    AttachKind = "tcx_ingress"
	KindTCXEgress     AttachKind = "tcx_egress"
	KindSockOps       AttachKind = "sockops"
	KindSkMsg         AttachKind = "sk_msg"
)

// AttachSpec is the parsed form of an ELF section name.
type AttachSpec struct {
	Kind   AttachKind
	Symbol string // kernel/user symbol, tracepoint name, or btf tracepoint
	Group  string // tracepoint group (KindTracepoint only)
}

// ParseSection converts an ELF section name such as "kprobe/tcp_v4_connect"
// or "tracepoint/tcp/tcp_probe" into an AttachSpec.
func ParseSection(section string) (AttachSpec, error) {
	section = strings.TrimSpace(section)
	head, rest, hasRest := strings.Cut(section, "/")

	need := func(k AttachKind) (AttachSpec, error) {
		if !hasRest || rest == "" {
			return AttachSpec{}, fmt.Errorf("section %q: missing symbol", section)
		}
		return AttachSpec{Kind: k, Symbol: rest}, nil
	}

	switch head {
	case "kprobe", "ksyscall":
		return need(KindKprobe)
	case "kretprobe", "kretsyscall":
		return need(KindKretprobe)
	case "uprobe":
		return need(KindUprobe)
	case "uretprobe":
		return need(KindUretprobe)
	case "raw_tracepoint", "raw_tp":
		return need(KindRawTracepoint)
	case "tp_btf":
		return need(KindTPBTF)
	case "tracepoint", "tp":
		if !hasRest {
			return AttachSpec{}, fmt.Errorf("section %q: missing tracepoint", section)
		}
		group, name, ok := strings.Cut(rest, "/")
		if !ok || group == "" || name == "" {
			return AttachSpec{}, fmt.Errorf("section %q: want tracepoint/<group>/<name>", section)
		}
		return AttachSpec{Kind: KindTracepoint, Group: group, Symbol: name}, nil
	case "xdp":
		return AttachSpec{Kind: KindXDP}, nil
	case "tcx":
		switch rest {
		case "ingress":
			return AttachSpec{Kind: KindTCXIngress}, nil
		case "egress":
			return AttachSpec{Kind: KindTCXEgress}, nil
		}
		return AttachSpec{}, fmt.Errorf("section %q: want tcx/ingress or tcx/egress", section)
	case "sockops":
		return AttachSpec{Kind: KindSockOps}, nil
	case "sk_msg":
		return AttachSpec{Kind: KindSkMsg}, nil
	}
	return AttachSpec{}, fmt.Errorf("unsupported section %q", section)
}

// Config controls which config-dependent program types get attached.
type Config struct {
	Dir        string // directory with compiled .o files
	Iface      string // interface for XDP/TCX; empty disables them
	CgroupPath string // cgroup v2 path for sockops/sk_msg; empty disables them
	NCCLLib    string // path to libnccl.so; empty = auto-discover
	CUDALib    string // path to libcudart.so; empty = auto-discover
	CuFileLib  string // path to libcufile.so (GPUDirect Storage); empty = auto-discover
	UprobePID  int    // if >0, find libraries via /proc/<pid>/maps
}

// SkipReason returns a non-empty explanation when the program described by
// spec must not be attached with the given configuration.
func SkipReason(spec AttachSpec, cfg Config) string {
	switch spec.Kind {
	case KindXDP, KindTCXIngress, KindTCXEgress:
		if cfg.Iface == "" {
			return "no network interface configured (set -iface)"
		}
	case KindSockOps, KindSkMsg:
		if cfg.CgroupPath == "" {
			return "no cgroup path configured (set -cgroup-path)"
		}
	case KindUprobe, KindUretprobe:
		if LibraryFor(spec.Symbol, cfg) == "" {
			return "no library found for symbol " + spec.Symbol + " (set -nccl-lib/-cuda-lib/-cufile-lib or -uprobe-pid)"
		}
	}
	return ""
}

// LibraryFor picks the configured library for a user-space symbol.
func LibraryFor(symbol string, cfg Config) string {
	switch {
	case strings.HasPrefix(symbol, "nccl"):
		return cfg.NCCLLib
	case strings.HasPrefix(symbol, "cuda"):
		return cfg.CUDALib
	case strings.HasPrefix(symbol, "cuFile"):
		return cfg.CuFileLib
	}
	return ""
}

// defaultLibDirs are searched when no library was configured.
var defaultLibDirs = []string{
	"/usr/lib/x86_64-linux-gnu", "/usr/lib/aarch64-linux-gnu", "/usr/lib64", "/usr/lib",
	"/usr/local/lib", "/usr/local/cuda/lib64", "/usr/local/cuda/targets/x86_64-linux/lib",
	"/opt/nccl/lib",
}

// findLibInDirs returns the first existing file whose name starts with
// prefix (e.g. "libnccl.so") in dirs, preferring the shortest name.
func findLibInDirs(dirs []string, prefix string) string {
	for _, d := range dirs {
		matches, _ := filepath.Glob(filepath.Join(d, prefix+"*"))
		sort.Slice(matches, func(i, j int) bool { return len(matches[i]) < len(matches[j]) })
		for _, m := range matches {
			if st, err := os.Stat(m); err == nil && st.Mode().IsRegular() {
				return m
			}
		}
	}
	return ""
}

// ResolveLibraries fills NCCLLib/CUDALib/CuFileLib when unset, first from the process
// given by UprobePID, then from standard library directories.
func ResolveLibraries(cfg Config, res *UprobeResolver, dirs []string) Config {
	if dirs == nil {
		dirs = defaultLibDirs
	}
	if cfg.NCCLLib == "" && cfg.UprobePID > 0 && res != nil {
		if p, err := res.FindNCCLLibrary(cfg.UprobePID); err == nil {
			cfg.NCCLLib = p
		}
	}
	if cfg.CUDALib == "" && cfg.UprobePID > 0 && res != nil {
		if p, err := res.FindCUDALibrary(cfg.UprobePID); err == nil {
			cfg.CUDALib = p
		}
	}
	if cfg.CuFileLib == "" && cfg.UprobePID > 0 && res != nil {
		if p, err := res.FindCuFileLibrary(cfg.UprobePID); err == nil {
			cfg.CuFileLib = p
		}
	}
	if cfg.NCCLLib == "" {
		cfg.NCCLLib = findLibInDirs(dirs, "libnccl.so")
	}
	if cfg.CUDALib == "" {
		cfg.CUDALib = findLibInDirs(dirs, "libcudart.so")
	}
	if cfg.CuFileLib == "" {
		cfg.CuFileLib = findLibInDirs(dirs, "libcufile.so")
	}
	return cfg
}

// MapClass says which decoder consumes a map's events.
type MapClass string

const (
	ClassFlow     MapClass = "flow"
	ClassGPU      MapClass = "gpu"
	ClassSecurity MapClass = "security"
	ClassFabric   MapClass = "fabric"
	ClassNone     MapClass = ""
)

var mapClasses = map[string]MapClass{
	// perf event arrays carrying struct flow_event
	"events":         ClassFlow, // tcp_trace
	"latency_events": ClassFlow, // latency_probe
	// ring buffers carrying struct gpu_event
	"nccl_events":     ClassGPU,
	"cuda_events":     ClassGPU,
	"rdma_events":     ClassGPU,
	"pattern_events":  ClassGPU,
	"pipeline_events": ClassGPU,
	"grad_events":     ClassGPU,
	// ring buffers carrying struct fabric_signal (straggler, rdma_health, gds_trace)
	"fabric_events": ClassFabric,
	// ring buffers carrying struct security_event
	"escape_events":  ClassSecurity,
	"mining_events":  ClassSecurity,
	"exfil_events":   ClassSecurity,
	"privesc_events": ClassSecurity,
	"fim_events":     ClassSecurity,
}

// ClassifyMap returns the decoder class for an event map and whether a
// reader should be opened for it. Flow maps must be perf arrays; GPU, fabric
// and security maps must be ring buffers. Maps with no decoder yet
// (syscall_events, dns_events, connpool_events, ...) return ClassNone.
func ClassifyMap(name string, t ebpf.MapType) MapClass {
	c, ok := mapClasses[name]
	if !ok {
		return ClassNone
	}
	switch {
	case c == ClassFlow && t == ebpf.PerfEventArray:
		return c
	case (c == ClassGPU || c == ClassSecurity || c == ClassFabric) && t == ebpf.RingBuf:
		return c
	}
	return ClassNone
}

// IsMissingSymbol reports whether an attach error for the given kind means the
// target symbol does not exist: a kprobe on a function this kernel (or a
// not-loaded module) does not have, or a uprobe on a symbol the library does
// not export. Optional hooks such as mlx5_ib_post_send or nvidia_fs_read are
// skipped, not failed. A missing library file is a real error, not a skip.
func IsMissingSymbol(kind AttachKind, err error) bool {
	switch kind {
	case KindKprobe, KindKretprobe:
		return errors.Is(err, os.ErrNotExist)
	case KindUprobe, KindUretprobe:
		return errors.Is(err, link.ErrNoSymbol)
	}
	return false
}
