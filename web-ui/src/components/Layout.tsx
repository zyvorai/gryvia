import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '@/lib/auth'
import { applyTheme, readStoredTheme, toggleTheme, type Theme } from '@/theme'

interface LayoutProps {
  children: ReactNode
}

const navigation = [
  { name: 'Dashboard', href: '/dashboard' },
  { name: 'Jobs', href: '/jobs' },
  { name: 'Workspaces', href: '/workspaces' },
  { name: 'Models', href: '/models' },
  { name: 'Inference', href: '/inference' },
  { name: 'Workflows', href: '/workflows' },
  { name: 'Tuner', href: '/tuner' },
  { name: 'Quotas', href: '/quotas' },
  { name: 'Nodes', href: '/nodes' },
  { name: 'Network', href: '/network' },
  { name: 'Security', href: '/security' },
  { name: 'GPU', href: '/gpu' },
  { name: 'Costs', href: '/costs' },
]

/** Same markup and classes as netra's Nav (styled by netra.css). */
export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { logout } = useAuth()
  const [theme, setTheme] = useState<Theme>(readStoredTheme)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  const isActive = (href: string) =>
    href === '/dashboard'
      ? location.pathname === '/dashboard' || location.pathname === '/'
      : location.pathname.startsWith(href)

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <>
      <nav className="nav" aria-label="Global">
        <div className="nav-inner">
          <button type="button" className="brand" onClick={() => navigate('/dashboard')} aria-label="Gryvia home">
            <img src="/gryvia-logomark.svg" alt="" className="brand-mark" aria-hidden />
            Gryvia
          </button>
          <div className="navlinks">
            {navigation.map((item) => (
              <button
                key={item.name}
                type="button"
                className={isActive(item.href) ? 'active' : ''}
                aria-current={isActive(item.href) ? 'page' : undefined}
                onClick={() => navigate(item.href)}
              >
                {item.name}
              </button>
            ))}
          </div>
          <div className="nav-actions">
            <button type="button" className="theme-toggle" onClick={handleLogout} aria-label="Log out" title="Log out">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                <path d="M15 4H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h8" strokeLinecap="round" strokeLinejoin="round" />
                <path d="M10 12h11m0 0-3.5-3.5M21 12l-3.5 3.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
            <button
              type="button"
              className="theme-toggle"
              onClick={() => setTheme(toggleTheme(theme))}
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
              title={theme === 'dark' ? 'Light' : 'Dark'}
            >
              {theme === 'dark' ? (
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                  <circle cx="12" cy="12" r="4" />
                  <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
                </svg>
              ) : (
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden>
                  <path d="M21 14.5A8.5 8.5 0 1 1 11.5 3a7 7 0 0 0 9.5 11.5z" />
                </svg>
              )}
            </button>
          </div>
        </div>
      </nav>
      <main>
        <div key={location.pathname}>{children}</div>
      </main>
    </>
  )
}
