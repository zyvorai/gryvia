import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Activity, Cpu, Zap, TrendingUp, Server, CheckCircle, XCircle,
  Clock, Loader2, BarChart3, Gauge, AlertCircle, Briefcase,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { ClusterStats } from '@/lib/api'
import type { FabricAIJob } from '@/types'
import StatCard from '@/components/StatCard'
import GPUChart from '@/components/GPUChart'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function Dashboard() {
  const { data: clusterStats, isLoading: statsLoading, isError: statsError } = useQuery({
    queryKey: ['clusterStats'],
    queryFn: api.getClusterStats,
    refetchInterval: 15000,
  })

  const { data: jobs, isLoading: jobsLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 10000,
  })

  const { data: nodes } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 30000,
  })

  const { data: gpuMetrics } = useQuery({
    queryKey: ['gpuMetrics'],
    queryFn: api.getGPUMetrics,
    refetchInterval: 15000,
  })

  if (statsLoading) {
    return <LoadingSpinner />
  }

  if (statsError) {
    return (
      <div className="p-4 bg-red-500/10 border border-red-500/30 text-red-400 rounded-xl text-sm">
        Failed to load cluster stats. Please check your API connection.
      </div>
    )
  }

  const runningJobs = jobs?.filter(j => j.status?.phase === 'Running') || []
  const pendingJobs = jobs?.filter(j => j.status?.phase === 'Pending' || j.status?.phase === 'Queued') || []
  const completedJobs = jobs?.filter(j => j.status?.phase === 'Completed' || j.status?.phase === 'Succeeded') || []
  const failedJobs = jobs?.filter(j => j.status?.phase === 'Failed') || []
  const totalJobs = jobs?.length || 0
  const successRate = totalJobs > 0 ? Math.round((completedJobs.length / totalJobs) * 100) : 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-blue">Dashboard</h2>
          <p className="text-sm text-slate-400 mt-1">GPU Cluster Overview</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 bg-slate-800/50 rounded-lg border border-slate-700/50">
            <Activity className="h-3.5 w-3.5 text-green-400 animate-pulse-dot" />
            <span className="text-xs text-slate-400">Live</span>
          </div>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={Cpu}
          title="Total GPUs"
          value={clusterStats?.totalGPUs || 0}
          subtitle={`${clusterStats?.availableGPUs || 0} available`}
          color="blue"
        />
        <StatCard
          icon={Zap}
          title="Running Jobs"
          value={runningJobs.length}
          subtitle={pendingJobs.length > 0 ? `${pendingJobs.length} pending` : undefined}
          color="green"
        />
        <StatCard
          icon={CheckCircle}
          title="Completed"
          value={completedJobs.length}
          subtitle={successRate > 0 ? `${successRate}% success` : undefined}
          color="purple"
        />
        <StatCard
          icon={TrendingUp}
          title="GPU Utilization"
          value={`${Math.round(clusterStats?.utilizationPercent || 0)}%`}
          color="cyan"
        />
      </div>

      {/* Resource Cards */}
      {nodes && nodes.length > 0 && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <ResourceCard
            icon={Server}
            label="GPU Nodes"
            value={`${nodes.length} nodes`}
            subValue={`${clusterStats?.totalGPUs || 0} GPUs total`}
            percent={clusterStats?.totalGPUs ? Math.round(((clusterStats.allocatedGPUs || 0) / clusterStats.totalGPUs) * 100) : 0}
            color="blue"
          />
          <ResourceCard
            icon={Cpu}
            label="Allocated"
            value={`${clusterStats?.allocatedGPUs || 0} GPUs`}
            subValue={`${clusterStats?.availableGPUs || 0} free`}
            percent={clusterStats?.totalGPUs ? Math.round(((clusterStats.allocatedGPUs || 0) / clusterStats.totalGPUs) * 100) : 0}
            color="green"
          />
          <ResourceCard
            icon={Briefcase}
            label="Active Jobs"
            value={`${runningJobs.length} running`}
            subValue={`${totalJobs} total`}
            percent={totalJobs > 0 ? Math.round((runningJobs.length / totalJobs) * 100) : 0}
            color="yellow"
          />
          <ResourceCard
            icon={Gauge}
            label="Utilization"
            value={`${Math.round(clusterStats?.utilizationPercent || 0)}%`}
            subValue="avg across GPUs"
            percent={Math.round(clusterStats?.utilizationPercent || 0)}
            color="cyan"
          />
        </div>
      )}

      {/* Main content grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: GPU chart + recent jobs */}
        <div className="lg:col-span-2 space-y-6">
          {/* GPU Metrics Chart */}
          <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
            <div className="flex items-center gap-2 mb-4">
              <BarChart3 className="h-4 w-4 text-blue-400" />
              <h3 className="text-sm font-semibold text-white">GPU Utilization</h3>
            </div>
            <GPUChart data={gpuMetrics?.metrics?.map(m => ({
              time: new Date(m.timestamp).toLocaleTimeString(),
              [`${m.node}-GPU${m.gpuIndex}`]: m.utilization,
            }))} />
          </div>

          {/* Job Pipeline */}
          <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Briefcase className="h-4 w-4 text-blue-400" />
                <h3 className="text-sm font-semibold text-white">Job Pipeline</h3>
              </div>
              <span className="text-xs text-slate-500">{totalJobs} total</span>
            </div>

            <div className="grid grid-cols-3 sm:grid-cols-5 gap-2 sm:gap-3 mb-5">
              <PipelineStage label="Pending" count={pendingJobs.length} color="yellow" icon={Clock} />
              <PipelineStage label="Running" count={runningJobs.length} color="blue" icon={Loader2} spinning />
              <PipelineStage label="Completed" count={completedJobs.length} color="green" icon={CheckCircle} />
              <PipelineStage label="Failed" count={failedJobs.length} color="red" icon={XCircle} />
              <PipelineStage label="Nodes" count={clusterStats?.totalNodes || 0} color="cyan" icon={Server} />
            </div>

            {/* Recent Jobs */}
            <div className="text-xs text-slate-500 mb-2">Recent Jobs</div>
            <div className="space-y-1">
              {jobsLoading ? (
                <LoadingSpinner />
              ) : (
                <>
                  {(jobs || []).slice(0, 5).map((job) => (
                    <JobRow key={job.metadata?.name} job={job} />
                  ))}
                  {(!jobs || jobs.length === 0) && (
                    <div className="text-center py-6 text-sm text-slate-600">No jobs yet</div>
                  )}
                </>
              )}
            </div>

            <div className="mt-4 pt-4 border-t border-slate-700/30">
              <Link to="/jobs" className="text-xs text-blue-400 hover:text-blue-300 font-medium">
                View all jobs →
              </Link>
            </div>
          </div>
        </div>

        {/* Right sidebar */}
        <div className="space-y-6">
          {/* Cluster Health */}
          <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
            <div className="flex items-center gap-2 mb-4">
              <Gauge className="h-4 w-4 text-yellow-400" />
              <h3 className="text-sm font-semibold text-white">Cluster Health</h3>
            </div>
            <div className="space-y-2">
              <HealthCheck label="GPU Nodes" ok={(clusterStats?.totalNodes || 0) > 0} detail={`${clusterStats?.totalNodes || 0} nodes`} />
              <HealthCheck label="GPU Utilization" ok={(clusterStats?.utilizationPercent || 0) < 95} warn={(clusterStats?.utilizationPercent || 0) > 80} detail={`${Math.round(clusterStats?.utilizationPercent || 0)}%`} />
              <HealthCheck label="Failed Jobs" ok={(failedJobs.length) === 0} detail={`${failedJobs.length}`} />
              <HealthCheck label="Available GPUs" ok={(clusterStats?.availableGPUs || 0) > 0} detail={`${clusterStats?.availableGPUs || 0}`} />
              <HealthCheck label="Job Queue" ok={pendingJobs.length < 10} warn={pendingJobs.length > 5} detail={`${pendingJobs.length} pending`} />
            </div>
          </div>

          {/* GPU Distribution */}
          {nodes && nodes.length > 0 && (
            <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
              <div className="flex items-center gap-2 mb-4">
                <Server className="h-4 w-4 text-cyan-400" />
                <h3 className="text-sm font-semibold text-white">GPU Nodes</h3>
              </div>
              <div className="space-y-2">
                {nodes.slice(0, 6).map((node) => (
                  <div key={node.metadata?.name} className="flex items-center gap-3 p-2.5 bg-slate-900/50 rounded-lg border border-slate-700/30">
                    <div className={`w-2 h-2 rounded-full ${
                      node.status?.phase === 'Ready' ? 'bg-green-400' :
                      node.status?.phase === 'Degraded' ? 'bg-yellow-400' : 'bg-red-400'
                    }`} />
                    <div className="flex-1 min-w-0">
                      <div className="text-xs font-medium text-white truncate">{node.spec?.nodeName || node.metadata?.name}</div>
                      <div className="text-[10px] text-slate-500">{node.spec?.gpuType} x{node.spec?.gpuCount}</div>
                    </div>
                  </div>
                ))}
              </div>
              {nodes.length > 6 && (
                <div className="mt-3 text-center">
                  <Link to="/nodes" className="text-xs text-blue-400 hover:text-blue-300">
                    View all {nodes.length} nodes →
                  </Link>
                </div>
              )}
            </div>
          )}

          {/* Quick Actions */}
          <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
            <div className="flex items-center gap-2 mb-4">
              <Activity className="h-4 w-4 text-green-400" />
              <h3 className="text-sm font-semibold text-white">Quick Actions</h3>
            </div>
            <div className="space-y-2">
              <Link to="/jobs/new" className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-blue-600/10 border border-blue-500/20 text-blue-400 hover:bg-blue-600/20 transition-colors text-sm">
                <Zap className="h-4 w-4" />
                Submit New Job
              </Link>
              <Link to="/quotas" className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-slate-900/50 border border-slate-700/30 text-slate-300 hover:bg-slate-800 transition-colors text-sm">
                <Gauge className="h-4 w-4 text-purple-400" />
                View Quotas
              </Link>
              <Link to="/costs" className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-slate-900/50 border border-slate-700/30 text-slate-300 hover:bg-slate-800 transition-colors text-sm">
                <TrendingUp className="h-4 w-4 text-yellow-400" />
                Cost Analysis
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// --- Sub-components ---

