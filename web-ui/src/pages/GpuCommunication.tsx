import { useQuery } from '@tanstack/react-query'
import {
  Cpu, Activity, AlertTriangle, BarChart3, Clock,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { NCCLStats, TrainingInsight, GPUMemStats } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function GpuCommunication() {
  const { data: ncclStats, isLoading: ncclLoading, isError: ncclError } = useQuery({
    queryKey: ['ncclStats'],
    queryFn: api.getNCCLStats,
    refetchInterval: 15000,
  })

  const { data: trainingInsight } = useQuery({
    queryKey: ['trainingInsight'],
    queryFn: api.getTrainingInsight,
    refetchInterval: 15000,
  })

  const { data: gpuMemStats } = useQuery({
    queryKey: ['gpuMemoryStats'],
    queryFn: api.getGPUMemoryStats,
    refetchInterval: 15000,
  })

  if (ncclLoading) return <LoadingSpinner />

  if (ncclError) {
    return (
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load GPU communication data. Please check your API connection.
      </div>
    )
  }

  const totalOps = ncclStats?.operations?.length ?? 0
  const stragglerCount = trainingInsight?.stragglers?.length ?? 0
  const commPattern = trainingInsight?.commPattern ?? '-'
  const bottleneck = trainingInsight?.bottleneck ?? '-'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-purple">GPU Communication</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">NCCL collective operations, memory transfers, and training analysis</p>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={Activity}
          title="NCCL Operations"
          value={totalOps}
          color="purple"
        />
        <StatCard
          icon={AlertTriangle}
          title="Stragglers"
          value={stragglerCount}
          color={stragglerCount > 0 ? 'red' : 'green'}
        />
        <StatCard
          icon={Cpu}
          title="Comm Pattern"
          value={commPattern.replace('_', ' ')}
          color="blue"
        />
        <StatCard
          icon={BarChart3}
          title="Bottleneck"
          value={bottleneck.replace('_', ' ')}
          color={bottleneck === 'communication' ? 'orange' : 'cyan'}
        />
      </div>

      {/* Main content grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* NCCL Collective Operation Stats */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <Activity className="h-4 w-4 text-purple-400" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">NCCL Collective Operations</h3>
          </div>
          {(!ncclStats?.operations || ncclStats.operations.length === 0) ? (
            <div className="text-center py-8 text-sm text-[#344e6a]">
              No NCCL operation data available. Ensure training jobs are running with eBPF tracing.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
                    <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">OP TYPE</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">COUNT</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">AVG LATENCY</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">P99 LATENCY</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">TOTAL BYTES</th>
                  </tr>
                </thead>
                <tbody>
                  {ncclStats.operations.map((op) => (
                    <tr key={op.opType} className="table-row-hover" style={{ borderBottom: '1px solid rgba(192,204,224,0.03)' }}>
                      <td className="py-2 px-2 text-[#c0cce0] font-mono text-[11px]">{op.opType}</td>
                      <td className="py-2 px-2 text-right text-[#c0cce0]">{op.count.toLocaleString()}</td>
                      <td className="py-2 px-2 text-right text-[#8ba4c0]">{formatNs(op.avgLatencyNs)}</td>
                      <td className="py-2 px-2 text-right text-[#8ba4c0]">{formatNs(op.p99LatencyNs)}</td>
                      <td className="py-2 px-2 text-right text-[#5a7a9e]">{formatBytes(op.totalBytes)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Rank-to-Rank Communication Heatmap */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <Cpu className="h-4 w-4 text-purple-400" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Rank Communication Heatmap</h3>
          </div>
          <RankHeatmap rankStats={trainingInsight?.rankStats} />
        </div>

        {/* Straggler Detection Panel */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <AlertTriangle className="h-4 w-4 text-[#fbbf24]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Straggler Detection</h3>
          </div>
          {(!trainingInsight?.stragglers || trainingInsight.stragglers.length === 0) ? (
            <div className="text-center py-6 text-sm text-[#344e6a]">
              No stragglers detected. All ranks performing within normal bounds.
            </div>
          ) : (
            <div className="space-y-2">
              {trainingInsight.stragglers.map((straggler) => (
                <div key={straggler.rank} className="p-3 rounded-lg" style={{
                  background: 'rgba(239,68,68,0.05)',
                  border: '1px solid rgba(239,68,68,0.1)',
                }}>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs font-medium text-[#f87171]">
                      Rank {straggler.rank}
                    </span>
                    <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
                      background: 'rgba(239,68,68,0.1)',
                      color: '#f87171',
                    }}>
                      {straggler.slowdownFactor.toFixed(1)}x slower
                    </span>
                  </div>
                  <div className="text-[10px] text-[#5a7a9e]">{straggler.reason?.replace(/_/g, ' ') || 'Unknown cause'}</div>
                </div>
              ))}
            </div>
          )}

          {/* Bottleneck Indicator */}
          <div className="mt-4 pt-4" style={{ borderTop: '1px solid rgba(192,204,224,0.06)' }}>
            <div className="text-[10px] text-[#5a7a9e] uppercase tracking-wide mb-2">Training Bottleneck</div>
            <BottleneckIndicator bottleneck={bottleneck} ratio={trainingInsight?.commComputeRatio ?? 0} />
          </div>
        </div>

        {/* GPU Memory Transfer Bandwidth */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <BarChart3 className="h-4 w-4 text-purple-400" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">GPU Memory Transfers</h3>
          </div>
          <GPUMemoryChart stats={gpuMemStats} />
        </div>
      </div>
    </div>
  )
}

// --- Sub-components ---

function RankHeatmap({ rankStats }: { rankStats?: TrainingInsight['rankStats'] }) {
  if (!rankStats || rankStats.length === 0) {
    return (
      <div className="text-center py-8 text-sm text-[#344e6a]">
        No rank data available.
      </div>
    )
  }

  const maxLatency = Math.max(...rankStats.map(r => r.avgLatencyNs))
  const gridSize = Math.ceil(Math.sqrt(rankStats.length))

  return (
    <div className="flex flex-col items-center gap-2">
      <div className="grid gap-1" style={{
        gridTemplateColumns: `repeat(${gridSize}, minmax(0, 1fr))`,
        width: '100%',
        maxWidth: '280px',
      }}>
        {rankStats.map((rank) => {
          const intensity = maxLatency > 0 ? rank.avgLatencyNs / maxLatency : 0
          const r = Math.round(168 + intensity * 71)
          const g = Math.round(85 - intensity * 60)
          const b = Math.round(247 - intensity * 120)
          return (
            <div
              key={rank.rank}
              className="aspect-square rounded flex items-center justify-center text-[9px] font-mono"
              style={{
                background: `rgba(${r}, ${g}, ${b}, ${0.15 + intensity * 0.45})`,
                border: rank.isStraggler
                  ? '1px solid rgba(239,68,68,0.5)'
                  : '1px solid rgba(192,204,224,0.04)',
                color: rank.isStraggler ? '#f87171' : '#8ba4c0',
              }}
              title={`Rank ${rank.rank}: ${formatNs(rank.avgLatencyNs)} avg, ${formatBytes(rank.totalBytes)}`}
            >
              {rank.rank}
            </div>
          )
        })}
      </div>
      <div className="flex items-center gap-4 text-[10px] text-[#5a7a9e] mt-1">
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ background: 'rgba(168,85,247,0.2)' }} />
          Fast
        </div>
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ background: 'rgba(239,25,127,0.5)' }} />
          Slow
        </div>
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ background: 'rgba(239,68,68,0.4)', border: '1px solid rgba(239,68,68,0.5)' }} />
          Straggler
        </div>
      </div>
    </div>
  )
}

function BottleneckIndicator({ bottleneck, ratio }: { bottleneck: string; ratio: number }) {
  const categories = [
    { key: 'compute', label: 'Compute', color: '#fbbf24' },
    { key: 'communication', label: 'Communication', color: '#f87171' },
    { key: 'data_loading', label: 'Data Loading', color: '#60a5fa' },
  ]

  return (
    <div className="space-y-2">
      {categories.map(cat => {
        const isActive = cat.key === bottleneck
        return (
          <div key={cat.key} className="flex items-center gap-3">
            <div className="w-3 h-3 rounded-full" style={{
              background: isActive ? cat.color : '#1a2332',
              border: `1px solid ${isActive ? cat.color : 'rgba(192,204,224,0.1)'}`,
              boxShadow: isActive ? `0 0 6px ${cat.color}40` : 'none',
            }} />
            <span className="text-xs flex-1" style={{ color: isActive ? cat.color : '#5a7a9e' }}>
              {cat.label}
            </span>
            {cat.key === 'communication' && (
              <span className="text-[10px] text-[#5a7a9e]">{(ratio * 100).toFixed(0)}%</span>
            )}
          </div>
        )
      })}
    </div>
  )
}

function GPUMemoryChart({ stats }: { stats?: GPUMemStats }) {
  const transfers = [
    { label: 'H2D', sublabel: 'Host to Device', bytes: stats?.h2dBytes ?? 0, color: '#a855f7' },
    { label: 'D2H', sublabel: 'Device to Host', bytes: stats?.d2hBytes ?? 0, color: '#7c3aed' },
    { label: 'D2D', sublabel: 'Device to Device', bytes: stats?.d2dBytes ?? 0, color: '#6366f1' },
  ]

  const maxBytes = Math.max(...transfers.map(t => t.bytes), 1)

  return (
    <div className="space-y-3">
      {transfers.map(t => (
        <div key={t.label}>
          <div className="flex items-center justify-between mb-1">
            <div>
              <span className="text-xs font-medium text-[#c0cce0]">{t.label}</span>
              <span className="text-[10px] text-[#5a7a9e] ml-2">{t.sublabel}</span>
            </div>
            <span className="text-xs font-mono text-[#8ba4c0]">{formatBytes(t.bytes)}</span>
          </div>
          <div className="h-3 rounded-full overflow-hidden" style={{ background: 'rgba(10,14,20,0.5)' }}>
            <div
              className="h-full rounded-full transition-all duration-500"
              style={{
                width: `${Math.max((t.bytes / maxBytes) * 100, 2)}%`,
                background: `linear-gradient(90deg, ${t.color}80, ${t.color})`,
                boxShadow: `0 0 8px ${t.color}30`,
              }}
            />
          </div>
        </div>
      ))}
      {(!stats || (stats.h2dBytes === 0 && stats.d2hBytes === 0 && stats.d2dBytes === 0)) && (
        <div className="text-center py-4 text-[10px] text-[#344e6a]">
          No memory transfer data available yet.
        </div>
      )}
    </div>
  )
}

function formatNs(ns: number): string {
  if (ns >= 1_000_000_000) return `${(ns / 1_000_000_000).toFixed(2)}s`
  if (ns >= 1_000_000) return `${(ns / 1_000_000).toFixed(2)}ms`
  if (ns >= 1_000) return `${(ns / 1_000).toFixed(1)}us`
  return `${ns}ns`
}

function formatBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(2)} GB`
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}
