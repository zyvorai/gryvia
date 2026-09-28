import { useState, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '@/lib/auth'

export default function Login() {
  const { isAuthenticated, authConfig, loginWithApiKey, loginWithSSO, error: authError } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const [apiKey, setApiKey] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/dashboard'

  useEffect(() => {
    if (isAuthenticated) {
      navigate(from, { replace: true })
    }
  }, [isAuthenticated, navigate, from])

  const error = localError || authError

  const handleApiKeySubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!apiKey.trim()) {
      setLocalError('Please enter an API key')
      return
    }
    setLocalError(null)
    setIsSubmitting(true)
    try {
      await loginWithApiKey(apiKey.trim())
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
          <form onSubmit={handleApiKeySubmit} className="stack" style={{ gap: 14 }}>
            <label htmlFor="api-key">
              API key
              <input
                id="api-key"
                type="password"
                value={apiKey}
                onChange={(e) => {
                  setApiKey(e.target.value)
                  setLocalError(null)
                }}
                placeholder="Enter your API key"
                autoComplete="current-password"
              />
            </label>
            <button type="submit" className={authConfig?.oidcEnabled ? 'btn-secondary' : 'primary'} disabled={isSubmitting || !apiKey.trim()}>
              {isSubmitting ? 'Authenticating…' : 'Sign in with API key'}
            </button>
          </form>
        )}

        {authConfig === null && <p className="faint" style={{ textAlign: 'center' }}>Loading authentication…</p>}
      </div>
    </div>
  )
}
