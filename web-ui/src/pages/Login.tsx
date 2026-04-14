import { useState, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { KeyRound, Shield, AlertCircle, Loader2, ExternalLink } from 'lucide-react'
import { useAuth } from '@/lib/auth'

export default function Login() {
  const { isAuthenticated, authConfig, loginWithApiKey, loginWithSSO, error: authError } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const [apiKey, setApiKey] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  const from = (location.state as any)?.from?.pathname || '/dashboard'

  // Redirect if already authenticated
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
    } catch (err: any) {
      setLocalError(err.message || 'Authentication failed')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSSOLogin = () => {
    setLocalError(null)
    loginWithSSO()
  }

  return (
    <div
      className="min-h-screen flex items-center justify-center p-4"
      style={{
        background: `
          radial-gradient(ellipse at 30% 20%, rgba(212,118,78,0.06) 0%, transparent 50%),
          radial-gradient(ellipse at 70% 80%, rgba(95,168,211,0.04) 0%, transparent 50%),
          #0a0e14
        `,
      }}
    >
      <div className="w-full max-w-md animate-fade-in">
        {/* Logo and branding */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center mb-4">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center relative"
              style={{
                background: 'linear-gradient(135deg, #d4764e 0%, #e8a87c 50%, #d4764e 100%)',
                boxShadow:
                  '0 4px 20px rgba(212,118,78,0.4), inset 0 1px 0 rgba(255,255,255,0.2)',
              }}
            >
              <span className="text-[#0a0e14] font-bold text-2xl">TR</span>
            </div>
          </div>
          <h1 className="text-3xl font-bold text-gradient-copper mb-2">TensorReaper</h1>
          <p className="text-[#8090a8] text-sm">GPU Cluster Management Platform</p>
        </div>

        {/* Login card */}
        <div
          className="rounded-2xl p-8"
          style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.08)',
            boxShadow:
              '0 25px 60px rgba(0,0,0,0.5), inset 0 1px 0 rgba(192,204,224,0.06)',
          }}
        >
          {/* Error message */}
          {error && (
            <div
              className="flex items-start gap-3 p-3 rounded-lg mb-6"
              style={{
                background: 'rgba(239,68,68,0.1)',
                border: '1px solid rgba(239,68,68,0.2)',
              }}
            >
              <AlertCircle className="h-5 w-5 text-red-400 flex-shrink-0 mt-0.5" />
              <p className="text-red-300 text-sm">{error}</p>
            </div>
          )}

          {/* SSO button */}
          {authConfig?.oidcEnabled && (
            <>
              <button
                onClick={handleSSOLogin}
                className="w-full flex items-center justify-center gap-2.5 px-4 py-3 rounded-lg text-sm font-semibold transition-all duration-200"
                style={{
                  background:
                    'linear-gradient(135deg, rgba(95,168,211,0.15) 0%, rgba(95,168,211,0.05) 100%)',
                  border: '1px solid rgba(95,168,211,0.25)',
                  color: '#7ecbf5',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background =
                    'linear-gradient(135deg, rgba(95,168,211,0.25) 0%, rgba(95,168,211,0.1) 100%)'
                  e.currentTarget.style.borderColor = 'rgba(95,168,211,0.4)'
                  e.currentTarget.style.boxShadow = '0 0 20px rgba(95,168,211,0.15)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background =
                    'linear-gradient(135deg, rgba(95,168,211,0.15) 0%, rgba(95,168,211,0.05) 100%)'
                  e.currentTarget.style.borderColor = 'rgba(95,168,211,0.25)'
                  e.currentTarget.style.boxShadow = 'none'
                }}
              >
                <Shield className="h-5 w-5" />
                Sign in with SSO
                <ExternalLink className="h-3.5 w-3.5 ml-1 opacity-60" />
              </button>

              {/* Divider */}
              {authConfig?.apiKeyEnabled && (
                <div className="flex items-center gap-4 my-6">
                  <div className="flex-1 h-px" style={{ background: 'rgba(192,204,224,0.08)' }} />
                  <span className="text-xs text-[#5a7a9e] uppercase tracking-wider font-medium">
                    or
                  </span>
                  <div className="flex-1 h-px" style={{ background: 'rgba(192,204,224,0.08)' }} />
                </div>
              )}
            </>
          )}

          {/* API Key form */}
          {authConfig?.apiKeyEnabled !== false && (
            <form onSubmit={handleApiKeySubmit}>
              <label
                htmlFor="api-key"
                className="block text-sm font-medium text-[#8ba4c0] mb-2"
              >
                <KeyRound className="h-3.5 w-3.5 inline mr-1.5 -mt-0.5" />
                API Key
              </label>
              <div className="relative mb-4">
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
                  className="w-full px-4 py-3 rounded-lg text-sm text-[#d0dae6] placeholder-[#5a7a9e] focus:outline-none focus:ring-2 focus:ring-[#d4764e]/40 transition-all"
                  style={{
                    background: 'rgba(10,14,20,0.6)',
                    border: '1px solid rgba(192,204,224,0.08)',
                    boxShadow: 'inset 0 2px 4px rgba(0,0,0,0.3)',
                  }}
                  onFocus={(e) => {
                    e.currentTarget.style.borderColor = 'rgba(212,118,78,0.3)'
                  }}
                  onBlur={(e) => {
                    e.currentTarget.style.borderColor = 'rgba(192,204,224,0.08)'
                  }}
                />
              </div>
              <button
                type="submit"
                disabled={isSubmitting || !apiKey.trim()}
                className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-lg text-sm font-semibold transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed"
                style={{
                  background: 'linear-gradient(135deg, #d4764e 0%, #c4653d 100%)',
                  color: '#0a0e14',
                  boxShadow:
                    '0 2px 8px rgba(212,118,78,0.3), inset 0 1px 0 rgba(255,255,255,0.15)',
                }}
              >
                {isSubmitting ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Authenticating...
                  </>
                ) : (
                  <>
                    <KeyRound className="h-4 w-4" />
                    Sign in with API Key
                  </>
                )}
              </button>
            </form>
          )}

          {/* Loading state when config hasn't loaded yet */}
          {authConfig === null && (
            <div className="flex items-center justify-center py-8 gap-3">
              <Loader2 className="h-5 w-5 animate-spin text-[#5a7a9e]" />
              <span className="text-sm text-[#5a7a9e]">Loading authentication...</span>
            </div>
          )}
        </div>

        {/* Footer */}
        <p className="text-center text-xs text-[#5a7a9e] mt-6">
          TensorReaper v1.0 &middot; Fabric GPU Orchestration
        </p>
      </div>
    </div>
  )
}
