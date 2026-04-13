import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Activity, Cpu, Zap, TrendingUp, Server, CheckCircle, XCircle,
  Clock, Loader2, BarChart3, Gauge, AlertCircle, Briefcase,
} from 'lucide-react'
import { api } from '@/lib/api'
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
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
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
          <h2 className="text-2xl font-bold text-gradient-copper">Dashboard</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">GPU Cluster Overview</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg"
            style={{
              background: 'rgba(10,14,20,0.5)',
              border: '1px solid rgba(34,197,94,0.12)',
            }}
          >
            <Activity className="h-3.5 w-3.5 text-emerald-400 animate-pulse-dot" />
            <span className="text-xs text-[#5a7a9e]">Live</span>
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
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <BarChart3 className="h-4 w-4 text-[#7ecbf5]" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">GPU Utilization</h3>
            </div>
            <GPUChart data={gpuMetrics?.metrics?.map(m => ({
              time: new Date(m.timestamp).toLocaleTimeString(),
              [`${m.node}-GPU${m.gpuIndex}`]: m.utilization,
            }))} />
          </div>

          {/* Job Pipeline */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Briefcase className="h-4 w-4 text-[#e8a87c]" />
                <h3 className="text-sm font-semibold text-[#e8ecf1]">Job Pipeline</h3>
              </div>
              <span className="text-xs text-[#5a7a9e]">{totalJobs} total</span>
            </div>

            <div className="grid grid-cols-3 sm:grid-cols-5 gap-2 sm:gap-3 mb-5">
              <PipelineStage label="Pending" count={pendingJobs.length} color="yellow" icon={Clock} />
              <PipelineStage label="Running" count={runningJobs.length} color="blue" icon={Loader2} spinning />
              <PipelineStage label="Completed" count={completedJobs.length} color="green" icon={CheckCircle} />
              <PipelineStage label="Failed" count={failedJobs.length} color="red" icon={XCircle} />
              <PipelineStage label="Nodes" count={clusterStats?.totalNodes || 0} color="cyan" icon={Server} />
            </div>

            {/* Recent Jobs */}
            <div className="text-xs text-[#5a7a9e] mb-2">Recent Jobs</div>
            <div className="space-y-1">
              {jobsLoading ? (
                <LoadingSpinner />
              ) : (
                <>
                  {(jobs || []).slice(0, 5).map((job) => (
                    <JobRow key={job.metadata?.name} job={job} />
                  ))}
                  {(!jobs || jobs.length === 0) && (
                    <div className="text-center py-6 text-sm text-[#344e6a]">No jobs yet</div>
                  )}
                </>
              )}
            </div>

            <div className="mt-4 pt-4" style={{ borderTop: '1px solid rgba(192,204,224,0.06)' }}>
              <Link to="/jobs" className="text-xs text-[#e8a87c] hover:text-[#f0c4a0] font-medium transition-colors">
                View all jobs &rarr;
              </Link>
            </div>
          </div>
        </div>

        {/* Right sidebar */}
        <div className="space-y-6">
          {/* Cluster Health */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Gauge className="h-4 w-4 text-[#fbbf24]" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Cluster Health</h3>
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
            <div className="rounded-xl p-5" style={{
              background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
              border: '1px solid rgba(192,204,224,0.06)',
              boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
            }}>
              <div className="flex items-center gap-2 mb-4">
                <Server className="h-4 w-4 text-cyan-400" />
                <h3 className="text-sm font-semibold text-[#e8ecf1]">GPU Nodes</h3>
              </div>
              <div className="space-y-2">
                {nodes.slice(0, 6).map((node) => (
                  <div key={node.metadata?.name} className="flex items-center gap-3 p-2.5 rounded-lg"
                    style={{
                      background: 'rgba(10,14,20,0.5)',
                      border: '1px solid rgba(192,204,224,0.04)',
                    }}
                  >
                    <div className={`w-2 h-2 rounded-full ${
                      node.status?.phase === 'Ready' ? 'bg-emerald-400' :
                      node.status?.phase === 'Degraded' ? 'bg-yellow-400' : 'bg-red-400'
                    }`} style={{
                      boxShadow: node.status?.phase === 'Ready'
                        ? '0 0 6px rgba(52,211,153,0.4)'
                        : node.status?.phase === 'Degraded'
                        ? '0 0 6px rgba(250,204,21,0.4)'
                        : '0 0 6px rgba(248,113,113,0.4)',
                    }} />
                    <div className="flex-1 min-w-0">
                      <div className="text-xs font-medium text-[#c0cce0] truncate">{node.spec?.nodeName || node.metadata?.name}</div>
                      <div className="text-[10px] text-[#5a7a9e]">{node.spec?.gpuType} x{node.spec?.gpuCount}</div>
                    </div>
                  </div>
                ))}
              </div>
              {nodes.length > 6 && (
                <div className="mt-3 text-center">
                  <Link to="/nodes" className="text-xs text-[#e8a87c] hover:text-[#f0c4a0] transition-colors">
                    View all {nodes.length} nodes &rarr;
                  </Link>
                </div>
              )}
            </div>
          )}

          {/* Quick Actions */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Activity className="h-4 w-4 text-emerald-400" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Quick Actions</h3>
            </div>
            <div className="space-y-2">
              <Link to="/jobs/new" className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-all duration-200 text-[#e8a87c] hover:text-[#f0c4a0]"
                style={{
                  background: 'rgba(212,118,78,0.08)',
                  border: '1px solid rgba(212,118,78,0.15)',
                }}
              >
                <Zap className="h-4 w-4" />
                Submit New Job
              </Link>
              <Link to="/quotas" className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-[#8ba4c0] hover:text-[#c0cce0] transition-all duration-200"
                style={{
                  background: 'rgba(10,14,20,0.4)',
                  border: '1px solid rgba(192,204,224,0.04)',
                }}
              >
                <Gauge className="h-4 w-4 text-purple-400" />
                View Quotas
              </Link>
              <Link to="/costs" className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-[#8ba4c0] hover:text-[#c0cce0] transition-all duration-200"
                style={{
                  background: 'rgba(10,14,20,0.4)',
                  border: '1px solid rgba(192,204,224,0.04)',
                }}
              >
                <TrendingUp className="h-4 w-4 text-[#fbbf24]" />
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
  const barColors: Record<string, string> = {
    blue: 'linear-gradient(90deg, #344e6a, #5fa8d3)',
    green: 'linear-gradient(90deg, #166534, #4ade80)',
    yellow: 'linear-gradient(90deg, #7c3a1a, #d4764e)',
    cyan: 'linear-gradient(90deg, #164e63, #22d3ee)',
  }
  const textColors: Record<string, string> = { blue: 'text-[#7ecbf5]', green: 'text-emerald-400', yellow: 'text-[#e8a87c]', cyan: 'text-cyan-400' }
  const pctColor = percent > 90 ? 'text-red-400' : percent > 75 ? 'text-[#fbbf24]' : textColors[color]

  return (
    <div className="rounded-xl p-4 card-glow transition-all duration-300 hover:scale-[1.01]"
      style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Icon className={`h-4 w-4 ${textColors[color]}`} />
          <span className="text-xs font-medium text-[#8ba4c0]">{label}</span>
        </div>
        <span className={`text-sm font-bold ${pctColor}`}>{percent}%</span>
      </div>
      <div className="h-1.5 rounded-full overflow-hidden mb-2"
        style={{ background: 'rgba(10,14,20,0.6)', boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.4)' }}
      >
        <div className="h-full rounded-full transition-all"
          style={{
            width: `${percent}%`,
            background: percent > 90 ? 'linear-gradient(90deg, #7f1d1d, #ef4444)' : percent > 75 ? 'linear-gradient(90deg, #7c3a1a, #fbbf24)' : barColors[color],
            boxShadow: '0 0 6px rgba(192,204,224,0.1)',
          }}
        />
      </div>
      <div className="text-xs text-[#8ba4c0]">{value}</div>
      <div className="text-[10px] text-[#5a7a9e]">{subValue}</div>
    </div>
  )
}

function PipelineStage({ label, count, color, icon: Icon, spinning }: {
  label: string; count: number; color: string; icon: React.ElementType; spinning?: boolean;
}) {
  const c: Record<string, { bg: string; text: string; ring: string; glow: string }> = {
    yellow: { bg: 'rgba(251,191,36,0.06)', text: 'text-[#fbbf24]', ring: 'rgba(251,191,36,0.15)', glow: 'rgba(251,191,36,0.1)' },
    blue: { bg: 'rgba(95,168,211,0.06)', text: 'text-[#7ecbf5]', ring: 'rgba(95,168,211,0.15)', glow: 'rgba(95,168,211,0.1)' },
    cyan: { bg: 'rgba(6,182,212,0.06)', text: 'text-cyan-400', ring: 'rgba(6,182,212,0.15)', glow: 'rgba(6,182,212,0.1)' },
    green: { bg: 'rgba(34,197,94,0.06)', text: 'text-emerald-400', ring: 'rgba(34,197,94,0.15)', glow: 'rgba(34,197,94,0.1)' },
    red: { bg: 'rgba(239,68,68,0.06)', text: 'text-red-400', ring: 'rgba(239,68,68,0.15)', glow: 'rgba(239,68,68,0.1)' },
  }
  const s = c[color] || c.blue
  return (
    <div className="text-center p-3 rounded-lg" style={{
      background: s.bg,
      border: `1px solid ${s.ring}`,
      boxShadow: `inset 0 1px 0 rgba(192,204,224,0.02), 0 0 12px ${s.glow}`,
    }}>
      <Icon className={`h-4 w-4 mx-auto mb-1 ${s.text} ${spinning && count > 0 ? 'animate-spin' : ''}`} />
      <div className={`text-lg font-bold ${s.text}`}>{count}</div>
      <div className="text-[10px] text-[#5a7a9e]">{label}</div>
    </div>
  )
}

function JobRow({ job }: { job: FabricAIJob }) {
  const phase = job.status?.phase || 'Unknown'
  const cfg: Record<string, { icon: React.ElementType; color: string; bg: string }> = {
    Pending: { icon: Clock, color: 'text-[#fbbf24]', bg: 'rgba(251,191,36,0.08)' },
    Queued: { icon: Clock, color: 'text-[#fbbf24]', bg: 'rgba(251,191,36,0.08)' },
    Running: { icon: Loader2, color: 'text-[#7ecbf5]', bg: 'rgba(95,168,211,0.08)' },
    Completed: { icon: CheckCircle, color: 'text-emerald-400', bg: 'rgba(34,197,94,0.08)' },
    Succeeded: { icon: CheckCircle, color: 'text-emerald-400', bg: 'rgba(34,197,94,0.08)' },
    Failed: { icon: XCircle, color: 'text-red-400', bg: 'rgba(239,68,68,0.08)' },
  }
  const c = cfg[phase] || { icon: AlertCircle, color: 'text-[#5a7a9e]', bg: 'rgba(90,122,158,0.08)' }
  const Icon = c.icon

  return (
    <Link to={`/jobs/${job.metadata?.name}`} className="flex items-center gap-3 p-2.5 rounded-lg table-row-hover">
      <span className="inline-flex items-center justify-center w-7 h-7 rounded-lg" style={{ background: c.bg }}>
        <Icon className={`h-3.5 w-3.5 ${c.color} ${phase === 'Running' ? 'animate-spin' : ''}`} />
      </span>
      <div className="flex-1 min-w-0">
        <div className="text-xs font-medium text-[#c0cce0] truncate">{job.metadata?.name}</div>
        <div className="text-[10px] text-[#5a7a9e]">
          {job.spec?.framework || 'job'} · {job.spec?.resources?.gpuType || 'GPU'} x{job.spec?.resources?.gpuCount || '?'}
        </div>
      </div>
      <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{ background: c.bg, color: undefined }}>
        <span className={c.color}>{phase}</span>
      </span>
    </Link>
  )
}

function HealthCheck({ label, ok, warn, detail }: { label: string; ok: boolean; warn?: boolean; detail?: string }) {
  const dotColor = ok ? (warn ? '#fbbf24' : '#4ade80') : '#f87171'
  return (
    <div className="flex items-center justify-between py-1.5">
      <span className="text-xs text-[#8090a8]">{label}</span>
      <div className="flex items-center gap-2">
        {detail && <span className="text-[10px] text-[#5a7a9e]">{detail}</span>}
        <div className="w-2 h-2 rounded-full" style={{
          backgroundColor: dotColor,
          boxShadow: `0 0 6px ${dotColor}60`,
        }} />
      </div>
    </div>
  )
}
