import { ReactNode, useState, useRef, useEffect } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import {
  LayoutDashboard,
  Briefcase,
  Users,
  Server,
  DollarSign,
  Plus,
  Menu,
  X,
  Activity,
  LogOut,
  ChevronDown,
  Network,
} from 'lucide-react'
import { useAuth } from '@/lib/auth'

interface LayoutProps {
  children: ReactNode
}

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { name: 'Jobs', href: '/jobs', icon: Briefcase },
  { name: 'Quotas', href: '/quotas', icon: Users },
  { name: 'Nodes', href: '/nodes', icon: Server },
  { name: 'Network', href: '/network', icon: Network },
  { name: 'Costs', href: '/costs', icon: DollarSign },
]

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { user, logout } = useAuth()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const userMenuRef = useRef<HTMLDivElement>(null)

  // Close user menu when clicking outside
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
  const displayInitial = displayName.charAt(0).toUpperCase()

  const isActive = (href: string) => {
    if (href === '/dashboard') return location.pathname === '/dashboard' || location.pathname === '/'
    return location.pathname.startsWith(href)
  }

  return (
    <div className="h-screen flex flex-col" style={{ background: '#0a0e14' }}>
      {/* Top Navbar - Frosted Titanium */}
      <header className="sticky top-0 z-50 navbar-gradient flex-shrink-0 relative">
        <div className="flex items-center h-14 px-4">
          {/* Logo */}
          <Link to="/dashboard" className="flex items-center gap-2.5 mr-8 flex-shrink-0 group">
            <div className="w-8 h-8 rounded-lg flex items-center justify-center relative overflow-hidden"
              style={{
                background: 'linear-gradient(135deg, #d4764e 0%, #e8a87c 50%, #d4764e 100%)',
                boxShadow: '0 2px 8px rgba(212,118,78,0.3), inset 0 1px 0 rgba(255,255,255,0.2)',
              }}
            >
              <span className="text-[#0a0e14] font-bold text-sm relative z-10">TR</span>
            </div>
            <h1 className="text-xl font-bold text-gradient-copper">
              TensorReaper
            </h1>
          </Link>

          {/* Nav items (desktop) */}
          <nav className="hidden md:flex items-center gap-1 flex-1">
            {navigation.map((item) => {
              const Icon = item.icon
              const active = isActive(item.href)
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
                    active
                      ? 'text-[#e8a87c]'
                      : 'text-[#8090a8] hover:text-[#c0cce0] hover:bg-[#1a2332]/60'
                  }`}
                  style={active ? {
                    background: 'linear-gradient(135deg, rgba(212,118,78,0.12) 0%, rgba(212,118,78,0.04) 100%)',
                    boxShadow: 'inset 0 1px 0 rgba(212,118,78,0.1)',
                  } : undefined}
                >
                  <Icon className="h-4 w-4" />
                  {item.name}
                </Link>
              )
            })}
          </nav>

          {/* Right section */}
          <div className="flex items-center gap-3 ml-auto">
            <Link
              to="/jobs/new"
              className="hidden sm:inline-flex items-center gap-1.5 px-3.5 py-1.5 btn-copper text-sm rounded-lg"
            >
              <Plus className="h-3.5 w-3.5" />
              Submit Job
            </Link>

            <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg"
              style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(34,197,94,0.15)',
              }}
            >
              <Activity className="h-3.5 w-3.5 text-emerald-400 animate-pulse-dot" />
              <span className="text-xs text-[#8090a8]">Live</span>
            </div>

            {/* User menu */}
            <div className="relative" ref={userMenuRef}>
              <button
                onClick={() => setUserMenuOpen(!userMenuOpen)}
                className="flex items-center gap-2 px-2 py-1.5 rounded-lg transition-all duration-200 hover:bg-[#1a2332]/60"
              >
                <div
                  className="w-7 h-7 rounded-lg flex items-center justify-center text-xs font-bold"
                  style={{
                    background: user?.method === 'oidc'
                      ? 'linear-gradient(135deg, rgba(95,168,211,0.25) 0%, rgba(95,168,211,0.1) 100%)'
                      : 'linear-gradient(135deg, rgba(212,118,78,0.25) 0%, rgba(212,118,78,0.1) 100%)',
                    border: user?.method === 'oidc'
                      ? '1px solid rgba(95,168,211,0.2)'
                      : '1px solid rgba(212,118,78,0.2)',
                    color: user?.method === 'oidc' ? '#7ecbf5' : '#e8a87c',
                  }}
                >
                  {displayInitial}
                </div>
                <span className="hidden lg:block text-xs text-[#8ba4c0] max-w-[100px] truncate">
                  {displayName}
                </span>
                <ChevronDown className="h-3 w-3 text-[#5a7a9e] hidden lg:block" />
              </button>

              {/* Dropdown */}
              {userMenuOpen && (
                <div
                  className="absolute right-0 top-full mt-2 w-56 rounded-xl py-1 z-50 animate-scale-in"
                  style={{
                    background: 'linear-gradient(180deg, #151d28 0%, #111820 100%)',
                    border: '1px solid rgba(192,204,224,0.08)',
                    boxShadow: '0 15px 40px rgba(0,0,0,0.5), inset 0 1px 0 rgba(192,204,224,0.06)',
                  }}
                >
                  {/* User info */}
                  <div className="px-4 py-3 border-b" style={{ borderColor: 'rgba(192,204,224,0.06)' }}>
                    <p className="text-sm font-medium text-[#d0dae6] truncate">{displayName}</p>
                    {user?.email && (
                      <p className="text-xs text-[#5a7a9e] truncate mt-0.5">{user.email}</p>
                    )}
                    <span
                      className="inline-flex items-center gap-1 text-[10px] font-medium mt-2 px-2 py-0.5 rounded-full"
                      style={{
                        background: user?.method === 'oidc'
                          ? 'rgba(95,168,211,0.1)'
                          : 'rgba(212,118,78,0.1)',
                        color: user?.method === 'oidc' ? '#7ecbf5' : '#e8a87c',
                        border: user?.method === 'oidc'
                          ? '1px solid rgba(95,168,211,0.15)'
                          : '1px solid rgba(212,118,78,0.15)',
                      }}
                    >
                      {user?.method === 'oidc' ? 'SSO' : 'API Key'}
                    </span>
                  </div>

                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm text-[#8ba4c0] hover:text-red-400 hover:bg-[#1a2332]/60 transition-colors"
                  >
                    <LogOut className="h-4 w-4" />
                    Sign out
                  </button>
                </div>
              )}
            </div>

            {/* Mobile hamburger */}
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="h-8 w-8 rounded-lg hover:bg-[#1a2332] flex md:hidden items-center justify-center transition-colors text-[#e8a87c]"
            >
              {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
            </button>
          </div>
        </div>
      </header>

      {/* Mobile menu */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-[#05070a]/75 backdrop-blur-sm" onClick={() => setMobileMenuOpen(false)} />
          <div className="absolute top-14 left-0 right-0 shadow-2xl p-4 space-y-1 z-50"
            style={{
              background: 'linear-gradient(180deg, #111820 0%, #0d1219 100%)',
              borderBottom: '1px solid rgba(192,204,224,0.06)',
            }}
          >
            {navigation.map((item) => {
              const Icon = item.icon
              const active = isActive(item.href)
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  onClick={() => setMobileMenuOpen(false)}
                  className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-all ${
                    active
                      ? 'text-[#e8a87c]'
                      : 'text-[#8ba4c0] hover:text-[#c0cce0] hover:bg-[#1a2332]'
                  }`}
                  style={active ? { background: 'rgba(212,118,78,0.1)' } : undefined}
                >
                  <Icon className="h-4 w-4" />
                  {item.name}
                </Link>
              )
            })}
            <Link
              to="/jobs/new"
              onClick={() => setMobileMenuOpen(false)}
              className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm btn-copper"
            >
              <Plus className="h-4 w-4" />
              Submit Job
            </Link>
          </div>
        </div>
      )}

      {/* Main content */}
      <main className="flex-1 overflow-auto px-3 py-4 md:px-6 md:py-6 page-bg">
        <div className="animate-fade-in">
          {children}
        </div>
      </main>
    </div>
  )
}
