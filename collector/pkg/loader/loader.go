// Package loader handles loading, attaching, and lifecycle management
// of compiled eBPF programs.  It discovers .o files in a directory,
// loads them with cilium/ebpf (CO-RE relocations use the kernel BTF from
// /sys/kernel/btf/vmlinux automatically), attaches each program according
// to its ELF section name, and provides event readers.  Individual
// failures are logged and recorded in a status list; they never abort
// the whole load.
package loader

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"go.uber.org/zap"
)

// ProgramStatus records the outcome of loading/attaching one program.
type ProgramStatus struct {
	Object   string `json:"object"`
	Program  string `json:"program"`
	Section  string `json:"section"`
	Kind     string `json:"kind"`
	Target   string `json:"target,omitempty"`
	Attached bool   `json:"attached"`
	Reason   string `json:"reason,omitempty"` // why not attached
}

// MapReader is an opened event reader plus its dispatch class.
type MapReader struct {
	Object string
	Map    string
	Class  MapClass
	Perf   *perf.Reader
	Ring   *ringbuf.Reader
}

// Manager owns the lifecycle of all loaded eBPF programs.
type Manager struct {
	cfg Config
	log *zap.SugaredLogger

	mu          sync.Mutex
	collections []*ebpf.Collection
	links       []io.Closer
	readers     []MapReader
	status      []ProgramStatus
	exes        map[string]*link.Executable
}

// New creates a Manager for the given configuration.
func New(cfg Config, log *zap.SugaredLogger) (*Manager, error) {
	if cfg.Dir == "" {
		cfg.Dir = "/opt/gryvia/ebpf"
	}
	if _, err := os.Stat(cfg.Dir); err != nil {
		return nil, fmt.Errorf("ebpf directory not found: %w", err)
	}
	return &Manager{cfg: cfg, log: log, exes: map[string]*link.Executable{}}, nil
}

// LoadAndAttach loads every .o file in the directory and attaches its
// programs. It returns an error only if the directory cannot be read or
// nothing at all could be attached.
func (m *Manager) LoadAndAttach() error {
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		return fmt.Errorf("reading ebpf dir: %w", err)
	}
	m.cfg = ResolveLibraries(m.cfg, NewUprobeResolver(""), nil)
	m.log.Infow("uprobe libraries", "nccl", m.cfg.NCCLLib, "cuda", m.cfg.CUDALib)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".o" {
			continue
		}
		objPath := filepath.Join(m.cfg.Dir, entry.Name())
		if err := m.loadObject(entry.Name(), objPath); err != nil {
			m.log.Warnw("failed to load object, continuing", "path", objPath, "error", err)
			m.addStatus(ProgramStatus{Object: entry.Name(), Kind: "object", Reason: err.Error()})
		}
	}

	attached := 0
	for _, s := range m.Status() {
		if s.Attached {
			attached++
		}
	}
	m.log.Infow("eBPF load complete", "attached", attached, "total", len(m.Status()))
	if attached == 0 {
		return fmt.Errorf("no eBPF programs attached")
	}
	return nil
}

func (m *Manager) addStatus(s ProgramStatus) {
	m.mu.Lock()
	m.status = append(m.status, s)
	m.mu.Unlock()
}

func (m *Manager) loadObject(file, path string) error {
	spec, err := ebpf.LoadCollectionSpec(path)
	if err != nil {
		return fmt.Errorf("parse collection spec: %w", err)
	}
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	m.collections = append(m.collections, coll)

	names := make([]string, 0, len(coll.Programs))
	for n := range coll.Programs {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		st := ProgramStatus{Object: file, Program: name}
		ps := spec.Programs[name]
		st.Section = ps.SectionName
		as, err := ParseSection(ps.SectionName)
		if err != nil {
			st.Reason = err.Error()
			m.log.Warnw("skipping program", "object", file, "program", name, "reason", st.Reason)
			m.addStatus(st)
			continue
		}
		st.Kind = string(as.Kind)
		st.Target = as.Symbol
		if reason := SkipReason(as, m.cfg); reason != "" {
			st.Reason = "skipped: " + reason
			m.log.Infow("skipping program", "object", file, "program", name, "reason", reason)
			m.addStatus(st)
			continue
		}
		l, err := m.attach(as, coll.Programs[name], coll)
		if err != nil {
			st.Reason = err.Error()
			m.log.Warnw("failed to attach program", "object", file, "program", name,
				"section", ps.SectionName, "error", err)
			m.addStatus(st)
			continue
		}
		m.mu.Lock()
		m.links = append(m.links, l)
		m.mu.Unlock()
		st.Attached = true
		m.addStatus(st)
		m.log.Infow("attached program", "object", file, "program", name, "section", ps.SectionName)
	}

	// Open a reader of the matching type for each event map.
	for name, mp := range coll.Maps {
		class := ClassifyMap(name, mp.Type())
		if class == ClassNone {
			continue
		}
		mr := MapReader{Object: file, Map: name, Class: class}
		if mp.Type() == ebpf.PerfEventArray {
			mr.Perf, err = perf.NewReader(mp, os.Getpagesize()*64)
		} else {
			mr.Ring, err = ringbuf.NewReader(mp)
		}
		if err != nil {
			m.log.Warnw("failed to open event reader", "map", name, "error", err)
			continue
		}
		m.mu.Lock()
		m.readers = append(m.readers, mr)
		m.mu.Unlock()
		m.log.Infow("opened event reader", "object", file, "map", name, "class", string(class))
	}
	return nil
}

