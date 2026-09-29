import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { TrainingInsight, GPUMemStats } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

export default function GpuCommunication() {
  const { data: ncclStats, isLoading: ncclLoading, isError: ncclError, dataUpdatedAt } = useQuery({
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
        <p className="warning" role="alert">
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
    <>
      <PageHero
        eyebrow="GPU communication"
        title="How your GPUs talk."
        lede="NCCL collective operations, memory transfers, and training analysis"
      />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={
            stragglerCount > 0
              ? `${stragglerCount} straggler rank${stragglerCount === 1 ? '' : 's'} slowing the job.`
              : bottleneck === 'communication'
                ? 'Training is bottlenecked on communication.'
                : 'All ranks performing within normal bounds.'
          }
          tone={stragglerCount > 0 ? 'bad' : bottleneck === 'communication' ? 'warn' : undefined}
          figures={[
            { label: 'NCCL operations', value: totalOps },
            { label: 'stragglers', value: stragglerCount, tone: countTone(stragglerCount) },
            { label: 'comm pattern', value: commPattern.replace('_', ' ') },
            { label: 'bottleneck', value: bottleneck.replace('_', ' '), tone: bottleneck === 'communication' ? 'warn' : undefined },
          ]}
        />

        <section className="card span2">
          <p className="eyebrow">NCCL</p>
          <h2 className="card-title">Collective operations</h2>
          {(!ncclStats?.operations || ncclStats.operations.length === 0) ? (
            <p className="empty-state">
              No NCCL operation data available. Ensure training jobs are running with eBPF tracing.
            </p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Op type</th>
                    <th>Count</th>
                    <th>Avg latency</th>
                    <th>P99 latency</th>
                    <th>Total bytes</th>
                  </tr>
                </thead>
                <tbody>
                  {ncclStats.operations.map((op) => (
                    <tr key={op.opType}>
                      <td className="mono">{op.opType}</td>
                      <td>{op.count.toLocaleString()}</td>
                      <td>{formatNs(op.avgLatencyNs)}</td>
                      <td>{formatNs(op.p99LatencyNs)}</td>
                      <td className="muted">{formatBytes(op.totalBytes)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="card">
          <p className="eyebrow">RANKS</p>
          <h2 className="card-title">Communication heatmap</h2>
          <RankHeatmap rankStats={trainingInsight?.rankStats} />
        </section>

        <section className="card span2">
          <p className="eyebrow">STRAGGLERS</p>
          <h2 className="card-title">Straggler detection</h2>
          {(!trainingInsight?.stragglers || trainingInsight.stragglers.length === 0) ? (
            <p className="empty-state">
              No stragglers detected. All ranks performing within normal bounds.
            </p>
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
        </section>

        <section className="card">
          <p className="eyebrow">BOTTLENECK</p>
          <h2 className="card-title">Training bottleneck</h2>
          <BottleneckIndicator bottleneck={bottleneck} ratio={trainingInsight?.commComputeRatio ?? 0} />
        </section>

        <section className="card span3">
          <p className="eyebrow">MEMORY</p>
          <h2 className="card-title">GPU memory transfers</h2>
          <GPUMemoryChart stats={gpuMemStats} />
        </section>
      </div>
    </>
  )
}

function RankHeatmap({ rankStats }: { rankStats?: TrainingInsight['rankStats'] }) {
  if (!rankStats || rankStats.length === 0) {
    return <p className="empty-state">No rank data available.</p>
  }

  const maxLatency = Math.max(...rankStats.map(r => r.avgLatencyNs))
  const gridSize = Math.ceil(Math.sqrt(rankStats.length))

  return (
    <div className="stack">
      <div style={{
        display: 'grid',
        gap: 4,
        gridTemplateColumns: `repeat(${gridSize}, minmax(0, 1fr))`,
        width: '100%',
        maxWidth: '280px',
      }}>
        {rankStats.map((rank) => {
          const intensity = maxLatency > 0 ? rank.avgLatencyNs / maxLatency : 0
          return (
            <div
              key={rank.rank}
              className="mono"
              style={{
                display: 'grid',
                placeItems: 'center',
                aspectRatio: '1',
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

  if (transfers.every(t => t.bytes === 0)) {
    return <p className="empty-state">No memory transfer data available yet.</p>
  }

  return (
    <div className="stack">
      {transfers.map(t => (
        <div key={t.label}>
          <div className="list-row">
            <div className="grow">
              <b>{t.label}</b>
              <small>{t.sublabel}</small>
            </div>
            <span className="mono muted">{formatBytes(t.bytes)}</span>
          </div>
          <div className="progress">
            <span style={{ width: `${Math.max((t.bytes / maxBytes) * 100, 2)}%` }} />
          </div>
        </div>
      ))}
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
