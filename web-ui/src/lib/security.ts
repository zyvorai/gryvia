import type { SecurityPolicy } from './api'

export const RULE_CATALOG: Array<{ type: string; name: string; description: string }> = [
  { type: 'escape', name: 'Container escape', description: 'Detects processes breaking out of their container namespace.' },
  { type: 'mining', name: 'Cryptomining', description: 'Flags known miner binaries and mining-pool connections.' },
  { type: 'exfiltration', name: 'Data exfiltration', description: 'Flags unusual outbound transfers of sensitive data.' },
  { type: 'privesc', name: 'Privilege escalation', description: 'Detects setuid, capability and credential escalation.' },
  { type: 'driver_fim', name: 'Driver file integrity', description: 'Watches GPU driver and kernel module files for changes.' },
]

/** Friendly rule name; unknown types are humanised (underscores replaced, sentence case). */
export function ruleName(type: string): string {
  const known = RULE_CATALOG.find((r) => r.type === type)
  if (known) return known.name
  const t = type.replace(/_/g, ' ').trim()
  return t ? t.charAt(0).toUpperCase() + t.slice(1) : 'Unknown'
}

const isActive = (p: SecurityPolicy) => (p.status?.phase ?? '').toLowerCase() === 'active'

/** Rule types with at least one enabled rule in an Active policy (Pending/Failed policies protect nothing). */
export function activeRuleTypes(policies: SecurityPolicy[] | undefined): Set<string> {
  const out = new Set<string>()
  for (const p of policies ?? []) {
    if (!isActive(p)) continue
    for (const r of p.spec.detectionRules ?? []) if (r.enabled) out.add(r.type)
  }
  return out
}

export function countActivePolicies(policies: SecurityPolicy[] | undefined): number {
  return (policies ?? []).filter(isActive).length
}

/** Real per-policy counters, aggregated. */
export function policyCounters(policies: SecurityPolicy[] | undefined): { alerts: number; lastAlert?: string; byType: Array<[string, number]> } {
  let alerts = 0
  let last: string | undefined
  const by = new Map<string, number>()
  for (const p of policies ?? []) {
    alerts += p.status?.alertsTriggered ?? 0
    const la = p.status?.lastAlert
    if (la && (!last || new Date(la).getTime() > new Date(last).getTime())) last = la
    for (const [k, v] of Object.entries(p.status?.detectionCounts ?? {})) by.set(k, (by.get(k) ?? 0) + v)
  }
  return { alerts, lastAlert: last, byType: [...by.entries()].sort((a, b) => b[1] - a[1]) }
}
