package controllers

import (
	"context"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gryviav1 "github.com/zyvorai/gryvia/operators/quota-operator/api/v1"
	"github.com/zyvorai/gryvia/operators/quota-operator/pkg/pricing"
)

const (
	// usageRecordPrefix is the name prefix of a usage record: usage-<job UID>.
	usageRecordPrefix = "usage-"
	// tenantNamespacePrefix is the namespace convention of the GryviaTenant controller.
	tenantNamespacePrefix = "tenant-"
	// usageRequeue is how often a running job's record is refreshed.
	usageRequeue = time.Minute

	labelTenant = "gryvia.io/tenant"
	labelJob    = "gryvia.io/job"
	labelTeam   = "gryvia.io/team"
)

// GryviaUsageRecordReconciler meters GryviaAIJobs into GryviaUsageRecords.
//
// One record per job UID: while the job runs the record is refreshed every minute
// (final=false, end=nil); once the job reaches Succeeded, Failed or Cancelled it is
// finalized from status.completionTime and never modified again. Hours are wall-clock
// time multiplied by GPUs, so costs are metered estimates, not invoices.
type GryviaUsageRecordReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Now returns the current time; tests override it. Defaults to time.Now.
	Now func() time.Time
}

//+kubebuilder:rbac:groups=gryvia.io,resources=gryviaaijobs,verbs=get;list;watch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviausagerecords,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviagpuskus,verbs=get;list;watch
//+kubebuilder:rbac:groups=gryvia.io,resources=gryviatenants,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *GryviaUsageRecordReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func isTerminalPhase(phase string) bool {
	switch phase {
	case "Succeeded", "Failed", "Cancelled":
		return true
	}
	return false
}

func (r *GryviaUsageRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	job := &gryviav1.GryviaAIJob{}
	if err := r.Get(ctx, req.NamespacedName, job); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, r.closeOrphans(ctx, req.Namespace, req.Name)
		}
		return ctrl.Result{}, err
	}

	// Jobs that never started consumed nothing.
	if job.Status.StartTime == nil || job.Status.StartTime.IsZero() || job.UID == "" {
		return ctrl.Result{}, nil
	}

	recName := usageRecordPrefix + string(job.UID)
	rec := &gryviav1.GryviaUsageRecord{}
	err := r.Get(ctx, types.NamespacedName{Namespace: job.Namespace, Name: recName}, rec)
	exists := err == nil
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if exists && rec.Spec.Final {
		return ctrl.Result{}, nil // final records are immutable
	}

	now := r.now()
	terminal := isTerminalPhase(job.Status.Phase)
	start := job.Status.StartTime.Time
	end := now
	var endPtr *metav1.Time
	if terminal {
		if job.Status.CompletionTime != nil && !job.Status.CompletionTime.IsZero() {
			end = job.Status.CompletionTime.Time
		}
		t := metav1.NewTime(end)
		endPtr = &t
	}
	hours := end.Sub(start).Hours()
	if hours < 0 {
		hours = 0
	}
	gpuHours := hours * float64(job.Spec.GPUs)

	tenantName, tenant, err := r.resolveTenant(ctx, job)
	if err != nil {
		return ctrl.Result{}, err
	}
	skuName, rate, currency, err := r.resolveRate(ctx, job.Spec.GpuType, tenant)
	if err != nil {
		return ctrl.Result{}, err
	}

	spec := gryviav1.GryviaUsageRecordSpec{
		Tenant:   tenantName,
		Job:      job.Name,
		JobUID:   string(job.UID),
		GpuType:  job.Spec.GpuType,
		Sku:      skuName,
		Gpus:     job.Spec.GPUs,
		Start:    *job.Status.StartTime,
		End:      endPtr,
		GpuHours: gpuHours,
		Rate:     rate,
		Cost:     gpuHours * rate,
		Currency: currency,
		Final:    terminal,
	}

	if !exists {
		rec = &gryviav1.GryviaUsageRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      recName,
				Namespace: job.Namespace,
				Labels:    map[string]string{labelTenant: tenantName, labelJob: job.Name},
			},
			Spec: spec,
		}
		if err := r.Create(ctx, rec); err != nil {
			if errors.IsAlreadyExists(err) {
				return ctrl.Result{Requeue: true}, nil // raced with a cache lag; retry
			}
			return ctrl.Result{}, err
		}
		logger.Info("Created usage record", "record", recName, "tenant", tenantName, "final", terminal)
	} else if !equality.Semantic.DeepEqual(rec.Spec, spec) {
		rec.Spec = spec
		if rec.Labels == nil {
			rec.Labels = map[string]string{}
		}
		rec.Labels[labelTenant] = tenantName
		rec.Labels[labelJob] = job.Name
		if err := r.Update(ctx, rec); err != nil {
			return ctrl.Result{}, err
		}
	}

	if terminal {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: usageRequeue}, nil
}

