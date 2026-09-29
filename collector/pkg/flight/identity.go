package flight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zyvorai/gryvia/collector/pkg/kube"
)

// The pod UID is a canonical UUID; systemd cgroup names use underscores for
// the dashes. Match exactly so trailing text is never absorbed into the UID.
var podUID = regexp.MustCompile(`pod([0-9a-fA-F]{8}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{12})`)

type podList struct {
	Items []struct {
		Metadata struct {
			UID       string            `json:"uid"`
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Labels    map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			NodeName string `json:"nodeName"`
		} `json:"spec"`
	} `json:"items"`
}

// Resolver caches the Kubernetes pod list. It never guesses an identity when
// the PID's cgroup is unavailable or the pod is absent from the current list.
type Resolver struct {
	mu      sync.RWMutex
	byUID   map[string]Identity
	proc    string
	node    string
	updated time.Time
}

func NewResolver(node, proc string) *Resolver {
	if proc == "" {
		proc = "/proc"
	}
	// Inside a pod, the container's own /proc uses a different PID namespace
	// than the PIDs the kernel reports, so falling back to it would attribute
	// events to the wrong process. Only fall back when not running in a
	// cluster (then /proc is the host's); otherwise Resolve fails closed.
	if _, err := os.Stat(proc); err != nil && os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		proc = "/proc"
	}
	return &Resolver{byUID: map[string]Identity{}, proc: proc, node: node}
}
func normalizeUID(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "-")) }

func (r *Resolver) SetPods(raw []byte) error {
	var list podList
	if err := json.Unmarshal(raw, &list); err != nil {
		return err
	}
	next := make(map[string]Identity, len(list.Items))
	for _, p := range list.Items {
		if p.Spec.NodeName != r.node {
			continue
		}
		job := p.Metadata.Labels["gryvia.io/job"]
		if job == "" {
			continue
		}
		rank := p.Metadata.Labels["gryvia.io/rank"]
		if rank == "" {
			rank = p.Metadata.Labels["apps.kubernetes.io/pod-index"]
		}
		next[normalizeUID(p.Metadata.UID)] = Identity{Namespace: p.Metadata.Namespace, Job: job, Pod: p.Metadata.Name, Node: r.node, Rank: rank}
	}
	r.mu.Lock()
	r.byUID = next
	r.updated = time.Now()
	r.mu.Unlock()
	return nil
}

func (r *Resolver) Resolve(pid uint32) (Identity, bool) {
	if pid == 0 {
		return Identity{}, false
	}
	b, err := os.ReadFile(fmt.Sprintf("%s/%d/cgroup", r.proc, pid))
	if err != nil {
		return Identity{}, false
	}
	m := podUID.FindSubmatch(b)
	if m == nil {
		return Identity{}, false
	}
	r.mu.RLock()
	id, ok := r.byUID[normalizeUID(string(m[1]))]
	if time.Since(r.updated) > time.Minute {
		ok = false
	}
	r.mu.RUnlock()
	return id, ok
}

// JobOfPod returns the gryvia.io/job of a pod on this node (false when the pod
// is unknown, has no job label, or the pod list is stale). It never guesses.
func (r *Resolver) JobOfPod(namespace, pod string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if time.Since(r.updated) > time.Minute {
		return "", false
	}
	for _, id := range r.byUID {
		if id.Namespace == namespace && id.Pod == pod {
			return id.Job, true
		}
	}
	return "", false
}

// Run refreshes the node's pods every 15 seconds. The API server TLS CA and
// service account token are used; failed refreshes retain the last snapshot.
func (r *Resolver) Run(ctx context.Context, onError func(error)) {
	client, err := kube.NewInCluster()
	if err != nil {
		onError(err)
		return
	}
	refresh := func() {
		q := url.Values{"fieldSelector": {"spec.nodeName=" + r.node}}
		// The client re-reads the projected service account token on every request.
		b, err := client.Get(ctx, "/api/v1/pods?"+q.Encode())
		if err == nil {
			err = r.SetPods(b)
		}
		if err != nil {
			onError(err)
		}
	}
	refresh()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}
