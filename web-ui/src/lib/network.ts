// Pure helpers for the network pages (graph geometry, verdicts, flow filters, policy buckets, form validation).
import type { Tone } from './phase'

export type VerdictKind = 'good' | 'drop' | 'error' | 'unknown'

/** FORWARDED/ALLOW are fine, DROP/DENIED are policy drops, ERROR is a measurement/plumbing problem. */
export function verdictKind(verdict?: string): VerdictKind {
  switch ((verdict ?? '').trim().toUpperCase()) {
    case 'FORWARDED':
    case 'ALLOW':
      return 'good'
    case 'DROP':
    case 'DENIED':
      return 'drop'
    case 'ERROR':
      return 'error'
    default:
      return 'unknown'
  }
}

/** Pill tone for a verdict: forwarded is neutral (colour = deviation), drops red, errors amber. */
export function verdictTone(verdict?: string): Tone {
  const k = verdictKind(verdict)
  return k === 'drop' ? 'bad' : k === 'error' ? 'warn' : ''
}

export type HealthKind = 'healthy' | 'warning' | 'critical' | 'unknown'
export function healthKind(health?: string): HealthKind {
  const h = (health ?? '').trim().toLowerCase()
  return h === 'healthy' || h === 'warning' || h === 'critical' ? h : 'unknown'
}

/** "l7" -> "L7", "l3_l4" / "l3,l4" -> "L3/L4". */
export function formatCaptureLevel(level?: string): string {
  if (!level) return '—'
  return level
    .trim()
    .toUpperCase()
    .split(/[\s,_/+-]+/)
    .filter(Boolean)
    .join('/')
}

export type Pt = { x: number; y: number }

/**
 * Line between two node boxes, clipped to the box borders and shifted sideways by `offset`
 * (perpendicular to the direction of travel). A->B and B->A both shift to their own left, so a
 * two-way pair renders as two parallel lines `2 * offset` apart instead of overlapping.
 */
export function edgeGeometry(from: Pt, to: Pt, boxW: number, boxH: number, offset: number, gap = 6): { start: Pt; end: Pt; mid: Pt } {
  const dx = to.x - from.x
  const dy = to.y - from.y
  const dist = Math.hypot(dx, dy) || 1
  const nx = dx / dist
  const ny = dy / dist
  const px = -ny * offset
  const py = nx * offset
  // Distance from centre to the box border along the direction of travel.
  const t = Math.min(nx === 0 ? Infinity : boxW / 2 / Math.abs(nx), ny === 0 ? Infinity : boxH / 2 / Math.abs(ny))
  const start = { x: from.x + nx * t + px, y: from.y + ny * t + py }
  const end = { x: to.x - nx * (t + gap) + px, y: to.y - ny * (t + gap) + py }
  return { start, end, mid: { x: (start.x + end.x) / 2, y: (start.y + end.y) / 2 } }
}

/** Which edges have a reverse edge in the list (A->B and B->A). */
export function twoWayEdges(edges: Array<{ source: string; target: string }>): boolean[] {
  const seen = new Set(edges.map((e) => `${e.source}\u0000${e.target}`))
  return edges.map((e) => e.source !== e.target && seen.has(`${e.target}\u0000${e.source}`))
}

// ---- flows -------------------------------------------------------------------------------

export type FlowLike = { metadata: { namespace?: string }; spec: { source: string; destination: string; protocol: string; verdict: string } }
export type FlowFilters = { service: string; namespace: string; verdict: string; protocol: string }
export const EMPTY_FLOW_FILTERS: FlowFilters = { service: '', namespace: '', verdict: '', protocol: '' }

const ci = (s: string | undefined) => (s ?? '').toLowerCase()

export function filterFlows<T extends FlowLike>(flows: T[], f: FlowFilters): T[] {
  const svc = ci(f.service).trim()
  const ns = ci(f.namespace).trim()
  return flows.filter((x) => {
    if (svc && !ci(x.spec.source).includes(svc) && !ci(x.spec.destination).includes(svc)) return false
    if (ns && ci(x.metadata.namespace) !== ns) return false
    if (f.verdict && ci(x.spec.verdict) !== ci(f.verdict)) return false
    if (f.protocol && ci(x.spec.protocol) !== ci(f.protocol)) return false
    return true
  })
}

