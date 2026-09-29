import { describe, expect, it } from 'vitest'
import { dagLayers } from './dag'

const names = (layers: { name: string }[][]) => layers.map((l) => l.map((s) => s.name))

describe('dagLayers', () => {
  it('stacks independent steps in one level and chains dependents', () => {
    const layers = dagLayers([{ name: 'a' }, { name: 'b' }, { name: 'c', dependsOn: ['a', 'b'] }, { name: 'd', dependsOn: ['c'] }])
    expect(names(layers)).toEqual([['a', 'b'], ['c'], ['d']])
  })
  it('uses the deepest dependency', () => {
    const layers = dagLayers([{ name: 'a' }, { name: 'b', dependsOn: ['a'] }, { name: 'c', dependsOn: ['a', 'b'] }])
    expect(names(layers)).toEqual([['a'], ['b'], ['c']])
  })
  it('ignores unknown deps and survives cycles', () => {
    expect(names(dagLayers([{ name: 'a', dependsOn: ['ghost'] }]))).toEqual([['a']])
    const cyc = dagLayers([{ name: 'a', dependsOn: ['b'] }, { name: 'b', dependsOn: ['a'] }])
    expect(cyc.flat()).toHaveLength(2)
  })
  it('handles empty input', () => {
    expect(dagLayers([])).toEqual([])
  })
})
