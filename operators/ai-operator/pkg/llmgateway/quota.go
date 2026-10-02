package llmgateway

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	quotaListGVK = schema.GroupVersionKind{Group: "gryvia.io", Version: "v1alpha1", Kind: "GryviaQuotaList"}
	usageListGVK = schema.GroupVersionKind{Group: "gryvia.io", Version: "v1alpha1", Kind: "GryviaUsageRecordList"}
)

type quotaLimit struct {
	name       string
	namespaces []string
	perDay     int64
}

// Quotas enforces GryviaQuota.spec.tokensPerDay over the namespaces a quota lists. A namespace's tokens of the
// current UTC day are this process's own traffic (counted in memory from the start) plus, seeded once, the token
// records other gateway processes wrote (records labelled with this process's instance are skipped). With several
// replicas each sees the others' traffic only as of its seed, so the limit is approximate by up to
// (replicas - 1) times the traffic since the replicas started.
type Quotas struct {
	reader   client.Reader
	instance string

	mu     sync.Mutex
	limits []quotaLimit
	day    time.Time
	used   map[string]int64
	seeded map[string]bool
}

// NewQuotas reads quotas and records with reader; instance is the Meter's (its own records are not seeded).
func NewQuotas(reader client.Reader, instance string) *Quotas {
	return &Quotas{reader: reader, instance: instance, used: map[string]int64{}, seeded: map[string]bool{}}
}

// Refresh reloads the quotas that set tokensPerDay.
func (q *Quotas) Refresh(ctx context.Context) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(quotaListGVK)
	if err := q.reader.List(ctx, list); err != nil {
		return err
	}
	var limits []quotaLimit
	for _, item := range list.Items {
		per, found, _ := unstructured.NestedInt64(item.Object, "spec", "tokensPerDay")
		if !found || per <= 0 {
			continue
		}
		nss, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "namespaces")
		limits = append(limits, quotaLimit{name: item.GetName(), namespaces: nss, perDay: per})
	}
	q.mu.Lock()
	q.limits = limits
	q.mu.Unlock()
	return nil
}

func (q *Quotas) rollover(now time.Time) {
	day := now.UTC().Truncate(24 * time.Hour)
	if !day.Equal(q.day) {
		q.day = day
		q.used = map[string]int64{}
		q.seeded = map[string]bool{}
	}
}

// Check reports whether a key of namespace ns may make another request now. When a quota is used up it returns
// false with that quota's name, limit and usage.
func (q *Quotas) Check(ctx context.Context, ns string, now time.Time) (bool, string, int64, int64, error) {
	q.mu.Lock()
	q.rollover(now)
	var applicable []quotaLimit
	for _, l := range q.limits {
		for _, n := range l.namespaces {
			if n == ns {
				applicable = append(applicable, l)
				break
			}
		}
	}
	var unseeded []string
	for _, l := range applicable {
		for _, n := range l.namespaces {
			if !q.seeded[n] {
				unseeded = append(unseeded, n)
			}
		}
	}
	day := q.day
	q.mu.Unlock()
	if len(applicable) == 0 {
		return true, "", 0, 0, nil
	}
	for _, n := range unseeded {
		tokens, err := q.recorded(ctx, n, day)
		if err != nil {
			return false, "", 0, 0, err
		}
		q.mu.Lock()
		if q.day.Equal(day) && !q.seeded[n] {
			q.used[n] += tokens
			q.seeded[n] = true
		}
		q.mu.Unlock()
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, l := range applicable {
		var used int64
		for _, n := range l.namespaces {
			used += q.used[n]
		}
		if used >= l.perDay {
			return false, l.name, l.perDay, used, nil
		}
	}
	return true, "", 0, 0, nil
}

// Add counts the tokens of a finished request.
func (q *Quotas) Add(ns string, tokens int64, now time.Time) {
	if tokens <= 0 {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rollover(now)
	q.used[ns] += tokens
}

// recorded sums the input and output tokens of the namespace's token records that start on day, except this
// process's own.
func (q *Quotas) recorded(ctx context.Context, ns string, day time.Time) (int64, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(usageListGVK)
	if err := q.reader.List(ctx, list, client.InNamespace(ns), client.MatchingLabels{"gryvia.io/usage-kind": "tokens"}); err != nil {
		return 0, err
	}
	var total int64
	for _, item := range list.Items {
		if kind, _, _ := unstructured.NestedString(item.Object, "spec", "kind"); kind != "tokens" {
			continue
		}
		if q.instance != "" && item.GetLabels()[LabelInstance] == q.instance {
			continue
		}
		s, _, _ := unstructured.NestedString(item.Object, "spec", "start")
		start, err := time.Parse(time.RFC3339, s)
		if err != nil || start.UTC().Before(day) || !start.UTC().Before(day.Add(24*time.Hour)) {
			continue
		}
		in, _, _ := unstructured.NestedInt64(item.Object, "spec", "inputTokens")
		out, _, _ := unstructured.NestedInt64(item.Object, "spec", "outputTokens")
		total += in + out
	}
	return total, nil
}
