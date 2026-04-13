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
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load GPU nodes. Please check your API connection.
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gradient-copper">GPU Nodes</h2>
        <p className="text-sm text-[#5a7a9e] mt-1">Physical GPU nodes and real-time metrics</p>
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
          <div key={node.metadata?.name} className="rounded-xl overflow-hidden" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="px-5 py-4 flex items-center justify-between" style={{
              borderBottom: '1px solid rgba(192,204,224,0.04)',
            }}>
              <div className="flex items-center gap-3">
                <Server className="h-4 w-4 text-[#7ecbf5]" />
                <h3 className="text-sm font-semibold text-[#e8ecf1]">{node.spec?.nodeName || node.metadata?.name}</h3>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
                  background: node.status?.phase === 'Ready' ? 'rgba(34,197,94,0.08)' :
                    node.status?.phase === 'Degraded' ? 'rgba(251,191,36,0.08)' : 'rgba(239,68,68,0.08)',
                  color: node.status?.phase === 'Ready' ? '#4ade80' :
                    node.status?.phase === 'Degraded' ? '#fbbf24' : '#f87171',
                }}>
                  {node.status?.phase || 'Unknown'}
                </span>
                {node.spec?.rdmaEnabled && (
                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-full"
                    style={{ background: 'rgba(95,168,211,0.08)', color: '#7ecbf5' }}
                  >RDMA</span>
                )}
              </div>
            </div>

            <div className="px-5 py-4">
              {/* Node Info */}
              <div className="grid grid-cols-3 gap-4 mb-4">
                <div className="p-2.5 rounded-lg" style={{
                  background: 'rgba(10,14,20,0.5)',
                  border: '1px solid rgba(192,204,224,0.04)',
                  boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                }}>
                  <div className="text-[10px] text-[#5a7a9e]">GPU Type</div>
                  <div className="text-xs font-medium text-[#b0c4d8]">{node.spec?.gpuType}</div>
                </div>
                <div className="p-2.5 rounded-lg" style={{
                  background: 'rgba(10,14,20,0.5)',
                  border: '1px solid rgba(192,204,224,0.04)',
                  boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                }}>
                  <div className="text-[10px] text-[#5a7a9e]">GPU Count</div>
                  <div className="text-xs font-medium text-[#b0c4d8]">{node.spec?.gpuCount}</div>
                </div>
                <div className="p-2.5 rounded-lg" style={{
                  background: 'rgba(10,14,20,0.5)',
                  border: '1px solid rgba(192,204,224,0.04)',
                  boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                }}>
                  <div className="text-[10px] text-[#5a7a9e]">Memory</div>
                  <div className="text-xs font-medium text-[#b0c4d8]">{node.spec?.memory || 'N/A'}</div>
                </div>
              </div>

              {/* GPU Metrics */}
              {node.status?.gpus && node.status.gpus.length > 0 && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {node.status.gpus.map((gpu) => (
                    <div key={gpu.index} className="rounded-lg p-3" style={{
                      background: 'rgba(10,14,20,0.4)',
                      border: '1px solid rgba(192,204,224,0.04)',
                    }}>
                      <div className="flex items-center justify-between mb-3">
                        <div className="flex items-center gap-2">
                          <Cpu className="h-3.5 w-3.5 text-[#7ecbf5]" />
                          <span className="text-xs font-medium text-[#e8ecf1]">GPU {gpu.index}</span>
                        </div>
                        <span className="text-[10px] text-[#5a7a9e] font-mono">{gpu.uuid?.substring(0, 12)}...</span>
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
  const barGradients: Record<string, string> = {
    blue: 'linear-gradient(90deg, #344e6a, #5fa8d3)',
    green: 'linear-gradient(90deg, #166534, #4ade80)',
    yellow: 'linear-gradient(90deg, #7c3a1a, #fbbf24)',
    red: 'linear-gradient(90deg, #7f1d1d, #ef4444)',
    purple: 'linear-gradient(90deg, #3b0764, #a855f7)',
  }
  return (
    <div className="mb-2.5 last:mb-0">
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-1">
          <Icon className="h-3 w-3 text-[#5a7a9e]" />
          <span className="text-[10px] text-[#5a7a9e]">{label}</span>
        </div>
        <span className="text-[10px] font-medium text-[#b0c4d8]">{value}</span>
      </div>
      <div className="h-1.5 rounded-full overflow-hidden" style={{
        background: 'rgba(10,14,20,0.6)',
        boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.4)',
      }}>
        <div className="h-full rounded-full" style={{
          width: `${percent}%`,
          background: barGradients[color],
          boxShadow: '0 0 6px rgba(192,204,224,0.08)',
        }} />
      </div>
    </div>
  )
}
