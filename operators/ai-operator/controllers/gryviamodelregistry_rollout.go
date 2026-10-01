package controllers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

const (
	PhaseCanary     = "Canary"
	PhaseRolledBack = "RolledBack"

	ConditionPromotionGate = "PromotionGate"

	PromotionPromoted = "Promoted"
	PromotionRejected = "Rejected"
	PromotionWaiting  = "Waiting"

	defaultAutoCanaryWeight   = 10
	defaultAutoPromoteSeconds = 300

	labelServingGroup = "gryvia.io/serving-group"
	labelManagedBy    = "gryvia.io/managed-by"
	managedByRegistry = "gryvia-model-registry"
	directionMinimize = "minimize"
	promotionMaxPeers = 1000
)

// applyPromotionPolicy decides whether a staging entry with a promotionPolicy moves to production, and moves it
// (a patch of spec.stage, as the gateway's promote action does).
func (r *GryviaModelRegistryReconciler) applyPromotionPolicy(ctx context.Context, model *gryviav1.GryviaModelRegistry) error {
	if model.Spec.PromotionPolicy == nil || model.Spec.Stage != gryviav1.ModelStageStaging {
		return nil
	}
	decision, msg, err := r.promotionDecision(ctx, model)
	if err != nil {
		return err
	}
	model.Status.PromotionDecision = decision
	status := metav1.ConditionFalse
	if decision == PromotionPromoted {
		status = metav1.ConditionTrue
	}
	setCondition(&model.Status.Conditions, model.Generation, ConditionPromotionGate, status, decision, msg)
	if decision != PromotionPromoted {
		return nil
	}
	// The patch response carries the stored status; keep what this pass computed.
	computed := model.Status.DeepCopy()
	base := model.DeepCopy()
	model.Spec.Stage = gryviav1.ModelStageProduction
	if err := r.Patch(ctx, model, client.MergeFrom(base)); err != nil {
		return err
	}
	model.Status = *computed
	model.Status.Message = msg
	return nil
}

