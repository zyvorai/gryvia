import type { ReactNode } from 'react'
import { AlertCircle } from 'lucide-react'
import '@/styles/zyvor-login.css'

/** Zorvia's chapter login (PremiumLoginShell): a hero chapter, then a credentials chapter, then the footer. */

export type LoginTone = 'sky' | 'violet' | 'emerald' | 'amber' | 'orange'

export interface LoginShellProps {
  brand: ReactNode
  wordmark: string
  instanceMeta?: ReactNode
  heroTitle: ReactNode
  heroLede: ReactNode
  pills: { label: string; tone: LoginTone }[]
  heroCta?: ReactNode
  chapterNote?: ReactNode
  formHeading: ReactNode
  hint?: ReactNode
  footer?: ReactNode
  children: ReactNode
}

export function LoginShell(props: LoginShellProps) {
  return (
    <div className="login-page" data-tone="orange">
      <main className="login-scroll" aria-label="Sign in">
        <section className="login-chapter login-chapter-hero" aria-label={props.wordmark}>
          <div className="login-chapter-inner">
            {props.brand}
            <p className="login-wordmark">{props.wordmark}</p>
            {props.instanceMeta ? <div className="login-instance-meta">{props.instanceMeta}</div> : null}
            <h1 className="login-hero-title">{props.heroTitle}</h1>
            <p className="login-tagline">{props.heroLede}</p>
            <div className="login-pill-row">
              {props.pills.map((pill) => (
                <span key={pill.label} data-tone={pill.tone} className="login-pill">
                  <span className="login-pill-dot" aria-hidden />
                  {pill.label}
                </span>
              ))}
            </div>
            {props.heroCta ? <div className="login-cta">{props.heroCta}</div> : null}
            {props.chapterNote ? <p className="login-chapter-note">{props.chapterNote}</p> : null}
          </div>
        </section>
        <section id="login-sign-in" className="login-chapter login-chapter-sign-in" aria-label="Credentials">
          <div className="login-chapter-inner login-sign-in-inner">
            <h2 className="login-form-heading">{props.formHeading}</h2>
            <div className="login-panel">{props.children}</div>
            {props.hint ? <div className="login-hint">{props.hint}</div> : null}
          </div>
        </section>
        {props.footer}
      </main>
    </div>
  )
}

export function LoginError({ id, message }: { id?: string; message: string }) {
  return (
    <div id={id} className="login-alert" role="alert" aria-live="assertive">
      <AlertCircle size={16} aria-hidden />
      <div>
        <p className="login-alert-title">Unable to sign in</p>
        <p className="login-alert-text">{message}</p>
      </div>
    </div>
  )
}

export function LoginField({ label, id, children }: { label: string; id: string; children: ReactNode }) {
  return (
    <div className="login-field">
      <label htmlFor={id} className="login-field-label">
        {label}
      </label>
      <div className="login-field-control">{children}</div>
    </div>
  )
}
