import { BrowserRouter as Router, Routes, Route, Navigate, Link, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'react-hot-toast'
import { type ReactNode } from 'react'

import ErrorBoundary from './components/ErrorBoundary'
import AuthProvider from './components/AuthProvider'
import { useAuth } from './lib/auth'
import Layout from './components/Layout'
import PageHero from './components/PageHero'
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
import Workspaces from './pages/Workspaces'
import ModelRegistry from './pages/ModelRegistry'
import InferenceServices from './pages/InferenceServices'
import WorkflowsPage from './pages/Workflows'
import AutoTunerPage from './pages/AutoTuner'
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
    return (
      <div className="login-shell">
        <div className="spinner" role="status" aria-label="Loading" />
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
            <Route path="/workspaces" element={
              <RequireAuth>
                <Layout><Workspaces /></Layout>
              </RequireAuth>
            } />
            <Route path="/models" element={
              <RequireAuth>
                <Layout><ModelRegistry /></Layout>
              </RequireAuth>
            } />
            <Route path="/inference" element={
              <RequireAuth>
                <Layout><InferenceServices /></Layout>
              </RequireAuth>
            } />
            <Route path="/workflows" element={
              <RequireAuth>
                <Layout><WorkflowsPage /></Layout>
              </RequireAuth>
            } />
            <Route path="/tuner" element={
              <RequireAuth>
                <Layout><AutoTunerPage /></Layout>
              </RequireAuth>
            } />
            <Route path="*" element={
              <RequireAuth>
                <Layout>
                  <PageHero eyebrow="404" title="Page not found." lede="That page doesn't exist." />
                  <Link to="/dashboard" className="apple-text-link">Go to Dashboard</Link>
                </Layout>
              </RequireAuth>
            } />
          </Routes>
        </AuthProvider>
      </Router>
      <Toaster position="top-right" toastOptions={{ style: { background: 'var(--bg-elevated)', color: 'var(--text-primary)', border: '1px solid var(--hairline-1)' } }} />
    </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App
