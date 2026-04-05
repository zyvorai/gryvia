import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'react-hot-toast'

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
          </Routes>
        </Layout>
      </Router>
      <Toaster position="top-right" />
    </QueryClientProvider>
  )
}

export default App
