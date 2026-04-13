import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import StatCard from '@/components/StatCard'
import { Server, Cpu, HardDrive, Thermometer, Activity, Network } from 'lucide-react'

export default function Nodes() {
  const { data: nodes, isLoading, isError } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 10000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <div className="p-4 bg-red-500/10 border border-red-500/30 text-red-400 rounded-xl text-sm">
        Failed to load GPU nodes. Please check your API connection.
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gradient-blue">GPU Nodes</h2>
        <p className="text-sm text-slate-400 mt-1">Physical GPU nodes and real-time metrics</p>
      </div>

      {/* Overview Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={Server} title="Total Nodes" value={nodes?.length || 0} color="blue" />
        <StatCard icon={Cpu} title="Total GPUs" value={nodes?.reduce((sum, n) => sum + (n.spec?.gpuCount || 0), 0) || 0} color="green" />
        <StatCard icon={Network} title="RDMA Enabled" value={nodes?.filter(n => n.spec?.rdmaEnabled).length || 0} color="cyan" />
        <StatCard icon={Activity} title="Ready Nodes" value={nodes?.filter(n => n.status?.phase === 'Ready').length || 0} subtitle={`${nodes?.filter(n => n.status?.phase !== 'Ready').length || 0} degraded`} color="purple" />
      </div>

      {/* Nodes List */}
      <div className="grid grid-cols-1 gap-4">
        {nodes?.map((node) => (
          <div key={node.metadata?.name} className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
            <div className="px-5 py-4 border-b border-slate-700/30 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Server className="h-4 w-4 text-blue-400" />
                <h3 className="text-sm font-semibold text-white">{node.spec?.nodeName || node.metadata?.name}</h3>
              </div>
              <div className="flex items-center gap-2">
                <span className={`text-[10px] font-medium px-2 py-0.5 rounded-full ${
                  node.status?.phase === 'Ready' ? 'bg-green-500/10 text-green-400' :
                  node.status?.phase === 'Degraded' ? 'bg-yellow-500/10 text-yellow-400' :
                  'bg-red-500/10 text-red-400'
                }`}>
                  {node.status?.phase || 'Unknown'}
                </span>
                {node.spec?.rdmaEnabled && (
                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400">RDMA</span>
                )}
              </div>
            </div>

            <div className="px-5 py-4">
              {/* Node Info */}
              <div className="grid grid-cols-3 gap-4 mb-4">
                <div className="p-2.5 bg-slate-900/30 rounded-lg">
                  <div className="text-[10px] text-slate-500">GPU Type</div>
                  <div className="text-xs font-medium text-slate-300">{node.spec?.gpuType}</div>
                </div>
                <div className="p-2.5 bg-slate-900/30 rounded-lg">
                  <div className="text-[10px] text-slate-500">GPU Count</div>
                  <div className="text-xs font-medium text-slate-300">{node.spec?.gpuCount}</div>
                </div>
                <div className="p-2.5 bg-slate-900/30 rounded-lg">
                  <div className="text-[10px] text-slate-500">Memory</div>
                  <div className="text-xs font-medium text-slate-300">{node.spec?.memory || 'N/A'}</div>
                </div>
              </div>

              {/* GPU Metrics */}
              {node.status?.gpus && node.status.gpus.length > 0 && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {node.status.gpus.map((gpu) => (
                    <div key={gpu.index} className="bg-slate-900/30 rounded-lg p-3 border border-slate-700/30">
                      <div className="flex items-center justify-between mb-3">
                        <div className="flex items-center gap-2">
                          <Cpu className="h-3.5 w-3.5 text-blue-400" />
                          <span className="text-xs font-medium text-white">GPU {gpu.index}</span>
                        </div>
                        <span className="text-[10px] text-slate-500 font-mono">{gpu.uuid?.substring(0, 12)}...</span>
                      </div>

                      <MetricBar icon={Thermometer} label="Temperature" value={`${gpu.temperature}°C`} percent={Math.min(gpu.temperature, 100)}
                        color={gpu.temperature > 80 ? 'red' : gpu.temperature > 70 ? 'yellow' : 'green'} />
                      <MetricBar icon={Activity} label="Utilization" value={`${gpu.utilization}%`} percent={Math.min(gpu.utilization, 100)} color="blue" />
                      {(() => {
                        const memUsedGB = Number(gpu.memoryUsed) / 1024 || 0
                        const memTotalGB = Number(gpu.memoryTotal) / 1024 || 0
                        const memPercent = memTotalGB > 0 ? Math.min(100, (memUsedGB / memTotalGB) * 100) : 0
                        return <MetricBar icon={HardDrive} label="Memory" value={`${memUsedGB.toFixed(1)} / ${memTotalGB.toFixed(1)} GB`}
                          percent={memPercent} color="purple" />
                      })()}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function MetricBar({ icon: Icon, label, value, percent, color }: {
  icon: React.ElementType; label: string; value: string; percent: number; color: string;
}) {
  const barColors: Record<string, string> = { blue: 'bg-blue-500', green: 'bg-green-500', yellow: 'bg-yellow-500', red: 'bg-red-500', purple: 'bg-purple-500' }
  return (
    <div className="mb-2.5 last:mb-0">
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-1">
          <Icon className="h-3 w-3 text-slate-500" />
          <span className="text-[10px] text-slate-500">{label}</span>
        </div>
        <span className="text-[10px] font-medium text-slate-300">{value}</span>
      </div>
      <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${barColors[color]}`} style={{ width: `${percent}%` }} />
      </div>
    </div>
  )
}
