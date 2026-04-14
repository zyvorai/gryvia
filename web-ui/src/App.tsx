import { BrowserRouter as Router, Routes, Route, Navigate, Link, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'react-hot-toast'
import { type ReactNode } from 'react'

import ErrorBoundary from './components/ErrorBoundary'
import AuthProvider from './components/AuthProvider'
import { useAuth } from './lib/auth'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Jobs from './pages/Jobs'
import JobDetails from './pages/JobDetails'
import Quotas from './pages/Quotas'
import Nodes from './pages/Nodes'
import Costs from './pages/Costs'
import SubmitJob from './pages/SubmitJob'
import NetworkOverview from './pages/NetworkOverview'
import NetworkFlows from './pages/NetworkFlows'
import NetworkPolicies from './pages/NetworkPolicies'
import SecurityOverview from './pages/SecurityOverview'
import GpuCommunication from './pages/GpuCommunication'
import NetworkCost from './pages/NetworkCost'
import Login from './pages/Login'
import AuthCallback from './pages/AuthCallback'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30000, // 30 seconds
    },
  },
})

/**
 * Wrapper that redirects unauthenticated users to /login.
 * Shows nothing while auth state is loading to avoid flicker.
 */
function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()
  const location = useLocation()

  if (isLoading) {
    // Brief loading state while validating stored token
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: '#0a0e14' }}>
        <div className="flex flex-col items-center gap-3 animate-fade-in">
          <div
            className="w-10 h-10 rounded-lg flex items-center justify-center"
            style={{
              background: 'linear-gradient(135deg, #d4764e 0%, #e8a87c 50%, #d4764e 100%)',
              boxShadow: '0 2px 8px rgba(212,118,78,0.3)',
            }}
          >
            <span className="text-[#0a0e14] font-bold text-sm">TR</span>
          </div>
          <div className="h-1 w-24 rounded-full overflow-hidden" style={{ background: 'rgba(192,204,224,0.06)' }}>
            <div
              className="h-full rounded-full"
              style={{
                background: 'linear-gradient(90deg, #d4764e, #e8a87c)',
                animation: 'shimmer 1.5s infinite',
                width: '40%',
              }}
            />
          </div>
        </div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}

function App() {
  return (
    <ErrorBoundary>
    <QueryClientProvider client={queryClient}>
      <Router>
        <AuthProvider>
          <Routes>
            {/* Public routes */}
            <Route path="/login" element={<Login />} />
            <Route path="/auth/callback" element={<AuthCallback />} />

            {/* Protected routes */}
            <Route path="/" element={
              <RequireAuth>
                <Navigate to="/dashboard" replace />
              </RequireAuth>
            } />
            <Route path="/dashboard" element={
              <RequireAuth>
                <Layout><Dashboard /></Layout>
              </RequireAuth>
            } />
            <Route path="/jobs" element={
              <RequireAuth>
                <Layout><Jobs /></Layout>
              </RequireAuth>
            } />
            <Route path="/jobs/new" element={
              <RequireAuth>
                <Layout><SubmitJob /></Layout>
              </RequireAuth>
            } />
            <Route path="/jobs/:name" element={
              <RequireAuth>
                <Layout><JobDetails /></Layout>
              </RequireAuth>
            } />
            <Route path="/quotas" element={
              <RequireAuth>
                <Layout><Quotas /></Layout>
              </RequireAuth>
            } />
            <Route path="/nodes" element={
              <RequireAuth>
                <Layout><Nodes /></Layout>
              </RequireAuth>
            } />
            <Route path="/network" element={
              <RequireAuth>
                <Layout><NetworkOverview /></Layout>
              </RequireAuth>
            } />
            <Route path="/network/flows" element={
              <RequireAuth>
                <Layout><NetworkFlows /></Layout>
              </RequireAuth>
            } />
            <Route path="/network/policies" element={
              <RequireAuth>
                <Layout><NetworkPolicies /></Layout>
              </RequireAuth>
            } />
            <Route path="/network/costs" element={
              <RequireAuth>
                <Layout><NetworkCost /></Layout>
              </RequireAuth>
            } />
            <Route path="/security" element={
              <RequireAuth>
                <Layout><SecurityOverview /></Layout>
              </RequireAuth>
            } />
            <Route path="/gpu" element={
              <RequireAuth>
                <Layout><GpuCommunication /></Layout>
              </RequireAuth>
            } />
            <Route path="/gpu/communication" element={
              <RequireAuth>
                <Layout><GpuCommunication /></Layout>
              </RequireAuth>
            } />
            <Route path="/costs" element={
              <RequireAuth>
                <Layout><Costs /></Layout>
              </RequireAuth>
            } />
            <Route path="*" element={
              <RequireAuth>
                <Layout>
                  <div className="p-8 text-center">
                    <h1 className="text-2xl font-bold text-white">404 - Page Not Found</h1>
                    <Link to="/dashboard" className="text-blue-400 hover:text-blue-300 mt-4 inline-block text-sm">Go to Dashboard</Link>
                  </div>
                </Layout>
              </RequireAuth>
            } />
          </Routes>
        </AuthProvider>
      </Router>
      <Toaster position="top-right" />
    </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App
