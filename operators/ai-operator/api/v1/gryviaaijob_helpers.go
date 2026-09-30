package v1

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Workload kinds for GryviaAIJobSpec.WorkloadKind.
const (
	WorkloadKindJob         = "job"
	WorkloadKindStatefulSet = "statefulset"
)

// TimeoutSeconds parses spec.timeout ("90m", "24h", "7d", "1d12h") into seconds.
// An empty timeout returns nil (no deadline). time.ParseDuration has no day unit,
// so a leading "<n>d" is accepted in addition to the Go duration syntax.
func (s GryviaAIJobSpec) TimeoutSeconds() (*int64, error) {
	t := strings.TrimSpace(s.Timeout)
	if t == "" {
		return nil, nil
	}
	var total time.Duration
	if i := strings.Index(t, "d"); i > 0 {
		days, err := strconv.Atoi(t[:i])
		if err != nil || days < 0 {
			return nil, fmt.Errorf("invalid timeout %q: day count must be a non-negative integer", s.Timeout)
		}
		total += time.Duration(days) * 24 * time.Hour
		t = t[i+1:]
	}
	if t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout %q: %w", s.Timeout, err)
		}
		total += d
	}
	secs := int64(total / time.Second)
	if secs <= 0 {
		return nil, fmt.Errorf("invalid timeout %q: must be positive", s.Timeout)
	}
	return &secs, nil
}
