package controllers

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	gryviav1 "github.com/zyvorai/gryvia/operators/ai-operator/api/v1"
)

// validateElastic checks spec.distributed.elastic. kind is the resolved workload kind: elastic
// scaling needs the Indexed batch Job (inference StatefulSets have a fixed replica set).
func validateElastic(job *gryviav1.GryviaAIJob, kind string) error {
	min, max, ok := job.Spec.Distributed.ElasticBounds()
	if !ok {
		return nil
	}
	if kind == gryviav1.WorkloadKindStatefulSet {
		return fmt.Errorf("distributed.elastic needs a run-to-completion Job; a StatefulSet (inference) has a fixed replica count")
	}
	if fw := job.Spec.Distributed.Framework; fw != "" && fw != "pytorch" {
		return fmt.Errorf("distributed.elastic is for PyTorch (torchrun); framework %q is not supported", fw)
	}
	if min < 1 {
		return fmt.Errorf("distributed.elastic.minNodes must be at least 1, got %d", min)
	}
	if max < min {
		return fmt.Errorf("distributed.nodes (%d) must be at least distributed.elastic.minNodes (%d)", max, min)
	}
	return nil
}

// elasticSuccessPolicy lets an elastic Job finish once minNodes indexes succeeded: workers that never
// got scheduled (capacity lost) would otherwise keep the Job from completing. nil when not elastic or
// when min == max. Needs a Kubernetes version with batch/v1 Job successPolicy (beta in 1.31, GA 1.33).
func elasticSuccessPolicy(job *gryviav1.GryviaAIJob) *batchv1.SuccessPolicy {
	min, max, ok := job.Spec.Distributed.ElasticBounds()
	if !ok || min >= max {
		return nil
	}
	return &batchv1.SuccessPolicy{Rules: []batchv1.SuccessPolicyRule{{SucceededCount: &min}}}
}

func hasEnv(env []corev1.EnvVar, name string) bool {
	for _, e := range env {
		if e.Name == name {
			return true
		}
	}
	return false
}

// replaceEnv sets name to value, replacing an existing entry or appending.
func replaceEnv(env []corev1.EnvVar, name, value string) []corev1.EnvVar {
	for i := range env {
		if env[i].Name == name {
			env[i].Value, env[i].ValueFrom = value, nil
			return env
		}
	}
	return append(env, corev1.EnvVar{Name: name, Value: value})
}