/** Distinct, sorted, non-empty values (case-insensitive de-duplication, first spelling wins). */
export function distinctValues(values: Array<string | undefined>): string[] {
  const seen = new Map<string, string>()
  for (const v of values) {
    const t = (v ?? '').trim()
    if (t && !seen.has(t.toLowerCase())) seen.set(t.toLowerCase(), t)
  }
  return [...seen.values()].sort((a, b) => a.localeCompare(b))
}

export function flowFilterOptions(flows: FlowLike[]): { protocols: string[]; verdicts: string[]; namespaces: string[] } {
  return {
    protocols: distinctValues(flows.map((f) => f.spec.protocol)),
    verdicts: distinctValues(flows.map((f) => f.spec.verdict)),
    namespaces: distinctValues(flows.map((f) => f.metadata.namespace)),
  }
}

/** Page numbers to show: a window of up to `size` around `page`, always inside 1..total. */
export function pageWindow(page: number, total: number, size = 5): number[] {
  const n = Math.min(size, total)
  let start = page - Math.floor(n / 2)
  start = Math.max(1, Math.min(start, total - n + 1))
  return Array.from({ length: n }, (_, i) => start + i)
}

// ---- policies ----------------------------------------------------------------------------

/** No status = Pending. Enforced/Active enforced; Suggested; anything else lands in "other" so nothing vanishes. */
export type PolicyBucket = 'enforced' | 'pending' | 'suggested' | 'other'
export function policyBucket(phase?: string): PolicyBucket {
  const p = (phase ?? '').trim().toLowerCase()
  if (!p || p === 'pending') return 'pending'
  if (p === 'enforced' || p === 'active') return 'enforced'
  if (p === 'suggested') return 'suggested'
  return 'other'
}

export function bucketPolicies<T>(items: T[] | undefined, phaseOf: (t: T) => string | undefined): Record<PolicyBucket, T[]> {
  const out: Record<PolicyBucket, T[]> = { enforced: [], pending: [], suggested: [], other: [] }
  for (const it of items ?? []) out[policyBucket(phaseOf(it))].push(it)
  return out
}

export type PolicyFormValues = { name: string; sourceService: string; destinationService: string; port: string; intent: string }
export type PolicyFormErrors = Partial<Record<keyof PolicyFormValues, string>>

export const K8S_NAME = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
export const INTENT_MAX = 200

export function validateK8sName(v: string, label: string): string | undefined {
  if (!v) return `${label} is required.`
  if (v.length > 63) return `${label} must be 63 characters or fewer.`
  if (!K8S_NAME.test(v)) return `${label} must be lowercase letters, digits or "-", starting and ending with a letter or digit.`
  return undefined
}

export function validatePolicyForm(f: PolicyFormValues): PolicyFormErrors {
  const e: PolicyFormErrors = {}
  const name = f.name.trim()
  if (name) {
    const n = validateK8sName(name, 'Name')
    if (n) e.name = n
  }
  const s = validateK8sName(f.sourceService.trim(), 'Source')
  if (s) e.sourceService = s
  const d = validateK8sName(f.destinationService.trim(), 'Destination')
  if (d) e.destinationService = d
  const port = Number(f.port)
  if (f.port.trim() === '' || !Number.isInteger(port) || port < 1 || port > 65535) e.port = 'Port must be a whole number from 1 to 65535.'
  const intent = f.intent.trim()
  if (!intent) e.intent = 'Intent is required.'
  else if (intent.length > INTENT_MAX) e.intent = `Intent must be ${INTENT_MAX} characters or fewer.`
  return e
}

/** One severity vocabulary for network anomalies and security alerts. */
export function severityTone(severity?: string): Tone {
  switch ((severity ?? '').toLowerCase()) {
    case 'critical':
      return 'bad'
    case 'high':
      return 'warn'
    case 'medium':
      return 'info'
    default:
      return ''
  }
}
