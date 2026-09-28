import { useEffect, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '@/lib/auth'

export default function AuthCallback() {
  const { handleOIDCCallback, isAuthenticated, error: authError } = useAuth()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const processed = useRef(false)

  useEffect(() => {
    if (processed.current) return

    const code = searchParams.get('code')
    const errorParam = searchParams.get('error')

    if (errorParam) {
      processed.current = true
      // Show error and redirect to login after a delay
      setTimeout(() => navigate('/login', { replace: true }), 3000)
      return
    }

    if (code) {
      processed.current = true
      handleOIDCCallback(code)
    } else {
      // No code and no error - redirect to login
      navigate('/login', { replace: true })
    }
  }, [searchParams, handleOIDCCallback, navigate])

  // Redirect to dashboard once authenticated
  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard', { replace: true })
    }
  }, [isAuthenticated, navigate])

  const urlError = searchParams.get('error_description') || searchParams.get('error')
  const displayError = urlError || authError

  return (
    <div className="login-shell">
      {displayError ? (
        <div className="stack" style={{ alignItems: 'center' }}>
          <h1 className="apple-display">Authentication failed.</h1>
          <p className="login-error" role="alert">{displayError}</p>
          <p className="faint">Redirecting to login...</p>
        </div>
      ) : (
        <div className="stack" style={{ alignItems: 'center' }}>
          <h1 className="apple-display">Completing sign in...</h1>
          <div className="spinner" role="status" aria-label="Loading" />
        </div>
      )}
    </div>
  )
}