function ResourceCard({ icon: Icon, label, value, subValue, percent, color }: {
  icon: React.ElementType; label: string; value: string; subValue: string; percent: number; color: string;
}) {
  const barColors: Record<string, string> = { blue: 'bg-blue-500', green: 'bg-green-500', yellow: 'bg-yellow-500', cyan: 'bg-cyan-500' }
  const textColors: Record<string, string> = { blue: 'text-blue-400', green: 'text-green-400', yellow: 'text-yellow-400', cyan: 'text-cyan-400' }
  const pctColor = percent > 90 ? 'text-red-400' : percent > 75 ? 'text-yellow-400' : textColors[color]

  return (
    <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-4 card-glow transition-all hover:scale-[1.01]">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Icon className={`h-4 w-4 ${textColors[color]}`} />
          <span className="text-xs font-medium text-slate-300">{label}</span>
        </div>
        <span className={`text-sm font-bold ${pctColor}`}>{percent}%</span>
      </div>
      <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden mb-2">
        <div className={`h-full ${percent > 90 ? 'bg-red-500' : percent > 75 ? 'bg-yellow-500' : barColors[color]} rounded-full transition-all`} style={{ width: `${percent}%` }} />
      </div>
      <div className="text-xs text-slate-300">{value}</div>
      <div className="text-[10px] text-slate-500">{subValue}</div>
    </div>
  )
}

