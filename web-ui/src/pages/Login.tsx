import { useState, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/lib/auth'

export default function Login() {
  const { isAuthenticated, authConfig, loginWithPassword, loginWithSSO, error: authError } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/dashboard'

  useEffect(() => {
    if (isAuthenticated) {
      navigate(from, { replace: true })
    }
  }, [isAuthenticated, navigate, from])

  const error = localError || authError

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLocalError(null)
    setIsSubmitting(true)
    try {
      await loginWithPassword(username, password)
      navigate(from, { replace: true })
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'Authentication failed')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSSOLogin = () => {
    setLocalError(null)
    loginWithSSO()
  }

  return (
    <div className="login-shell">
      <section>
        <img className="brand-mark" src="/gryvia-mark.svg" alt="" style={{ width: 56, height: 56, margin: '0 auto 20px', borderRadius: 'var(--radius-lg)' }} />
        <p className="eyebrow">Gryvia</p>
        <h1 className="apple-display">Sign in.</h1>
        <p className="apple-lede" style={{ margin: '12px auto 0' }}>Open GPU orchestration for Kubernetes.</p>
      </section>

      <div className="card login-card">
        {error && <p className="login-error" role="alert">{error}</p>}

        {authConfig?.oidcEnabled && (
          <button className="primary" onClick={handleSSOLogin}>
            Sign in with SSO
          </button>
        )}

        {authConfig?.oidcEnabled && authConfig?.apiKeyEnabled && <p className="faint" style={{ textAlign: 'center', margin: 0 }}>or</p>}

        {authConfig?.apiKeyEnabled !== false && (
          <form onSubmit={handleSubmit} className="stack" style={{ gap: 14 }} noValidate>
            <label htmlFor="username">
              Username
              <input
                id="username"
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value)
                  setLocalError(null)
                }}
                autoFocus
                autoComplete="username"
                disabled={isSubmitting}
                aria-invalid={Boolean(error)}
              />
            </label>
            <label htmlFor="password">
              Password
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value)
                  setLocalError(null)
                }}
                autoComplete="current-password"
                disabled={isSubmitting}
                aria-invalid={Boolean(error)}
              />
            </label>
            <button type="submit" className={authConfig?.oidcEnabled ? 'btn-secondary' : 'primary'} disabled={isSubmitting}>
              {isSubmitting ? 'Signing in…' : 'Sign in'}
            </button>
          </form>
        )}

        {authConfig === null && <p className="faint" style={{ textAlign: 'center' }}>Loading authentication…</p>}
      </div>
    </div>
  )
}
