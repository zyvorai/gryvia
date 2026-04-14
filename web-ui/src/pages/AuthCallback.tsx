import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Loader2, AlertCircle } from 'lucide-react'
import { useAuth } from '@/lib/auth'

export default function AuthCallback() {
  const { handleOIDCCallback, isAuthenticated, error: authError } = useAuth()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const [processed, setProcessed] = useState(false)

  useEffect(() => {
    if (processed) return

    const code = searchParams.get('code')
    const errorParam = searchParams.get('error')
    const errorDescription = searchParams.get('error_description')

    if (errorParam) {
      setProcessed(true)
      // Show error and redirect to login after a delay
      setTimeout(() => navigate('/login', { replace: true }), 3000)
      return
    }

    if (code) {
      setProcessed(true)
      handleOIDCCallback(code)
    } else {
      // No code and no error - redirect to login
      navigate('/login', { replace: true })
    }
  }, [searchParams, handleOIDCCallback, navigate, processed])

  // Redirect to dashboard once authenticated
  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard', { replace: true })
    }
  }, [isAuthenticated, navigate])

  const urlError = searchParams.get('error_description') || searchParams.get('error')
  const displayError = urlError || authError

  return (
    <div
      className="min-h-screen flex items-center justify-center"
      style={{ background: '#0a0e14' }}
    >
      <div className="text-center animate-fade-in">
        {displayError ? (
          <div className="flex flex-col items-center gap-4">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center"
              style={{
                background: 'rgba(239,68,68,0.15)',
                border: '1px solid rgba(239,68,68,0.2)',
              }}
            >
              <AlertCircle className="h-8 w-8 text-red-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[#d0dae6] mb-2">
                Authentication Failed
              </h2>
              <p className="text-sm text-[#8090a8] max-w-sm">{displayError}</p>
            </div>
            <p className="text-xs text-[#5a7a9e] mt-4">Redirecting to login...</p>
          </div>
        ) : (
          <div className="flex flex-col items-center gap-4">
            <div
              className="w-16 h-16 rounded-2xl flex items-center justify-center"
              style={{
                background: 'linear-gradient(135deg, #d4764e 0%, #e8a87c 50%, #d4764e 100%)',
                boxShadow: '0 4px 20px rgba(212,118,78,0.4)',
              }}
            >
              <span className="text-[#0a0e14] font-bold text-2xl">TR</span>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[#d0dae6] mb-2">
                Completing sign in...
              </h2>
              <Loader2 className="h-6 w-6 animate-spin text-[#d4764e] mx-auto" />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
