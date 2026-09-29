import { describe, expect, it } from 'vitest'
import { envSource, isSensitiveEnv } from './jobs'

describe('env redaction', () => {
  it('masks credential-like names only', () => {
    for (const n of ['PASSWORD', 'DB_PASSWORD', 'HF_TOKEN', 'API_KEY', 'APIKEY', 'aws_secret', 'PRIVATE_KEY', 'CREDENTIALS']) expect(isSensitiveEnv(n)).toBe(true)
    for (const n of ['KEYBOARD', 'MONKEY', 'AUTHOR', 'DATABASE_URL', 'CONNECTIONS', 'BATCH_SIZE', 'TOKENIZERS_PARALLELISM']) expect(isSensitiveEnv(n)).toBe(false)
  })
  it('describes references', () => {
    expect(envSource({ name: 'A', value: 'x' })).toBeUndefined()
    expect(envSource({ name: 'A', valueFrom: { secretKeyRef: { name: 'db', key: 'pw' } } })).toBe('from secret/db (pw)')
    expect(envSource({ name: 'A', valueFrom: { configMapKeyRef: { name: 'cfg' } } })).toBe('from configmap/cfg')
  })
})
