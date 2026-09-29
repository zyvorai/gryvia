import { describe, expect, it } from 'vitest'
import { bearerCandidates } from './credentials'

describe('bearerCandidates', () => {
  it('rejects a wrong username without any candidate', () => {
    expect(bearerCandidates('root', 'Admin@321')).toEqual([])
  })

  it('rejects an empty password', () => {
    expect(bearerCandidates('admin', '')).toEqual([])
  })

  it('maps the default pair to the default bearer once', () => {
    expect(bearerCandidates('admin', 'Admin@321')).toEqual(['Admin@321'])
  })

  it('tries any other password as the bearer (API key as password)', () => {
    expect(bearerCandidates('admin', 'k3y-from-deploy')).toEqual(['k3y-from-deploy'])
  })
})
