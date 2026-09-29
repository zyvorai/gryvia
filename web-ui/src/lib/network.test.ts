import { describe, expect, it } from 'vitest'
import { bucketPolicies, edgeGeometry, filterFlows, flowFilterOptions, formatCaptureLevel, pageWindow, policyBucket, twoWayEdges, validatePolicyForm, verdictKind, verdictTone, EMPTY_FLOW_FILTERS } from './network'

describe('edge geometry', () => {
  it('keeps A->B and B->A apart', () => {
    const a = { x: 0, y: 0 }
    const b = { x: 200, y: 0 }
    const ab = edgeGeometry(a, b, 100, 50, 8)
    const ba = edgeGeometry(b, a, 100, 50, 8)
    expect(Math.abs(ab.mid.y - ba.mid.y)).toBeCloseTo(16)
    expect(ab.start.x).toBeCloseTo(50)
  })
  it('does not offset without a reverse edge', () => {
    expect(edgeGeometry({ x: 0, y: 0 }, { x: 200, y: 0 }, 100, 50, 0).mid.y).toBe(0)
  })
  it('detects two-way pairs', () => {
    expect(twoWayEdges([{ source: 'a', target: 'b' }, { source: 'b', target: 'a' }, { source: 'a', target: 'c' }])).toEqual([true, true, false])
  })
})

describe('verdicts', () => {
  it('classifies', () => {
    expect(verdictKind('allow')).toBe('good')
    expect(verdictTone('DROP')).toBe('bad')
    expect(verdictTone('ERROR')).toBe('warn')
    expect(verdictTone('FORWARDED')).toBe('')
    expect(verdictKind('')).toBe('unknown')
  })
  it('formats capture level', () => {
    expect(formatCaptureLevel('l7')).toBe('L7')
    expect(formatCaptureLevel('l3_l4')).toBe('L3/L4')
  })
})

const flow = (source: string, destination: string, protocol: string, verdict: string, namespace?: string) => ({ metadata: { namespace }, spec: { source, destination, protocol, verdict } })

describe('flow filters', () => {
  const flows = [flow('Web', 'db', 'TCP', 'FORWARDED', 'prod'), flow('web', 'api', 'tcp', 'ERROR', 'Prod'), flow('x', 'y', 'UDP', 'DROP', 'dev')]
  it('builds options from data, case-insensitively de-duplicated', () => {
    const o = flowFilterOptions(flows)
    expect(o.protocols).toEqual(['TCP', 'UDP'])
    expect(o.verdicts).toEqual(['DROP', 'ERROR', 'FORWARDED'])
    expect(o.namespaces).toEqual(['dev', 'prod'])
  })
  it('filters case-insensitively', () => {
    expect(filterFlows(flows, { ...EMPTY_FLOW_FILTERS, service: 'WEB' })).toHaveLength(2)
    expect(filterFlows(flows, { ...EMPTY_FLOW_FILTERS, namespace: 'PROD', protocol: 'tcp' })).toHaveLength(2)
    expect(filterFlows(flows, { ...EMPTY_FLOW_FILTERS, verdict: 'error' })).toHaveLength(1)
  })
})

describe('pageWindow', () => {
  it('stays inside bounds', () => {
    expect(pageWindow(1, 10)).toEqual([1, 2, 3, 4, 5])
    expect(pageWindow(5, 10)).toEqual([3, 4, 5, 6, 7])
    expect(pageWindow(10, 10)).toEqual([6, 7, 8, 9, 10])
    expect(pageWindow(2, 3)).toEqual([1, 2, 3])
  })
})

describe('policy buckets', () => {
  it('puts missing status in pending and unknown in other', () => {
    expect(policyBucket(undefined)).toBe('pending')
    expect(policyBucket('Active')).toBe('enforced')
    expect(policyBucket('Weird')).toBe('other')
    const b = bucketPolicies(['Enforced', undefined, 'Suggested', 'Failed'], (x) => x)
    expect(Object.values(b).flat()).toHaveLength(4)
  })
})

describe('policy form validation', () => {
  const ok = { name: '', sourceService: 'web', destinationService: 'db', port: '5432', intent: 'allow db' }
  it('accepts valid', () => expect(validatePolicyForm(ok)).toEqual({}))
  it('rejects bad values', () => {
    const e = validatePolicyForm({ name: 'Bad_Name', sourceService: 'Web', destinationService: 'a'.repeat(64), port: '70000', intent: '' })
    expect(Object.keys(e).sort()).toEqual(['destinationService', 'intent', 'name', 'port', 'sourceService'])
  })
})