// closeOrphans finalizes open records of a job that was deleted while running, so a
// record never stays open forever. The end time is the moment the deletion is seen.
func (r *GryviaUsageRecordReconciler) closeOrphans(ctx context.Context, namespace, jobName string) error {
	recs := &gryviav1.GryviaUsageRecordList{}
	if err := r.List(ctx, recs, client.InNamespace(namespace), client.MatchingLabels{labelJob: jobName}); err != nil {
		return err
	}
	now := metav1.NewTime(r.now())
	for i := range recs.Items {
		rec := &recs.Items[i]
		if rec.Spec.Final {
			continue
		}
		hours := now.Sub(rec.Spec.Start.Time).Hours()
		if hours < 0 {
			hours = 0
		}
		rec.Spec.End = &now
		rec.Spec.GpuHours = hours * float64(rec.Spec.Gpus)
		rec.Spec.Cost = rec.Spec.GpuHours * rec.Spec.Rate
		rec.Spec.Final = true
		if err := r.Update(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

// resolveTenant returns the tenant name for a job and the GryviaTenant object when
// one owns the namespace (tenant-<name>). Otherwise it falls back to the job's or
// namespace's gryvia.io/team label, then to the namespace name.
func (r *GryviaUsageRecordReconciler) resolveTenant(ctx context.Context, job *gryviav1.GryviaAIJob) (string, *gryviav1.GryviaTenant, error) {
	if strings.HasPrefix(job.Namespace, tenantNamespacePrefix) {
		t := &gryviav1.GryviaTenant{}
		err := r.Get(ctx, types.NamespacedName{Name: strings.TrimPrefix(job.Namespace, tenantNamespacePrefix)}, t)
		if err == nil {
			return t.Name, t, nil
		}
		if !errors.IsNotFound(err) {
			return "", nil, err
		}
	}
	if team := job.Labels[labelTeam]; team != "" {
		return team, nil, nil
	}
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: job.Namespace}, ns); err == nil {
		if team := ns.Labels[labelTeam]; team != "" {
			return team, nil, nil
		}
	} else if !errors.IsNotFound(err) {
		return "", nil, err
	}
	return job.Namespace, nil, nil
}

// resolveRate prices a GPU type: the matching enabled GryviaGpuSku (restricted to the
// tenant's allowedSkus when set) or, only when the cluster has no SKUs at all, the
// shared default table. A type no SKU covers is metered at rate 0 (and is rejected by
// the quota controller when the tenant restricts SKUs).
func (r *GryviaUsageRecordReconciler) resolveRate(ctx context.Context, gpuType string, tenant *gryviav1.GryviaTenant) (sku string, rate float64, currency string, err error) {
	skus := &gryviav1.GryviaGpuSkuList{}
	if err = r.List(ctx, skus); err != nil {
		return "", 0, "", err
	}
	if len(skus.Items) == 0 {
		return "", pricing.DefaultRate(gpuType), pricing.DefaultCurrency, nil
	}
	var allowed []string
	if tenant != nil {
		allowed = tenant.Spec.AllowedSkus
	}
	if s := findSku(skus.Items, gpuType, allowed); s != nil {
		cur := s.Spec.Currency
		if cur == "" {
			cur = pricing.DefaultCurrency
		}
		return s.Name, s.Spec.HourlyRate, cur, nil
	}
	return "", 0, pricing.DefaultCurrency, nil
}

// skuEnabled treats an unset enabled flag as true (the CRD default).
func skuEnabled(s *gryviav1.GryviaGpuSku) bool {
	return s.Spec.Enabled == nil || *s.Spec.Enabled
}

// findSku returns the enabled SKU (by name order, so the choice is stable) whose gpuType
// matches, honouring allowedSkus when non-empty.
func findSku(items []gryviav1.GryviaGpuSku, gpuType string, allowedSkus []string) *gryviav1.GryviaGpuSku {
	sorted := make([]*gryviav1.GryviaGpuSku, 0, len(items))
	for i := range items {
		sorted = append(sorted, &items[i])
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, s := range sorted {
		if !skuEnabled(s) || !strings.EqualFold(s.Spec.GpuType, gpuType) {
			continue
		}
		if len(allowedSkus) > 0 && !containsString(allowedSkus, s.Name) {
			continue
		}
		return s
	}
	return nil
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *GryviaUsageRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("gryviausagerecord").
		For(&gryviav1.GryviaAIJob{}).
		Complete(r)
}
