import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { TrainingInsight, GPUMemStats } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'

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
      <>
        <PageHero eyebrow="GPU communication" title="Communication data unavailable." tint="red" />
        <p className="login-error" role="alert">
          Failed to load GPU communication data. Please check your API connection.
        </p>
      </>
    )
  }

  const totalOps = ncclStats?.operations?.length ?? 0
  const stragglerCount = trainingInsight?.stragglers?.length ?? 0
  const commPattern = trainingInsight?.commPattern ?? '-'
  const bottleneck = trainingInsight?.bottleneck ?? '-'

  return (
    <div className="apple-story-stack">
      <PageHero
        eyebrow="GPU communication"
        title="How your GPUs talk."
        lede="NCCL collective operations, memory transfers, and training analysis"
      />

      <div className="apple-metric-band">
        <div>
          <span>NCCL Operations</span>
          <b>{totalOps}</b>
        </div>
        <div>
          <span>Stragglers</span>
          <b className={stragglerCount > 0 ? 'text-bad' : undefined}>{stragglerCount}</b>
        </div>
        <div>
          <span>Comm Pattern</span>
          <b>{commPattern.replace('_', ' ')}</b>
        </div>
        <div>
          <span>Bottleneck</span>
          <b className={bottleneck === 'communication' ? 'text-warn' : undefined}>{bottleneck.replace('_', ' ')}</b>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <section className="card">
          <h2>NCCL Collective Operations</h2>
          {(!ncclStats?.operations || ncclStats.operations.length === 0) ? (
            <div className="list-empty">
              No NCCL operation data available. Ensure training jobs are running with eBPF tracing.
            </div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Op type</th>
                    <th style={{ textAlign: 'right' }}>Count</th>
                    <th style={{ textAlign: 'right' }}>Avg latency</th>
                    <th style={{ textAlign: 'right' }}>P99 latency</th>
                    <th style={{ textAlign: 'right' }}>Total bytes</th>
                  </tr>
                </thead>
                <tbody>
                  {ncclStats.operations.map((op) => (
                    <tr key={op.opType}>
                      <td className="mono">{op.opType}</td>
                      <td style={{ textAlign: 'right' }}>{op.count.toLocaleString()}</td>
                      <td style={{ textAlign: 'right' }}>{formatNs(op.avgLatencyNs)}</td>
                      <td style={{ textAlign: 'right' }}>{formatNs(op.p99LatencyNs)}</td>
                      <td className="muted" style={{ textAlign: 'right' }}>{formatBytes(op.totalBytes)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="card">
          <h2>Rank Communication Heatmap</h2>
          <RankHeatmap rankStats={trainingInsight?.rankStats} />
        </section>

        <section className="card">
          <h2>Straggler Detection</h2>
          {(!trainingInsight?.stragglers || trainingInsight.stragglers.length === 0) ? (
            <div className="list-empty">
              No stragglers detected. All ranks performing within normal bounds.
            </div>
          ) : (
            trainingInsight.stragglers.map((straggler) => (
              <div key={straggler.rank} className="list-row">
                <div className="grow">
                  <b>Rank {straggler.rank}</b>
                  <small>{straggler.reason?.replace(/_/g, ' ') || 'Unknown cause'}</small>
                </div>
                <span className="pill bad">{straggler.slowdownFactor.toFixed(1)}x slower</span>
              </div>
            ))
          )}

          <h3 style={{ marginTop: 20 }}>Training Bottleneck</h3>
          <BottleneckIndicator bottleneck={bottleneck} ratio={trainingInsight?.commComputeRatio ?? 0} />
        </section>

        <section className="card">
          <h2>GPU Memory Transfers</h2>
          <GPUMemoryChart stats={gpuMemStats} />
        </section>
      </div>
    </div>
  )
}

// --- Sub-components ---

function RankHeatmap({ rankStats }: { rankStats?: TrainingInsight['rankStats'] }) {
  if (!rankStats || rankStats.length === 0) {
    return <div className="list-empty">No rank data available.</div>
  }

  const maxLatency = Math.max(...rankStats.map(r => r.avgLatencyNs))
  const gridSize = Math.ceil(Math.sqrt(rankStats.length))

  return (
    <div className="stack" style={{ alignItems: 'center' }}>
      <div className="grid gap-1" style={{
        gridTemplateColumns: `repeat(${gridSize}, minmax(0, 1fr))`,
        width: '100%',
        maxWidth: '280px',
      }}>
        {rankStats.map((rank) => {
          const intensity = maxLatency > 0 ? rank.avgLatencyNs / maxLatency : 0
          return (
            <div
              key={rank.rank}
              className="aspect-square flex items-center justify-center mono"
              style={{
                position: 'relative',
                borderRadius: 6,
                fontSize: 11,
                background: 'var(--fill-tertiary)',
                border: rank.isStraggler ? '1px solid var(--danger)' : '1px solid var(--hairline-1)',
                color: rank.isStraggler ? 'var(--danger)' : 'var(--text-secondary)',
                overflow: 'hidden',
              }}
              title={`Rank ${rank.rank}: ${formatNs(rank.avgLatencyNs)} avg, ${formatBytes(rank.totalBytes)}`}
            >
              <span
                aria-hidden
                style={{
                  position: 'absolute',
                  inset: 0,
                  background: 'var(--apple-blue)',
                  opacity: 0.08 + intensity * 0.4,
                }}
              />
              <span style={{ position: 'relative' }}>{rank.rank}</span>
            </div>
          )
        })}
      </div>
      <div className="row faint">
        <span>Lighter = faster</span>
        <span>Darker = slower</span>
        <span className="text-bad">Outlined = straggler</span>
      </div>
    </div>
  )
}

function BottleneckIndicator({ bottleneck, ratio }: { bottleneck: string; ratio: number }) {
  const categories = [
    { key: 'compute', label: 'Compute', tone: 'warn' },
    { key: 'communication', label: 'Communication', tone: 'bad' },
    { key: 'data_loading', label: 'Data Loading', tone: 'warn' },
  ]

  return (
    <div>
      {categories.map(cat => {
        const isActive = cat.key === bottleneck
        return (
          <div key={cat.key} className="list-row">
            <span className={`dot ${isActive ? cat.tone : ''}`} />
            <span className={`grow ${isActive ? '' : 'muted'}`}>{cat.label}</span>
            {cat.key === 'communication' && (
              <span className="faint">{(ratio * 100).toFixed(0)}%</span>
            )}
          </div>
        )
      })}
    </div>
  )
}

function GPUMemoryChart({ stats }: { stats?: GPUMemStats }) {
  const transfers = [
    { label: 'H2D', sublabel: 'Host to Device', bytes: stats?.h2dBytes ?? 0 },
    { label: 'D2H', sublabel: 'Device to Host', bytes: stats?.d2hBytes ?? 0 },
    { label: 'D2D', sublabel: 'Device to Device', bytes: stats?.d2dBytes ?? 0 },
  ]

  const maxBytes = Math.max(...transfers.map(t => t.bytes), 1)

  return (
    <div className="stack">
      {transfers.map(t => (
        <div key={t.label}>
          <div className="stat-head" style={{ marginBottom: 6 }}>
            <span>
              <b>{t.label}</b> <span className="faint">{t.sublabel}</span>
            </span>
            <span className="mono muted">{formatBytes(t.bytes)}</span>
          </div>
          <div className="progress">
            <span style={{ width: `${Math.max((t.bytes / maxBytes) * 100, 2)}%` }} />
          </div>
        </div>
      ))}
      {(!stats || (stats.h2dBytes === 0 && stats.d2hBytes === 0 && stats.d2dBytes === 0)) && (
        <div className="list-empty">No memory transfer data available yet.</div>
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
