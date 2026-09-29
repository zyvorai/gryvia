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

const SENSITIVE_ENV = /(^|_)(PASSWORD|PASSWD|SECRET|TOKEN|API_?KEY|PRIVATE_?KEY|CREDENTIALS?)($|_)/i

/** True when an env var's value should be masked until the user reveals it. */
export function isSensitiveEnv(name: string): boolean {
  return SENSITIVE_ENV.test(name)
}

export type EnvEntry = {
  name: string
  value?: string
  valueFrom?: {
    secretKeyRef?: { name?: string; key?: string }
    configMapKeyRef?: { name?: string; key?: string }
  }
}

/** Where an env var's value comes from when it is a reference, e.g. "from secret/db-creds"; undefined for literals. */
export function envSource(env: EnvEntry): string | undefined {
  const s = env.valueFrom?.secretKeyRef
  if (s) return `from secret/${s.name ?? 'unknown'}${s.key ? ` (${s.key})` : ''}`
  const c = env.valueFrom?.configMapKeyRef
  if (c) return `from configmap/${c.name ?? 'unknown'}${c.key ? ` (${c.key})` : ''}`
  if (env.valueFrom) return 'from reference'
  return undefined
}
