import { describe, expect, it } from 'vitest'
import type { SecurityPolicy } from './api'
import { activeRuleTypes, countActivePolicies, policyCounters, ruleName } from './security'

const pol = (phase: string | undefined, rules: Array<[string, boolean]>, status: Partial<NonNullable<SecurityPolicy['status']>> = {}): SecurityPolicy => ({
  metadata: { name: 'p' },
  spec: { targetNamespaces: [], autoBlock: false, detectionRules: rules.map(([type, enabled]) => ({ type, enabled, sensitivity: 'high' })) },
  status: phase ? { phase, activeDetections: 0, alertsTriggered: 0, ...status } : undefined,
})

describe('security helpers', () => {
  it('names rules', () => {
    expect(ruleName('driver_fim')).toBe('Driver file integrity')
    expect(ruleName('some_new_rule')).toBe('Some new rule')
  })
  it('counts coverage only from Active policies', () => {
    const ps = [pol('Active', [['escape', true], ['mining', false]]), pol('Pending', [['privesc', true]]), pol(undefined, [['mining', true]])]
    expect([...activeRuleTypes(ps)]).toEqual(['escape'])
    expect(countActivePolicies(ps)).toBe(1)
  })
  it('aggregates counters', () => {
    const c = policyCounters([pol('Active', [], { alertsTriggered: 2, lastAlert: '2026-01-01T00:00:00Z', detectionCounts: { escape: 2 } }), pol('Active', [], { alertsTriggered: 1, lastAlert: '2026-02-01T00:00:00Z', detectionCounts: { escape: 1, mining: 5 } })])
    expect(c.alerts).toBe(3)
    expect(c.lastAlert).toBe('2026-02-01T00:00:00Z')
    expect(c.byType[0]).toEqual(['mining', 5])
  })
})