function PipelineStage({ label, count, color, icon: Icon, spinning }: {
  label: string; count: number; color: string; icon: React.ElementType; spinning?: boolean;
}) {
  const c: Record<string, { bg: string; text: string; ring: string }> = {
    yellow: { bg: 'bg-yellow-500/10', text: 'text-yellow-400', ring: 'ring-yellow-500/30' },
    blue: { bg: 'bg-blue-500/10', text: 'text-blue-400', ring: 'ring-blue-500/30' },
    cyan: { bg: 'bg-cyan-500/10', text: 'text-cyan-400', ring: 'ring-cyan-500/30' },
    green: { bg: 'bg-green-500/10', text: 'text-green-400', ring: 'ring-green-500/30' },
    red: { bg: 'bg-red-500/10', text: 'text-red-400', ring: 'ring-red-500/30' },
  }
  const s = c[color] || c.blue
  return (
    <div className={`text-center p-3 rounded-lg ${s.bg} ring-1 ${s.ring}`}>
      <Icon className={`h-4 w-4 mx-auto mb-1 ${s.text} ${spinning && count > 0 ? 'animate-spin' : ''}`} />
      <div className={`text-lg font-bold ${s.text}`}>{count}</div>
      <div className="text-[10px] text-slate-500">{label}</div>
    </div>
  )
}

