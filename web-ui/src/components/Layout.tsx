import { ReactNode, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
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
} from 'lucide-react'

interface LayoutProps {
  children: ReactNode
}

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { name: 'Jobs', href: '/jobs', icon: Briefcase },
  { name: 'Quotas', href: '/quotas', icon: Users },
  { name: 'Nodes', href: '/nodes', icon: Server },
  { name: 'Costs', href: '/costs', icon: DollarSign },
]

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  const isActive = (href: string) => {
    if (href === '/dashboard') return location.pathname === '/dashboard' || location.pathname === '/'
    return location.pathname.startsWith(href)
  }

  return (
    <div className="h-screen flex flex-col bg-slate-950">
      {/* Top Navbar */}
      <header className="sticky top-0 z-50 navbar-gradient border-b border-slate-700/50 flex-shrink-0">
        <div className="flex items-center h-14 px-4">
          {/* Logo */}
          <Link to="/dashboard" className="flex items-center gap-2 mr-8 flex-shrink-0">
            <div className="w-8 h-8 bg-gradient-to-br from-blue-400 to-cyan-600 rounded-lg flex items-center justify-center">
              <span className="text-white font-bold text-sm">KF</span>
            </div>
            <h1 className="text-xl font-bold bg-gradient-to-r from-blue-400 to-blue-600 bg-clip-text text-transparent">
              KubeFabric
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
                  className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                    active
                      ? 'bg-blue-600/20 text-blue-400'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
                  }`}
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
              className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors"
            >
              <Plus className="h-3.5 w-3.5" />
              Submit Job
            </Link>

            <div className="flex items-center gap-2 px-3 py-1.5 bg-slate-800/50 rounded-lg border border-slate-700/50">
              <Activity className="h-3.5 w-3.5 text-green-400 animate-pulse-dot" />
              <span className="text-xs text-slate-400">Live</span>
            </div>

            {/* Mobile hamburger */}
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="h-8 w-8 rounded-lg hover:bg-slate-800 flex md:hidden items-center justify-center transition-colors text-blue-400"
            >
              {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
            </button>
          </div>
        </div>
      </header>

      {/* Mobile menu */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setMobileMenuOpen(false)} />
          <div className="absolute top-14 left-0 right-0 bg-slate-900 border-b border-slate-700 shadow-2xl p-4 space-y-1 z-50">
            {navigation.map((item) => {
              const Icon = item.icon
              const active = isActive(item.href)
              return (
                <Link
                  key={item.name}
                  to={item.href}
                  onClick={() => setMobileMenuOpen(false)}
                  className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                    active
                      ? 'bg-blue-600/20 text-blue-400'
                      : 'text-slate-300 hover:bg-slate-800 hover:text-slate-100'
                  }`}
                >
                  <Icon className="h-4 w-4" />
                  {item.name}
                </Link>
              )
            })}
            <Link
              to="/jobs/new"
              onClick={() => setMobileMenuOpen(false)}
              className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm bg-blue-600/20 text-blue-400"
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
