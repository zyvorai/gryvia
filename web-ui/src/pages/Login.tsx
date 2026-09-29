import { useState, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/lib/auth'

/** Same composition as netra's Login (styled by netra.css: .login-shell/.login-info/.login-card). */
export default function Login() {
  const { isAuthenticated, authConfig, loginWithPassword, loginWithSSO, error: authError } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [localError, setLocalError] = useState('')
  const host = window.location.host || window.location.hostname

  const state = location.state as { from?: { pathname?: string; search?: string; hash?: string }; reason?: string } | null
  const from = state?.from?.pathname ? `${state.from.pathname}${state.from.search ?? ''}${state.from.hash ?? ''}` : '/dashboard'
  const expired = state?.reason === 'expired'

  useEffect(() => {
    if (isAuthenticated) {
      navigate(from, { replace: true })
    }
  }, [isAuthenticated, navigate, from])

  const error = localError || authError || ''

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLocalError('')
    setBusy(true)
    try {
      await loginWithPassword(username, password)
      navigate(from, { replace: true })
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'Authentication failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="login-shell">
      <div className="login-info">
        <img src="/gryvia-mark.svg" alt="Gryvia" className="login-logo" />
        <p className="eyebrow">Gryvia</p>
        <h1>Open GPU orchestration for Kubernetes.</h1>
        <p>
          Schedule, share and observe GPU workloads with operators and CRDs you already know how to run — one control
          plane for jobs, quotas, nodes and network intelligence.
        </p>
        <p className="login-host">
          Connecting to <code>{host}</code>
        </p>
      </div>
      <form className="card login-card" onSubmit={submit} noValidate>
        <h1>Sign in.</h1>
        {expired && !error && (
          <p className="warning" role="status">
            Your session expired. Sign in again to continue.
          </p>
        )}
        {authConfig?.oidcEnabled && (
          <button type="button" className="primary" onClick={() => loginWithSSO()}>
            Sign in with SSO
          </button>
        )}
        {authConfig?.apiKeyEnabled !== false && (
          <>
            <label className="tokenbox">
              Username
              <input
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value)
                  if (localError) setLocalError('')
                }}
                autoFocus
                autoComplete="username"
                disabled={busy}
                aria-invalid={Boolean(error)}
              />
            </label>
            <label className="tokenbox">
              Password
              <input
                type="password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value)
                  if (localError) setLocalError('')
                }}
                autoComplete="current-password"
                disabled={busy}
                aria-invalid={Boolean(error)}
              />
            </label>
          </>
        )}
        {error ? (
          <p className="login-error" role="alert" aria-live="assertive">
            {error}
          </p>
        ) : null}
        {authConfig?.apiKeyEnabled !== false && (
          <button type="submit" className={authConfig?.oidcEnabled ? 'btn-secondary' : 'primary'} disabled={busy}>
            {busy ? 'Signing in…' : 'Sign in'}
          </button>
        )}
      </form>
    </div>
  )
}