function JobRow({ job }: { job: FabricAIJob }) {
  const phase = job.status?.phase || 'Unknown'
  const cfg: Record<string, { icon: React.ElementType; color: string; bg: string }> = {
    Pending: { icon: Clock, color: 'text-yellow-400', bg: 'bg-yellow-500/10' },
    Queued: { icon: Clock, color: 'text-yellow-400', bg: 'bg-yellow-500/10' },
    Running: { icon: Loader2, color: 'text-blue-400', bg: 'bg-blue-500/10' },
    Completed: { icon: CheckCircle, color: 'text-green-400', bg: 'bg-green-500/10' },
    Succeeded: { icon: CheckCircle, color: 'text-green-400', bg: 'bg-green-500/10' },
    Failed: { icon: XCircle, color: 'text-red-400', bg: 'bg-red-500/10' },
  }
  const c = cfg[phase] || { icon: AlertCircle, color: 'text-slate-400', bg: 'bg-slate-500/10' }
  const Icon = c.icon

  return (
    <Link to={`/jobs/${job.metadata?.name}`} className="flex items-center gap-3 p-2.5 rounded-lg table-row-hover">
      <span className={`inline-flex items-center justify-center w-7 h-7 rounded-lg ${c.bg}`}>
        <Icon className={`h-3.5 w-3.5 ${c.color} ${phase === 'Running' ? 'animate-spin' : ''}`} />
      </span>
      <div className="flex-1 min-w-0">
        <div className="text-xs font-medium text-white truncate">{job.metadata?.name}</div>
        <div className="text-[10px] text-slate-500">
          {job.spec?.framework || job.spec?.type || 'job'} · {job.spec?.resources?.gpuType || 'GPU'} x{job.spec?.resources?.gpuCount || job.spec?.gpus || '?'}
        </div>
      </div>
      <span className={`text-[10px] font-medium px-2 py-0.5 rounded-full ${c.bg} ${c.color}`}>{phase}</span>
    </Link>
  )
}

function HealthCheck({ label, ok, warn, detail }: { label: string; ok: boolean; warn?: boolean; detail?: string }) {
  return (
    <div className="flex items-center justify-between py-1.5">
      <span className="text-xs text-slate-400">{label}</span>
      <div className="flex items-center gap-2">
        {detail && <span className="text-[10px] text-slate-500">{detail}</span>}
        <div className={`w-2 h-2 rounded-full ${ok ? (warn ? 'bg-yellow-400' : 'bg-green-400') : 'bg-red-400'}`} />
      </div>
    </div>
  )
}
