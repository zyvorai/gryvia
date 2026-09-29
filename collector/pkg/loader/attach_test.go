package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cilium/ebpf"
)

func TestParseSection(t *testing.T) {
	cases := []struct {
		in   string
		want AttachSpec
		err  bool
	}{
		{"kprobe/tcp_v4_connect", AttachSpec{Kind: KindKprobe, Symbol: "tcp_v4_connect"}, false},
		{"kretprobe/tcp_recvmsg", AttachSpec{Kind: KindKretprobe, Symbol: "tcp_recvmsg"}, false},
		{"uprobe/ncclAllReduce", AttachSpec{Kind: KindUprobe, Symbol: "ncclAllReduce"}, false},
		{"uretprobe/cudaMalloc", AttachSpec{Kind: KindUretprobe, Symbol: "cudaMalloc"}, false},
		{"tracepoint/tcp/tcp_probe", AttachSpec{Kind: KindTracepoint, Group: "tcp", Symbol: "tcp_probe"}, false},
		{"raw_tracepoint/sys_enter", AttachSpec{Kind: KindRawTracepoint, Symbol: "sys_enter"}, false},
		{"tp_btf/block_rq_complete", AttachSpec{Kind: KindTPBTF, Symbol: "block_rq_complete"}, false},
		{"xdp", AttachSpec{Kind: KindXDP}, false},
		{"tcx/ingress", AttachSpec{Kind: KindTCXIngress}, false},
		{"tcx/egress", AttachSpec{Kind: KindTCXEgress}, false},
		{"sockops", AttachSpec{Kind: KindSockOps}, false},
		{"sk_msg", AttachSpec{Kind: KindSkMsg}, false},
		{"kprobe/", AttachSpec{}, true},
		{"tracepoint/tcp", AttachSpec{}, true},
		{"tcx/sideways", AttachSpec{}, true},
		{"license", AttachSpec{}, true},
	}
	for _, c := range cases {
		got, err := ParseSection(c.in)
		if (err != nil) != c.err {
			t.Errorf("%q: err=%v want err=%v", c.in, err, c.err)
			continue
		}
		if !c.err && got != c.want {
			t.Errorf("%q: got %+v want %+v", c.in, got, c.want)
		}
	}
}

func TestSkipReasonGating(t *testing.T) {
	if SkipReason(AttachSpec{Kind: KindKprobe, Symbol: "x"}, Config{}) != "" {
		t.Error("kprobe should never be gated")
	}
	for _, k := range []AttachKind{KindXDP, KindTCXIngress, KindTCXEgress} {
		if SkipReason(AttachSpec{Kind: k}, Config{}) == "" {
			t.Errorf("%s must be skipped without iface", k)
		}
		if SkipReason(AttachSpec{Kind: k}, Config{Iface: "lo"}) != "" {
			t.Errorf("%s must attach with iface", k)
		}
	}
	for _, k := range []AttachKind{KindSockOps, KindSkMsg} {
		if SkipReason(AttachSpec{Kind: k}, Config{Iface: "lo"}) == "" {
			t.Errorf("%s must be skipped without cgroup", k)
		}
		if SkipReason(AttachSpec{Kind: k}, Config{CgroupPath: "/sys/fs/cgroup"}) != "" {
			t.Errorf("%s must attach with cgroup", k)
		}
	}
	nccl := AttachSpec{Kind: KindUprobe, Symbol: "ncclSend"}
	if SkipReason(nccl, Config{CUDALib: "/x"}) == "" {
		t.Error("nccl uprobe needs NCCL lib")
	}
	if SkipReason(nccl, Config{NCCLLib: "/x"}) != "" {
		t.Error("nccl uprobe with lib should attach")
	}
	if SkipReason(AttachSpec{Kind: KindUretprobe, Symbol: "cudaMalloc"}, Config{CUDALib: "/x"}) != "" {
		t.Error("cuda uretprobe with lib should attach")
	}
}

func TestClassifyMap(t *testing.T) {
	cases := []struct {
		name string
		typ  ebpf.MapType
		want MapClass
	}{
		{"events", ebpf.PerfEventArray, ClassFlow},
		{"latency_events", ebpf.PerfEventArray, ClassFlow},
		{"events", ebpf.RingBuf, ClassNone}, // wrong reader type
		{"nccl_events", ebpf.RingBuf, ClassGPU},
		{"cuda_events", ebpf.RingBuf, ClassGPU},
		{"nccl_events", ebpf.PerfEventArray, ClassNone},
		{"privesc_events", ebpf.RingBuf, ClassSecurity},
		{"fim_events", ebpf.RingBuf, ClassSecurity},
		{"syscall_events", ebpf.PerfEventArray, ClassNone},
		{"connpool_events", ebpf.RingBuf, ClassNone},
	}
	for _, c := range cases {
		if got := ClassifyMap(c.name, c.typ); got != c.want {
			t.Errorf("%s/%s: got %q want %q", c.name, c.typ, got, c.want)
		}
	}
}

func TestResolveLibraries(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"libnccl.so.2", "libnccl.so", "libcudart.so.12"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := ResolveLibraries(Config{}, nil, []string{dir})
	if filepath.Base(cfg.NCCLLib) != "libnccl.so" || filepath.Base(cfg.CUDALib) != "libcudart.so.12" {
		t.Errorf("unexpected: %+v", cfg)
	}
	// Explicit config wins.
	cfg = ResolveLibraries(Config{NCCLLib: "/opt/n"}, nil, []string{dir})
	if cfg.NCCLLib != "/opt/n" {
		t.Errorf("explicit lib overridden: %s", cfg.NCCLLib)
	}
	// Nothing found.
	cfg = ResolveLibraries(Config{}, nil, []string{t.TempDir()})
	if cfg.NCCLLib != "" || cfg.CUDALib != "" {
		t.Errorf("expected empty, got %+v", cfg)
	}
}