func (m *Manager) executable(path string) (*link.Executable, error) {
	if ex, ok := m.exes[path]; ok {
		return ex, nil
	}
	ex, err := link.OpenExecutable(path)
	if err != nil {
		return nil, err
	}
	m.exes[path] = ex
	return ex, nil
}

func (m *Manager) attach(as AttachSpec, prog *ebpf.Program, coll *ebpf.Collection) (io.Closer, error) {
	switch as.Kind {
	case KindKprobe:
		return link.Kprobe(as.Symbol, prog, nil)
	case KindKretprobe:
		return link.Kretprobe(as.Symbol, prog, nil)
	case KindUprobe, KindUretprobe:
		lib := LibraryFor(as.Symbol, m.cfg)
		ex, err := m.executable(lib)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", lib, err)
		}
		if as.Kind == KindUprobe {
			return ex.Uprobe(as.Symbol, prog, nil)
		}
		return ex.Uretprobe(as.Symbol, prog, nil)
	case KindTracepoint:
		return link.Tracepoint(as.Group, as.Symbol, prog, nil)
	case KindRawTracepoint:
		return link.AttachRawTracepoint(link.RawTracepointOptions{Name: as.Symbol, Program: prog})
	case KindTPBTF:
		return link.AttachTracing(link.TracingOptions{Program: prog})
	case KindXDP:
		iface, err := net.InterfaceByName(m.cfg.Iface)
		if err != nil {
			return nil, fmt.Errorf("interface %s: %w", m.cfg.Iface, err)
		}
		return link.AttachXDP(link.XDPOptions{Program: prog, Interface: iface.Index})
	case KindTCXIngress, KindTCXEgress:
		iface, err := net.InterfaceByName(m.cfg.Iface)
		if err != nil {
			return nil, fmt.Errorf("interface %s: %w", m.cfg.Iface, err)
		}
		at := ebpf.AttachTCXIngress
		if as.Kind == KindTCXEgress {
			at = ebpf.AttachTCXEgress
		}
		return link.AttachTCX(link.TCXOptions{Program: prog, Attach: at, Interface: iface.Index})
	case KindSockOps:
		return link.AttachCgroup(link.CgroupOptions{
			Path: m.cfg.CgroupPath, Program: prog, Attach: ebpf.AttachCGroupSockOps})
	case KindSkMsg:
		for _, mp := range coll.Maps {
			if t := mp.Type(); t == ebpf.SockHash || t == ebpf.SockMap {
				opts := link.RawAttachProgramOptions{
					Target: mp.FD(), Program: prog, Attach: ebpf.AttachSkMsgVerdict}
				if err := link.RawAttachProgram(opts); err != nil {
					return nil, err
				}
				return closerFunc(func() error {
					return link.RawDetachProgram(link.RawDetachProgramOptions{
						Target: opts.Target, Program: prog, Attach: opts.Attach})
				}), nil
			}
		}
		return nil, fmt.Errorf("no sockmap/sockhash in object")
	}
	return nil, fmt.Errorf("unsupported attach kind %q", as.Kind)
}

// Readers returns all opened event readers.
func (m *Manager) Readers() []MapReader {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MapReader(nil), m.readers...)
}

// PerfReaders returns perf readers of the given class.
func (m *Manager) PerfReaders(class MapClass) []*perf.Reader {
	var out []*perf.Reader
	for _, r := range m.Readers() {
		if r.Perf != nil && r.Class == class {
			out = append(out, r.Perf)
		}
	}
	return out
}

// RingBufReaders returns ring buffer readers of the given class.
func (m *Manager) RingBufReaders(class MapClass) []*ringbuf.Reader {
	var out []*ringbuf.Reader
	for _, r := range m.Readers() {
		if r.Ring != nil && r.Class == class {
			out = append(out, r.Ring)
		}
	}
	return out
}

// Status returns a snapshot of per-program load/attach results.
func (m *Manager) Status() []ProgramStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ProgramStatus(nil), m.status...)
}

// Close detaches all programs and frees resources.
func (m *Manager) Close() {
	for _, l := range m.links {
		if err := l.Close(); err != nil {
			m.log.Warnw("closing link", "error", err)
		}
	}
	for _, r := range m.readers {
		if r.Perf != nil {
			_ = r.Perf.Close()
		}
		if r.Ring != nil {
			_ = r.Ring.Close()
		}
	}
	for _, c := range m.collections {
		c.Close()
	}
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }
