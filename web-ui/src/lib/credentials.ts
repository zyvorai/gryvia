// Friendly username + password login over the gateway's single shared bearer
// (GRYVIA_API_KEY). Same model as netra: the pair below maps to the deploy
// default bearer, and any other password for `admin` is tried as the bearer
// itself, so an operator with a custom key signs in with that key as the password.

export const DEFAULT_USERNAME = 'admin'
export const DEFAULT_PASSWORD = 'Admin@321'
/** Bearer the deploy script configures by default (GRYVIA_API_KEY overrides it). */
const DEFAULT_BEARER = 'Admin@321'

/**
 * Bearer tokens to try for a login, in order. Empty means reject before any
 * network call (wrong username or empty password).
 */
export function bearerCandidates(username: string, password: string): string[] {
  if (username !== DEFAULT_USERNAME || password === '') return []
  const out: string[] = []
  if (password === DEFAULT_PASSWORD) out.push(DEFAULT_BEARER)
  if (!out.includes(password)) out.push(password)
  return out
}
