import { useQuery } from '@tanstack/react-query'
import { Activity, Cpu, Zap, TrendingUp } from 'lucide-react'
import { api } from '@/lib/api'
import StatCard from '@/components/StatCard'
import GPUChart from '@/components/GPUChart'
import JobsTable from '@/components/JobsTable'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Dashboard() {
  const { data: clusterStats, isLoading: statsLoading } = useQuery({
    queryKey: ['clusterStats'],
    queryFn: api.getClusterStats,
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  const { data: jobs, isLoading: jobsLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 15000,
  })

  const { data: gpuMetrics } = useQuery({
    queryKey: ['gpuMetrics'],
    queryFn: api.getGPUMetrics,
    refetchInterval: 15000,
  })

  if (statsLoading) {
    return <LoadingSpinner />
  }

  const runningJobs = jobs?.filter(j => j.status?.phase === 'Running').length || 0
  const pendingJobs = jobs?.filter(j => j.status?.phase === 'Pending').length || 0

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold text-gray-900">Cluster Overview</h2>
        <p className="mt-1 text-sm text-gray-500">
          Real-time GPU cluster status and metrics
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total GPUs"
          value={clusterStats?.totalGPUs || 0}
          icon={Cpu}
          color="blue"
        />
        <StatCard
          title="Allocated GPUs"
          value={clusterStats?.allocatedGPUs || 0}
          subtitle={`${clusterStats?.utilizationPercent || 0}% utilized`}
          icon={Activity}
          color="green"
        />
        <StatCard
          title="Running Jobs"
          value={runningJobs}
          subtitle={`${pendingJobs} pending`}
          icon={Zap}
          color="purple"
        />
        <StatCard
          title="Avg Utilization"
          value={`${Math.round(clusterStats?.avgGPUUtilization || 0)}%`}
          icon={TrendingUp}
          color="orange"
        />
      </div>

      {/* GPU Metrics Chart */}
      <div className="bg-white shadow rounded-lg p-6">
        <h3 className="text-lg font-medium text-gray-900 mb-4">
          GPU Utilization
        </h3>
        <GPUChart data={gpuMetrics} />
      </div>

      {/* Recent Jobs */}
      <div className="bg-white shadow rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-medium text-gray-900">Recent Jobs</h3>
          <a
            href="/jobs"
            className="text-sm text-blue-600 hover:text-blue-700 font-medium"
          >
            View all →
          </a>
        </div>
        {jobsLoading ? (
          <LoadingSpinner />
        ) : (
          <JobsTable jobs={jobs?.slice(0, 5) || []} compact />
        )}
      </div>
    </div>
  )
}