// promotionDecision compares the entry's metric against every production entry of the same modelName.
func (r *GryviaModelRegistryReconciler) promotionDecision(ctx context.Context, model *gryviav1.GryviaModelRegistry) (string, string, error) {
	p := model.Spec.PromotionPolicy
	raw, ok := model.Spec.Metadata[p.Metric]
	if !ok || strings.TrimSpace(raw) == "" {
		return PromotionWaiting, fmt.Sprintf("metadata %q is not set yet", p.Metric), nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return PromotionRejected, fmt.Sprintf("metadata %q = %q is not a number", p.Metric, raw), nil
	}
	minimize := p.Direction == directionMinimize
	better := func(a, b float64) float64 { // how much a improves on b
		if minimize {
			return b - a
		}
		return a - b
	}
	delta := 0.0
	if p.MinDelta != "" {
		if delta, err = strconv.ParseFloat(p.MinDelta, 64); err != nil || delta < 0 {
			return PromotionRejected, fmt.Sprintf("promotionPolicy.minDelta %q is not a non-negative number", p.MinDelta), nil
		}
	}
	if p.Threshold != "" {
		th, err := strconv.ParseFloat(p.Threshold, 64)
		if err != nil {
			return PromotionRejected, fmt.Sprintf("promotionPolicy.threshold %q is not a number", p.Threshold), nil
		}
		if better(v, th) < 0 {
			return PromotionRejected, fmt.Sprintf("%s %s does not reach the threshold %s", p.Metric, raw, p.Threshold), nil
		}
	}

	list := &gryviav1.GryviaModelRegistryList{}
	if err := r.List(ctx, list, client.InNamespace(model.Namespace), client.Limit(promotionMaxPeers)); err != nil {
		return "", "", err
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
	var bestName, bestRaw string
	var best float64
	for _, m := range list.Items {
		if m.Name == model.Name || m.Spec.ModelName != model.Spec.ModelName || m.Spec.Stage != gryviav1.ModelStageProduction {
			continue
		}
		pr, ok := m.Spec.Metadata[p.Metric]
		pv, err := strconv.ParseFloat(strings.TrimSpace(pr), 64)
		if !ok || err != nil {
			return PromotionRejected, fmt.Sprintf("production entry %s has no numeric %q to compare against; promote by hand", m.Name, p.Metric), nil
		}
		if bestName == "" || better(pv, best) > 0 {
			bestName, bestRaw, best = m.Name, pr, pv
		}
	}
	if bestName == "" {
		return PromotionPromoted, fmt.Sprintf("Promoted: %s %s and no production version of %s yet", p.Metric, raw, model.Spec.ModelName), nil
	}
	if gain := better(v, best); gain <= 0 || gain < delta {
		return PromotionRejected, fmt.Sprintf("%s %s does not beat %s (%s) by %s", p.Metric, raw, bestName, bestRaw, orZero(p.MinDelta)), nil
	}
	return PromotionPromoted, fmt.Sprintf("Promoted: %s %s beats %s (%s)", p.Metric, raw, bestName, bestRaw), nil
}

func orZero(s string) string {
	if s == "" {
		return "more than 0"
	}
	return "at least " + s
}

func sharedServiceName(model *gryviav1.GryviaModelRegistry) string {
	if model.Spec.ServingConfig == nil {
		return ""
	}
	return model.Spec.ServingConfig.ServiceName
}

// reconcileShared serves the entry through the GryviaInferenceService shared by the versions of a model. The
// service is labelled, not owned, so archiving the version that created it does not delete it.
//
//   - no service yet: create it with this entry as modelRef;
//   - this entry is the modelRef: keep the serving settings in sync and mirror health;
//   - another entry is the modelRef: run this entry as the canary (autoPromote, autoRollback). Once promoted, the
//     entry becomes the modelRef and the entry it replaced is archived. A rolled-back entry stays RolledBack.
func (r *GryviaModelRegistryReconciler) reconcileShared(ctx context.Context, model *gryviav1.GryviaModelRegistry, name string) error {
	if !stepNameRE.MatchString(name) || len(name) > maxDNSLabel {
		return configError{fmt.Errorf("servingConfig.serviceName %q is not a lowercase DNS label", name)}
	}
	model.Status.InferenceServiceName = name
	svc := &gryviav1.GryviaInferenceService{}
	err := r.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: name}, svc)
	if errors.IsNotFound(err) {
		svc = &gryviav1.GryviaInferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: model.Namespace,
				Labels: map[string]string{
					labelServingGroup:     name,
					labelManagedBy:        managedByRegistry,
					"gryvia.io/component": "inference",
				},
			},
			Spec: r.servingSpec(model),
		}
		svc.Spec.HealthCheck = &gryviav1.HealthCheckConfig{AutoRollback: true}
		if err := r.Create(ctx, svc); err != nil {
			if errors.IsInvalid(err) {
				return configError{err}
			}
			return err
		}
		model.Status.Phase = PhaseDeploying
		model.Status.Message = "Deploying shared inference service " + name
		setCondition(&model.Status.Conditions, model.Generation, ConditionModelServing, metav1.ConditionFalse, "Deploying", model.Status.Message)
		return nil
	}
	if err != nil {
		return err
	}
	if svc.Labels[labelManagedBy] != managedByRegistry || svc.Labels[labelServingGroup] != name {
		return configError{fmt.Errorf("inference service %q exists and is not managed by the model registry", name)}
	}

	if svc.Spec.ModelRef == model.Name {
		desired := r.servingSpec(model)
		if servingSettingsDiffer(svc.Spec, desired) {
			base := svc.DeepCopy()
			copyServingSettings(&svc.Spec, desired)
			if err := r.Patch(ctx, svc, client.MergeFrom(base)); err != nil {
				return err
			}
		}
		r.syncInferenceHealth(ctx, model)
		return nil
	}

	c := svc.Spec.Canary
	cs := svc.Status.CanaryStatus
	// The stable Deployment's annotations are the decision record; the canary status can lag a version behind.
	promoted, rolledBack, err := r.canaryDecisions(ctx, svc)
	if err != nil {
		return err
	}
	switch {
	case c != nil && c.Enabled && c.ModelVersion == model.Name && promoted == model.Name:
		previous := svc.Spec.ModelRef
		base := svc.DeepCopy()
		svc.Spec.ModelRef = model.Name
		svc.Spec.Canary = nil
		if err := r.Patch(ctx, svc, client.MergeFrom(base)); err != nil {
			return err
		}
		if err := r.archiveReplaced(ctx, model, previous, name); err != nil {
			return err
		}
		model.Status.Phase = PhaseDeploying
		model.Status.PreviousVersion = previous
		model.Status.Message = fmt.Sprintf("Canary promoted: %s replaces %s on %s", model.Name, previous, name)
		setCondition(&model.Status.Conditions, model.Generation, ConditionModelServing, metav1.ConditionFalse, "Promoted", model.Status.Message)
	case c != nil && c.Enabled && c.ModelVersion == model.Name && rolledBack == model.Name:
		model.Status.Phase = PhaseRolledBack
		model.Status.Health = "Unhealthy"
		model.Status.Message = fmt.Sprintf("Canary on %s was rolled back; %s keeps serving", name, svc.Spec.ModelRef)
		setCondition(&model.Status.Conditions, model.Generation, ConditionModelServing, metav1.ConditionFalse, "RolledBack", model.Status.Message)
	case c != nil && c.Enabled && c.ModelVersion == model.Name:
		model.Status.Phase = PhaseCanary
		model.Status.Health = "Unknown"
		if cs != nil && cs.Health != "" {
			model.Status.Health = cs.Health
		}
		model.Status.Message = fmt.Sprintf("Canary on %s at %d%% of the pods; %s is stable", name, c.Weight, svc.Spec.ModelRef)
		model.Status.ServingEndpoint = svc.Status.Endpoint
		setCondition(&model.Status.Conditions, model.Generation, ConditionModelServing, metav1.ConditionFalse, "Canary", model.Status.Message)
	case c != nil && c.Enabled && c.ModelVersion != promoted && c.ModelVersion != rolledBack && r.canaryBusy(ctx, svc):
		model.Status.Phase = PhaseDeploying
		model.Status.Message = fmt.Sprintf("Waiting: %s is the canary on %s", c.ModelVersion, name)
		setCondition(&model.Status.Conditions, model.Generation, ConditionModelServing, metav1.ConditionFalse, "Waiting", model.Status.Message)
	default:
		sc := model.Spec.ServingConfig
		weight, after := int32(defaultAutoCanaryWeight), int64(defaultAutoPromoteSeconds)
		if sc.CanaryWeight > 0 && sc.CanaryWeight < 100 {
			weight = sc.CanaryWeight
		}
		if sc.PromoteAfterSeconds > 0 {
			after = sc.PromoteAfterSeconds
		}
		base := svc.DeepCopy()
		svc.Spec.Canary = &gryviav1.CanaryConfig{Enabled: true, Weight: weight, ModelVersion: model.Name, AutoPromote: true, PromoteAfterSeconds: after}
		if svc.Spec.HealthCheck == nil {
			svc.Spec.HealthCheck = &gryviav1.HealthCheckConfig{AutoRollback: true}
		}
		if err := r.Patch(ctx, svc, client.MergeFrom(base)); err != nil {
			return err
		}
		model.Status.Phase = PhaseCanary
		model.Status.Health = "Unknown"
		model.Status.Message = fmt.Sprintf("Started as canary on %s at %d%% of the pods", name, weight)
		setCondition(&model.Status.Conditions, model.Generation, ConditionModelServing, metav1.ConditionFalse, "Canary", model.Status.Message)
	}
	return nil
}

