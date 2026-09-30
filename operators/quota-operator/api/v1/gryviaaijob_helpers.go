package v1

// TotalGPUs is the number of GPUs the job holds in total, the number every cost and usage
// calculation must use. It mirrors the AI operator's placement: a distributed job runs
// distributed.nodes pods with distributed.gpusPerNode GPUs each (gpusPerNode falls back to 1
// when spec.gpus is set, 0 for a CPU-only job); any other job holds spec.gpus. Reading
// spec.gpus alone undercounts distributed jobs (spec.gpus is often the per-node figure or 0).
func (s GryviaAIJobSpec) TotalGPUs() int32 {
	if s.Distributed != nil && s.Distributed.Enabled {
		nodes := s.Distributed.Nodes
		if nodes < 1 {
			nodes = 1
		}
		per := s.Distributed.GpusPerNode
		if per <= 0 {
			if s.GPUs == 0 {
				return 0
			}
			per = 1
		}
		return nodes * per
	}
	if s.GPUs < 0 {
		return 1
	}
	return s.GPUs
}
