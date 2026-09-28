import { ReactNode, useState, useRef, useEffect } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { LogOut, Menu, Moon, Sun, X } from 'lucide-react'
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

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, logout } = useAuth()
  const [menuOpen, setMenuOpen] = useState(false)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const [theme, setTheme] = useState<Theme>(readStoredTheme)
  const userMenuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (userMenuRef.current && !userMenuRef.current.contains(event.target as Node)) {
        setUserMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleLogout = () => {
    setUserMenuOpen(false)
    logout()
    navigate('/login', { replace: true })
  }

  const displayName = user?.name || user?.email || 'User'

  const isActive = (href: string) =>
    href === '/dashboard'
      ? location.pathname === '/dashboard' || location.pathname === '/'
      : location.pathname.startsWith(href)

  return (
    <>
      <nav className="nav" aria-label="Primary">
        <div className="nav-inner">
          <Link to="/dashboard" className="brand">
            <img className="brand-mark" src="/gryvia-mark.svg" alt="" />
            Gryvia
          </Link>

          <div className={menuOpen ? 'navlinks open' : 'navlinks'}>
            {navigation.map((item) => (
              <NavLink
                key={item.name}
                to={item.href}
                className={isActive(item.href) ? 'active' : undefined}
                aria-current={isActive(item.href) ? 'page' : undefined}
                onClick={() => setMenuOpen(false)}
              >
                {item.name}
              </NavLink>
            ))}
          </div>

          <div className="nav-actions" ref={userMenuRef}>
            <span className="live-dot">
              <i /> Live
            </span>
            <button
              className="icon-button"
              onClick={() => setTheme(toggleTheme(theme))}
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <button
              className="icon-button"
              onClick={() => setUserMenuOpen(!userMenuOpen)}
              aria-label="Account menu"
              aria-expanded={userMenuOpen}
              style={{ fontWeight: 600, fontSize: 13 }}
            >
              {displayName.charAt(0).toUpperCase()}
            </button>
            <button
              className="icon-button nav-menu-toggle"
              onClick={() => setMenuOpen(!menuOpen)}
              aria-label={menuOpen ? 'Close menu' : 'Open menu'}
              aria-expanded={menuOpen}
            >
              {menuOpen ? <X size={18} /> : <Menu size={18} />}
            </button>

            {userMenuOpen && (
              <div className="menu" role="menu">
                <div className="menu-head">
                  <b>{displayName}</b>
                  {user?.email && <span>{user.email}</span>}
                  <span>{user?.method === 'oidc' ? 'Signed in with SSO' : 'Signed in with API key'}</span>
                </div>
                <button onClick={handleLogout} role="menuitem">
                  <LogOut size={16} /> Sign out
                </button>
              </div>
            )}
          </div>
        </div>
      </nav>
      <main>{children}</main>
    </>
  )
}
