import { useEffect, useId, useState } from 'react'
import type { FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { ArrowRight, Check, ChevronLeft, Copy, Eye, EyeOff, Loader2, Lock, User } from 'lucide-react'
import { kubectlCommand, useAuth } from '@/lib/auth'
import { LoginError, LoginField, LoginShell } from '@/components/LoginShell'
import { ZyvorLockup, ZYVOR_URL } from '@/components/ZyvorMark'

const SAVE_KEY = 'gryvia-saved-login'

/** Chart defaults, for gateways that predate `credentials` in /api/auth/config. */
const DEFAULT_CREDENTIALS = { username: 'admin', secret: 'gryvia-api-key', key: 'GRYVIA_API_KEY', namespace: 'gryvia-system' }

function savedUsername(): string {
  try {
    return (JSON.parse(localStorage.getItem(SAVE_KEY) || '{}') as { username?: string }).username || ''
  } catch {
    return ''
  }
}

function CopyCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      setCopied(false)
    }
  }
  return (
    <div className="login-cred">
      <pre aria-label="Command that prints the admin password">{command}</pre>
      <button type="button" className="login-copy" onClick={copy} aria-label="Copy command">
        {copied ? <Check size={14} aria-hidden /> : <Copy size={14} aria-hidden />}
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

/** Zorvia's sign-in: hero chapter, then username, then password; the hint says where the admin key lives. */
export default function Login() {
  const { isAuthenticated, authConfig, loginWithPassword, loginWithSSO, error: authError } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const errorId = useId()

  const remembered = savedUsername()
  const [step, setStep] = useState<'identify' | 'password'>(remembered ? 'password' : 'identify')
  const [username, setUsername] = useState(remembered)
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [rememberMe, setRememberMe] = useState(Boolean(remembered))
  const [busy, setBusy] = useState(false)
  const [localError, setLocalError] = useState('')

  const state = location.state as { from?: { pathname?: string; search?: string; hash?: string }; reason?: string } | null
  const from = state?.from?.pathname ? `${state.from.pathname}${state.from.search ?? ''}${state.from.hash ?? ''}` : '/dashboard'
  const expired = state?.reason === 'expired'

  useEffect(() => {
    if (isAuthenticated) navigate(from, { replace: true })
  }, [isAuthenticated, navigate, from])

  const passwordLogin = authConfig?.apiKeyEnabled !== false
  const sso = Boolean(authConfig?.oidcEnabled)
  const creds = authConfig?.credentials ?? DEFAULT_CREDENTIALS
  const host = window.location.host || window.location.hostname
  const namespace = authConfig?.instance?.namespace ?? creds.namespace
  const version = authConfig?.instance?.version
  const error = localError || authError || ''

  const toPassword = (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim()) return
    setLocalError('')
    setStep('password')
  }

  const back = () => {
    setStep('identify')
    setPassword('')
    setShowPassword(false)
    setLocalError('')
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password) return
    setLocalError('')
    setBusy(true)
    try {
      await loginWithPassword(username.trim(), password)
      if (rememberMe) localStorage.setItem(SAVE_KEY, JSON.stringify({ username: username.trim() }))
      else localStorage.removeItem(SAVE_KEY)
      navigate(from, { replace: true })
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'Authentication failed')
    } finally {
      setBusy(false)
    }
  }

  const scrollToForm = () => document.getElementById('login-sign-in')?.scrollIntoView({ behavior: 'smooth', block: 'start' })

  const instanceDetails = (
    <dl className="login-instance">
      <dt>Project</dt>
      <dd>Gryvia{version ? ` ${version}` : ''}</dd>
      <dt>System</dt>
      <dd>{host}</dd>
      <dt>Deploy</dt>
      <dd>Kubernetes · {namespace}</dd>
    </dl>
  )

  const notices = (
    <>
      {expired && !error ? (
        <p className="login-notice" role="status">
          Your session expired. Sign in again to continue.
        </p>
      ) : null}
      {error ? <LoginError id={errorId} message={error} /> : null}
    </>
  )

  return (
    <LoginShell
      brand={
        <a href={ZYVOR_URL} target="_blank" rel="noopener noreferrer" className="login-brand" aria-label="zyvor.dev">
          <ZyvorLockup />
          <span className="login-brand-product">Gryvia</span>
        </a>
      }
      wordmark="Gryvia"
      instanceMeta={
        <>
          <span className="login-instance-chip" data-kind="product">
            <strong>Gryvia</strong>
          </span>
          <span className="login-instance-chip" title="Host">
            {host}
          </span>
          <span className="login-instance-chip" data-kind="k8s" title="Namespace">
            Kubernetes · ns/{namespace}
          </span>
          {version ? (
            <span className="login-instance-chip" title="Version">
              v{version}
            </span>
          ) : null}
        </>
      }
      heroTitle="Open GPU orchestration. One console."
      heroLede="Schedule, share and observe GPU workloads with operators and CRDs you already know how to run: jobs, quotas, nodes, models and network intelligence in one control plane."
      pills={[
        { label: 'Zyvor', tone: 'orange' },
        { label: 'Kubernetes', tone: 'violet' },
        { label: 'GPUs', tone: 'emerald' },
        { label: 'Sovereign AI OS', tone: 'sky' },
      ]}
      heroCta={
        <a
          href="#login-sign-in"
          className="login-cta-primary"
          onClick={(e) => {
            e.preventDefault()
            scrollToForm()
          }}
        >
          Sign in
        </a>
      }
      chapterNote={`Gryvia · Kubernetes · ${host}`}
      formHeading={
        step === 'password' ? (
          <>
            Enter the password for <strong>{username.trim()}</strong>
          </>
        ) : (
          'Sign in to Gryvia'
        )
      }
      hint={
        passwordLogin && step === 'identify' ? (
          <>
            <p>
              Sign in as <code>{creds.username}</code>. The password is in Secret <code>{creds.secret}</code> (key{' '}
              <code>{creds.key}</code>) in namespace <code>{creds.namespace}</code>:
            </p>
            <CopyCommand command={kubectlCommand(creds)} />
          </>
        ) : null
      }
      footer={
        <p className="login-footer">
          © 2026 Zyvor ·{' '}
          <a href={ZYVOR_URL} target="_blank" rel="noopener noreferrer" className="login-zyvor-link">
            zyvor.dev
          </a>
        </p>
      }
    >
      {step === 'identify' || !passwordLogin ? (
        <form key="identify" className="login-step" onSubmit={toPassword} aria-label="Choose your account" noValidate>
          {instanceDetails}
          {notices}
          {sso ? (
            <>
              <button type="button" className="login-btn-secondary" onClick={() => loginWithSSO()}>
                Sign in with SSO
              </button>
              {passwordLogin ? <div className="login-divider">or</div> : null}
            </>
          ) : null}
          {passwordLogin ? (
            <>
              <LoginField label="Username" id="username">
                <User className="login-field-icon" aria-hidden />
                <input
                  id="username"
                  name="username"
                  className="login-input"
                  value={username}
                  onChange={(e) => {
                    setUsername(e.target.value)
                    if (localError) setLocalError('')
                  }}
                  placeholder={creds.username}
                  autoComplete="username"
                  autoFocus
                  aria-invalid={Boolean(error)}
                  aria-describedby={error ? errorId : undefined}
                />
              </LoginField>
              <button type="submit" className="login-btn-primary" disabled={!username.trim()}>
                Continue <ArrowRight size={16} aria-hidden />
              </button>
            </>
          ) : null}
        </form>
      ) : (
        <form key="password" className="login-step" onSubmit={submit} aria-label="Enter your password" noValidate aria-busy={busy}>
          <button type="button" className="login-identity" onClick={back} aria-label="Change account">
            <ChevronLeft size={16} aria-hidden />
            <span>{username.trim()}</span>
          </button>
          <input type="text" name="username" value={username} autoComplete="username" readOnly hidden />
          {instanceDetails}
          {notices}
          <LoginField label="Password" id="password">
            <Lock className="login-field-icon" aria-hidden />
            <input
              id="password"
              name="password"
              className="login-input"
              type={showPassword ? 'text' : 'password'}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value)
                if (localError) setLocalError('')
              }}
              placeholder="Password"
              autoComplete="current-password"
              autoFocus
              disabled={busy}
              aria-invalid={Boolean(error)}
              aria-describedby={error ? errorId : undefined}
            />
            <button
              type="button"
              className="login-eye"
              onClick={() => setShowPassword((s) => !s)}
              aria-label={showPassword ? 'Hide password' : 'Show password'}
            >
              {showPassword ? <EyeOff size={16} aria-hidden /> : <Eye size={16} aria-hidden />}
            </button>
          </LoginField>
          <label className="login-remember">
            <input
              type="checkbox"
              checked={rememberMe}
              onChange={(e) => {
                setRememberMe(e.target.checked)
                if (!e.target.checked) localStorage.removeItem(SAVE_KEY)
              }}
            />
            Remember me on this device
          </label>
          <button type="submit" className="login-btn-primary" disabled={!password || busy}>
            {busy ? (
              <>
                <Loader2 size={16} className="login-spin" aria-hidden /> Signing in…
              </>
            ) : (
              <>
                Sign in <ArrowRight size={16} aria-hidden />
              </>
            )}
          </button>
        </form>
      )}
    </LoginShell>
  )
}
