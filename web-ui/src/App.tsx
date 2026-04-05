import { BrowserRouter as Router, Routes, Route, Navigate, Link } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'react-hot-toast'

import ErrorBoundary from './components/ErrorBoundary'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Jobs from './pages/Jobs'
import JobDetails from './pages/JobDetails'
import Quotas from './pages/Quotas'
import Nodes from './pages/Nodes'
import Costs from './pages/Costs'
import SubmitJob from './pages/SubmitJob'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30000, // 30 seconds
    },
  },
})

function App() {
  return (
    <ErrorBoundary>
    <QueryClientProvider client={queryClient}>
      <Router>
        <Layout>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/jobs" element={<Jobs />} />
            <Route path="/jobs/new" element={<SubmitJob />} />
            <Route path="/jobs/:name" element={<JobDetails />} />
            <Route path="/quotas" element={<Quotas />} />
            <Route path="/nodes" element={<Nodes />} />
            <Route path="/costs" element={<Costs />} />
            <Route path="*" element={
              <div className="p-8 text-center">
                <h1 className="text-2xl font-bold text-white">404 - Page Not Found</h1>
                <Link to="/dashboard" className="text-blue-400 hover:text-blue-300 mt-4 inline-block text-sm">Go to Dashboard</Link>
              </div>
            } />
          </Routes>
        </Layout>
      </Router>
      <Toaster position="top-right" />
    </QueryClientProvider>
    </ErrorBoundary>
  )
}

export default App
