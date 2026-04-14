// Package loader handles loading, attaching, and lifecycle management
// of compiled eBPF programs.  It discovers .o files in a directory,
// loads them with cilium/ebpf, attaches kprobes/tracepoints/XDP, and
// provides access to perf-event readers and pinned maps.
package loader

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/ringbuf"
	"go.uber.org/zap"
)

// Manager owns the lifecycle of all loaded eBPF programs.
type Manager struct {
	dir       string
	iface     string
	cgroupDir string
	log       *zap.SugaredLogger

	collections []*ebpf.Collection
	links       []link.Link
	readers     []*perf.Reader
	ringReaders []*ringbuf.Reader
}

// New creates a Manager that will load programs from dir and attach
// XDP programs to the given network interface.
func New(dir, iface string, log *zap.SugaredLogger) (*Manager, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("ebpf directory not found: %w", err)
	}
	return &Manager{
		dir:       dir,
		iface:     iface,
		cgroupDir: "/sys/fs/cgroup",
		log:       log,
	}, nil
}

// SetCgroupDir sets the cgroup path used for SockOps attachment.
func (m *Manager) SetCgroupDir(path string) {
	m.cgroupDir = path
}

// LoadAndAttach discovers and loads every eBPF object file in the
// configured directory, then attaches programs to the kernel.
func (m *Manager) LoadAndAttach() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("reading ebpf dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".o" {
			continue
		}

		objPath := filepath.Join(m.dir, entry.Name())
		m.log.Infow("loading eBPF object", "path", objPath)

		if err := m.loadObject(objPath); err != nil {
			return fmt.Errorf("loading %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func (m *Manager) loadObject(path string) error {
	spec, err := ebpf.LoadCollectionSpec(path)
	if err != nil {
		return fmt.Errorf("parse collection spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	m.collections = append(m.collections, coll)

	// Attach each program based on its type.
	for name, prog := range coll.Programs {
		if err := m.attachProgram(name, prog, coll); err != nil {
			m.log.Warnw("failed to attach program",
				"name", name, "error", err)
			continue
		}
		m.log.Infow("attached program", "name", name, "type", prog.Type().String())
	}

	// Open perf readers for PERF_EVENT_ARRAY maps and ring buffer readers
	// for RINGBUF maps.
	for name, mp := range coll.Maps {
		info, err := mp.Info()
		if err != nil {
			continue
		}
		switch info.Type {
		case ebpf.PerfEventArray:
			reader, err := perf.NewReader(mp, os.Getpagesize()*64)
			if err != nil {
				m.log.Warnw("failed to create perf reader",
					"map", name, "error", err)
				continue
			}
			m.readers = append(m.readers, reader)
			m.log.Infow("opened perf reader", "map", name)

		case ebpf.RingBuf:
			reader, err := ringbuf.NewReader(mp)
			if err != nil {
				m.log.Warnw("failed to create ring buffer reader",
					"map", name, "error", err)
				continue
			}
			m.ringReaders = append(m.ringReaders, reader)
			m.log.Infow("opened ring buffer reader", "map", name)
		}
	}

	return nil
}

func (m *Manager) attachProgram(name string, prog *ebpf.Program, coll *ebpf.Collection) error {
	switch prog.Type() {
	case ebpf.Kprobe:
		// Derive kernel function name from the ELF section name.
		l, err := link.Kprobe(name, prog, nil)
		if err != nil {
			return fmt.Errorf("kprobe attach %s: %w", name, err)
		}
		m.links = append(m.links, l)

	case ebpf.XDP:
		iface, err := netInterfaceByName(m.iface)
		if err != nil {
			return fmt.Errorf("interface lookup %s: %w", m.iface, err)
		}
		l, err := link.AttachXDP(link.XDPOptions{
			Program:   prog,
			Interface: iface,
		})
		if err != nil {
			return fmt.Errorf("xdp attach: %w", err)
		}
		m.links = append(m.links, l)

	case ebpf.RawTracepoint:
		l, err := link.AttachRawTracepoint(link.RawTracepointOptions{
			Name:    name,
			Program: prog,
		})
		if err != nil {
			return fmt.Errorf("raw tracepoint attach %s: %w", name, err)
		}
		m.links = append(m.links, l)

	case ebpf.Tracing:
		// fentry/fexit programs.
		l, err := link.AttachTracing(link.TracingOptions{
			Program: prog,
		})
		if err != nil {
			return fmt.Errorf("tracing attach %s: %w", name, err)
		}
		m.links = append(m.links, l)

	case ebpf.SchedCLS:
		// TC classifier programs via TCX.
		iface, err := net.InterfaceByName(m.iface)
		if err != nil {
			return fmt.Errorf("interface lookup for tc %s: %w", m.iface, err)
		}
		l, err := link.AttachTCX(link.TCXOptions{
			Program:   prog,
			Attach:    ebpf.AttachTCXIngress,
			Interface: iface.Index,
		})
		if err != nil {
			return fmt.Errorf("tcx attach %s: %w", name, err)
		}
		m.links = append(m.links, l)

	case ebpf.SockOps:
		// Attach to the cgroup root for socket-level interception.
		cgroupF, err := os.Open(m.cgroupDir)
		if err != nil {
			return fmt.Errorf("open cgroup %s: %w", m.cgroupDir, err)
		}
		defer cgroupF.Close()
		l, err := link.AttachCgroup(link.CgroupOptions{
			Path:    m.cgroupDir,
			Program: prog,
			Attach:  ebpf.AttachCGroupSockOps,
		})
		if err != nil {
			return fmt.Errorf("sockops cgroup attach %s: %w", name, err)
		}
		m.links = append(m.links, l)

	case ebpf.SkMsg:
		// SkMsg programs attach to a sock_map via raw link.
		// Look for a sock_map in the same collection.
		var sockMap *ebpf.Map
		for _, mp := range coll.Maps {
			info, err := mp.Info()
			if err != nil {
				continue
			}
			if info.Type == ebpf.SockHash || info.Type == ebpf.Array {
				sockMap = mp
				break
			}
		}
		if sockMap == nil {
			return fmt.Errorf("sk_msg %s: no suitable sock_map found in collection", name)
		}
		l, err := link.AttachRawLink(link.RawLinkOptions{
			Program: prog,
			Attach:  ebpf.AttachSkMsgVerdict,
			Target:  sockMap.FD(),
		})
		if err != nil {
			return fmt.Errorf("sk_msg attach %s: %w", name, err)
		}
		m.links = append(m.links, l)

	default:
		return fmt.Errorf("unsupported program type: %s", prog.Type())
	}
	return nil
}

// PerfReaders returns all perf-event readers opened during loading.
func (m *Manager) PerfReaders() []*perf.Reader {
	return m.readers
}

// RingBufReaders returns all ring buffer readers opened during loading.
func (m *Manager) RingBufReaders() []*ringbuf.Reader {
	return m.ringReaders
}

// Close detaches all programs and frees resources.
func (m *Manager) Close() {
	for _, l := range m.links {
		if err := l.Close(); err != nil {
			m.log.Warnw("closing link", "error", err)
		}
	}
	for _, r := range m.readers {
		if err := r.Close(); err != nil {
			m.log.Warnw("closing perf reader", "error", err)
		}
	}
	for _, r := range m.ringReaders {
		if err := r.Close(); err != nil {
			m.log.Warnw("closing ring buffer reader", "error", err)
		}
	}
	for _, c := range m.collections {
		c.Close()
	}
}

// netInterfaceByName returns the interface index for the named interface.
func netInterfaceByName(name string) (int, error) {
	ifaces, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return 0, err
	}
	for i, entry := range ifaces {
		if entry.Name() == name {
			// Interface indices are 1-based in the kernel, but we
			// need the actual ifindex from /sys.
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("interface %s not found", name)
}