// canaryDecisions reads the promoted and rolled-back versions the inference controller recorded on the stable
// Deployment ("" before the Deployment exists).
func (r *GryviaModelRegistryReconciler) canaryDecisions(ctx context.Context, svc *gryviav1.GryviaInferenceService) (string, string, error) {
	d := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: inferPrimaryName(svc)}, d); err != nil {
		if errors.IsNotFound(err) {
			return "", "", nil
		}
		return "", "", err
	}
	return d.Annotations[annotationPromoted], d.Annotations[annotationRolledBack], nil
}

// canaryBusy reports whether another version's undecided canary is still wanted (its entry exists, is in
// production and auto-served).
func (r *GryviaModelRegistryReconciler) canaryBusy(ctx context.Context, svc *gryviav1.GryviaInferenceService) bool {
	other := &gryviav1.GryviaModelRegistry{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: svc.Namespace, Name: svc.Spec.Canary.ModelVersion}, other); err != nil {
		return !errors.IsNotFound(err)
	}
	return other.Spec.Stage == gryviav1.ModelStageProduction && other.Spec.AutoServe
}

// archiveReplaced moves the entry a promoted canary replaced to archived, if it is a production entry of the same
// shared service.
func (r *GryviaModelRegistryReconciler) archiveReplaced(ctx context.Context, model *gryviav1.GryviaModelRegistry, previous, service string) error {
	if previous == "" || previous == model.Name {
		return nil
	}
	old := &gryviav1.GryviaModelRegistry{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: model.Namespace, Name: previous}, old); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if old.Spec.Stage != gryviav1.ModelStageProduction || sharedServiceName(old) != service {
		return nil
	}
	base := old.DeepCopy()
	old.Spec.Stage = gryviav1.ModelStageArchived
	return r.Patch(ctx, old, client.MergeFrom(base))
}
