import type { FabricAIJob } from '@/types'

export const FRAMEWORK_LABEL = 'gryvia.io/framework'

/** ML framework of a job: distributed config, then the label the dashboard sets, then the job type. */
export function jobFramework(job: FabricAIJob): string {
  return job.spec?.distributed?.framework || job.metadata?.labels?.[FRAMEWORK_LABEL] || job.spec?.type || 'job'
}

/** "8 × H100", or "8 × GPU" when the type is unset. */
export function jobGpus(job: FabricAIJob): string {
  return `${job.spec?.gpus ?? 0} × ${job.spec?.gpuType || 'GPU'}`
}
