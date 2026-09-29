import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { TrainingInsight } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import Progress from '@/components/Progress'
import VisuallyHidden from '@/components/VisuallyHidden'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { formatBytes, formatDate, formatPercent, formatRelative } from '@/lib/format'
import { errorMessage } from '@/lib/errors'
import { collectorState, formatNs, humanize, normalizeBottleneck, ratioPercent, slowdownLabel, type Collectors } from '@/lib/gpuComm'

type WithCollectors = { collectors?: Collectors }
type InsightExt = TrainingInsight & { source?: { name?: string; namespace?: string } }

const NO_ANALYSIS_HELP = 'Training analysis is produced from NCCL traces of a running training job (a FabricAIJob with eBPF tracing enabled). Start or resume a training job and the analysis appears here.'

export default function GpuCommunication() {
  useDocumentTitle('GPU communication')
  const ncclQ = useQuery({ queryKey: ['ncclStats'], queryFn: api.getNCCLStats, refetchInterval: 15000 })
  const insightQ = useQuery({ queryKey: ['trainingInsight'], queryFn: api.getTrainingInsight, refetchInterval: 15000 })
  const memQ = useQuery({ queryKey: ['gpuMemoryStats'], queryFn: api.getGPUMemoryStats, refetchInterval: 15000 })

  const insight = insightQ.data as InsightExt | undefined
  const hasAnalysis = !!insight && ((insight.rankStats?.length ?? 0) > 0 || !!insight.lastAnalysis)
  const stragglerCount = insight?.stragglers?.length
  const bottleneck = normalizeBottleneck(insight?.bottleneck)
  const commPattern = humanize(insight?.commPattern)
  const ncclCollectors = (ncclQ.data as (WithCollectors & object) | undefined)?.collectors
  const memCollectors = (memQ.data as (WithCollectors & object) | undefined)?.collectors

  let headline: string | undefined
  if (insightQ.data) {
    if (!hasAnalysis) headline = 'No training analysis yet.'
    else if (stragglerCount && stragglerCount > 0) headline = `${stragglerCount} straggler rank${stragglerCount === 1 ? '' : 's'} slowing the job.`
    else if (bottleneck === 'communication') headline = 'Training is bottlenecked on communication.'
    else headline = 'No stragglers detected in the latest analysis.'
  }

  return (
    <>
      <PageHero eyebrow="GPU communication" title="How your GPUs talk." lede="NCCL collective operations, memory transfers, and training analysis" />

      <div className="grid">
        {ncclQ.isLoading ? (
          <section className="card span3" aria-label="Live summary">
            <Skeleton rows={2} />
          </section>
        ) : ncclQ.isError && !ncclQ.data ? (
          <div className="span3">
            <ErrorState title="Could not load NCCL data." error={ncclQ.error} onRetry={() => ncclQ.refetch()} retrying={ncclQ.isFetching} />
          </div>
        ) : (
          <PagePulse
            updatedAt={ncclQ.dataUpdatedAt}
            error={ncclQ.isError ? errorMessage(ncclQ.error) : undefined}
            headline={headline}
            tone={stragglerCount && stragglerCount > 0 ? 'bad' : bottleneck === 'communication' ? 'warn' : undefined}
            figures={[
              { label: 'Op types', value: ncclQ.data?.operations?.length },
              { label: 'stragglers', value: hasAnalysis ? stragglerCount : undefined, tone: stragglerCount && stragglerCount > 0 ? 'bad' : undefined },
              { label: 'comm pattern', value: hasAnalysis && commPattern ? commPattern : undefined },
              { label: 'bottleneck', value: hasAnalysis && bottleneck ? humanize(bottleneck) : undefined, tone: bottleneck === 'communication' ? 'warn' : undefined },
            ]}
          />
        )}

        <section className="card span3">
          <p className="eyebrow">ANALYSIS</p>
          <h2 className="card-title">Insight source</h2>
          {insightQ.isLoading ? (
            <Skeleton rows={1} />
          ) : insightQ.isError && !insightQ.data ? (
            <ErrorState title="Could not load the training insight." error={insightQ.error} onRetry={() => insightQ.refetch()} retrying={insightQ.isFetching} />
          ) : hasAnalysis ? (
            <p className="muted">
              {insight?.source?.name ? (
                <>
                  Showing insight <b>{insight.source.name}</b>
                  {insight.source.namespace && <> in namespace <b>{insight.source.namespace}</b></>}
                </>
              ) : (
                'Showing the latest training insight (source not reported)'
              )}
              {insight?.lastAnalysis && (
                <>
                  {' '}
                  · analysed <span title={formatDate(insight.lastAnalysis)}>{formatRelative(insight.lastAnalysis)}</span>
                </>
              )}
              {insightQ.isError && <span className="faint"> · refresh failed: {errorMessage(insightQ.error)}</span>}
            </p>
          ) : (
            <EmptyState title="No training analysis yet" action={<Link to="/jobs" className="buttonlike btn-secondary">View jobs</Link>}>
              {NO_ANALYSIS_HELP}
            </EmptyState>
          )}
        </section>

        <section className="card span2">
          <p className="eyebrow">NCCL</p>
          <h2 className="card-title">Collective operations</h2>
          {ncclQ.isLoading ? (
            <Skeleton rows={4} />
          ) : ncclQ.isError && !ncclQ.data ? (
            <ErrorState title="Could not load NCCL data." error={ncclQ.error} onRetry={() => ncclQ.refetch()} retrying={ncclQ.isFetching} />
          ) : !ncclQ.data?.operations || ncclQ.data.operations.length === 0 ? (
            <CollectorEmpty what="NCCL operations" collectors={ncclCollectors} onRetry={() => ncclQ.refetch()} retrying={ncclQ.isFetching} />
          ) : (
            <>
              <CollectorNote collectors={ncclCollectors} />
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Op type</th>
                      <th className="num">Count</th>
                      <th className="num">Avg latency</th>
                      <th className="num">Worst-node p99</th>
                      <th className="num">Total bytes</th>
                    </tr>
                  </thead>
                  <tbody>
                    {ncclQ.data.operations.map((op) => (
                      <tr key={op.opType}>
                        <td className="mono">{op.opType}</td>
                        <td className="num">{op.count.toLocaleString()}</td>
                        <td className="num">{formatNs(op.avgLatencyNs)}</td>
                        <td className="num">{formatNs(op.p99LatencyNs)}</td>
                        <td className="muted num">{formatBytes(op.totalBytes)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </section>

        <section className="card">
          <p className="eyebrow">RANKS</p>
          <h2 className="card-title">Rank latency</h2>
          {insightQ.isLoading ? <Skeleton rows={3} /> : insightQ.isError && !insightQ.data ? <ErrorState title="Could not load rank data." error={insightQ.error} onRetry={() => insightQ.refetch()} /> : <RankHeatmap rankStats={insight?.rankStats} />}
        </section>

        <section className="card span2">
          <p className="eyebrow">STRAGGLERS</p>
          <h2 className="card-title">Straggler detection</h2>
          {insightQ.isLoading ? (
            <Skeleton rows={2} />
          ) : insightQ.isError && !insightQ.data ? (
            <ErrorState title="Could not load straggler data." error={insightQ.error} onRetry={() => insightQ.refetch()} retrying={insightQ.isFetching} />
          ) : !hasAnalysis ? (
            <EmptyState title="No training analysis yet">{NO_ANALYSIS_HELP}</EmptyState>
          ) : !insight?.stragglers || insight.stragglers.length === 0 ? (
            <EmptyState title="No stragglers detected">The latest analysis found no rank lagging behind the others.</EmptyState>
          ) : (
            insight.stragglers.map((s) => (
              <div key={s.rank} className="list-row">
                <div className="grow">
                  <b>Rank {s.rank}</b>
                  <small>{humanize(s.reason) || 'Unknown cause'}</small>
                </div>
                <span className="pill bad">{slowdownLabel(s.slowdownFactor) ?? 'slowdown unknown'}</span>
              </div>
            ))
          )}
        </section>

        <section className="card">
          <p className="eyebrow">BOTTLENECK</p>
          <h2 className="card-title">Training bottleneck</h2>
          {insightQ.isLoading ? (
            <Skeleton rows={3} />
          ) : insightQ.isError && !insightQ.data ? (
            <ErrorState title="Could not load the bottleneck." error={insightQ.error} onRetry={() => insightQ.refetch()} retrying={insightQ.isFetching} />
          ) : !hasAnalysis ? (
            <EmptyState title="No training analysis yet" />
          ) : (
            <BottleneckIndicator bottleneck={bottleneck} ratio={insight?.commComputeRatio} />
          )}
        </section>

        <section className="card span3">
          <p className="eyebrow">MEMORY</p>
          <h2 className="card-title">GPU memory transfers</h2>
          {memQ.isLoading ? (
            <Skeleton rows={3} />
          ) : memQ.isError && !memQ.data ? (
            <ErrorState title="Could not load GPU memory transfers." error={memQ.error} onRetry={() => memQ.refetch()} retrying={memQ.isFetching} />
          ) : (
            <GPUMemoryChart stats={memQ.data} collectors={memCollectors} onRetry={() => memQ.refetch()} retrying={memQ.isFetching} />
          )}
        </section>
      </div>
    </>
  )
}

function CollectorNote({ collectors }: { collectors?: Collectors }) {
  if (collectorState(collectors) !== 'partial') return null
  return (
    <p className="warning" role="status">
      {collectors!.reachable} of {collectors!.total} collectors reachable; figures may be incomplete.
    </p>
  )
}

/** Empty data, explained by what the collectors report: unreachable, not deployed, or simply idle. */
function CollectorEmpty({ what, collectors, onRetry, retrying }: { what: string; collectors?: Collectors; onRetry: () => void; retrying?: boolean }) {
  const state = collectorState(collectors)
  const retry = (
    <button type="button" className="btn-secondary" onClick={onRetry} disabled={retrying}>
      {retrying ? 'Checking…' : 'Check again'}
    </button>
  )
  if (state === 'unreachable') {
    return (
      <EmptyState title="Collectors unreachable" action={retry}>
        All {collectors!.total} eBPF collector{collectors!.total === 1 ? '' : 's'} could not be reached, so {what} cannot be read. Check that the collector pods are running and reachable from the gateway.
      </EmptyState>
    )
  }
  if (state === 'none-deployed') {
    return (
      <EmptyState title="No collectors deployed" action={retry}>
        {what.charAt(0).toUpperCase() + what.slice(1)} come from eBPF collectors running on GPU nodes. None are deployed; deploy the collector DaemonSet to start collecting.
      </EmptyState>
    )
  }
  return (
    <EmptyState title={`No ${what} recorded`} action={retry}>
      {state === 'unknown' ? 'The eBPF collectors have not reported any yet.' : `${collectors!.reachable} of ${collectors!.total} collectors are reachable but have seen no activity.`} Run a training job with eBPF tracing to generate traffic.
    </EmptyState>
  )
}

function RankHeatmap({ rankStats }: { rankStats?: TrainingInsight['rankStats'] }) {
  if (!rankStats || rankStats.length === 0) {
    return <EmptyState title="No rank data yet">Per-rank latency appears once a training analysis exists.</EmptyState>
  }

  const latencies = rankStats.map((r) => r.avgLatencyNs).filter(Number.isFinite)
  const maxLatency = latencies.length ? Math.max(...latencies) : 0
  const minLatency = latencies.length ? Math.min(...latencies) : 0
  const gridSize = Math.ceil(Math.sqrt(rankStats.length))
  const stragglers = rankStats.filter((r) => r.isStraggler).length
  const summary = `Average NCCL latency per rank, ${rankStats.length} ranks, from ${formatNs(minLatency)} to ${formatNs(maxLatency)}; ${stragglers} straggler${stragglers === 1 ? '' : 's'}. A table follows.`

  return (
    <div className="stack">
      <div
        role="img"
        aria-label={summary}
        style={{ display: 'grid', gap: 4, gridTemplateColumns: `repeat(${gridSize}, minmax(0, 1fr))`, width: '100%', maxWidth: '280px' }}
      >
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
                border: rank.isStraggler ? '2px solid var(--danger)' : '1px solid var(--hairline-1)',
                color: rank.isStraggler ? 'var(--danger)' : 'var(--text-secondary)',
                overflow: 'hidden',
              }}
              title={`Rank ${rank.rank}: ${formatNs(rank.avgLatencyNs)} avg, ${formatBytes(rank.totalBytes)}${rank.isStraggler ? ' (straggler)' : ''}`}
            >
              <span aria-hidden style={{ position: 'absolute', inset: 0, background: 'var(--apple-blue)', opacity: 0.08 + intensity * 0.4 }} />
              <span style={{ position: 'relative' }}>
                {rank.isStraggler && <span aria-hidden="true">▲</span>}
                {rank.rank}
              </span>
            </div>
          )
        })}
      </div>
      <div className="row faint" style={{ flexWrap: 'wrap', gap: 12 }}>
        <span>
          Lighter = faster ({formatNs(minLatency)}) · Darker = slower ({formatNs(maxLatency)})
        </span>
        <span>▲ = straggler</span>
      </div>
      <VisuallyHidden>
        <table>
          <caption>Average NCCL latency per rank</caption>
          <thead>
            <tr>
              <th>Rank</th>
              <th>Average latency</th>
              <th>Total bytes</th>
              <th>Straggler</th>
            </tr>
          </thead>
          <tbody>
            {rankStats.map((r) => (
              <tr key={r.rank}>
                <td>{r.rank}</td>
                <td>{formatNs(r.avgLatencyNs)}</td>
                <td>{formatBytes(r.totalBytes)}</td>
                <td>{r.isStraggler ? 'Yes' : 'No'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </VisuallyHidden>
    </div>
  )
}

function BottleneckIndicator({ bottleneck, ratio }: { bottleneck: string; ratio?: number }) {
  const categories = [
    { key: 'compute', label: 'Compute', tone: 'warn' },
    { key: 'communication', label: 'Communication', tone: 'bad' },
    { key: 'data_loading', label: 'Data loading', tone: 'warn' },
  ]
  const pct = ratioPercent(ratio)
  const known = categories.some((c) => c.key === bottleneck)

  return (
    <div>
      {categories.map((cat) => {
        const isActive = cat.key === bottleneck
        return (
          <div key={cat.key} className="list-row">
            <span className={`dot ${isActive ? cat.tone : ''}`} aria-hidden="true" />
            <span className={`grow ${isActive ? '' : 'muted'}`}>
              {cat.label}
              {isActive && <span className="faint"> · current bottleneck</span>}
            </span>
          </div>
        )
      })}
      {!known && <small className="faint">{bottleneck ? `Reported bottleneck: ${humanize(bottleneck)}` : 'No bottleneck reported.'}</small>}
      <div className="list-row">
        <span className="grow muted">Comm/compute ratio</span>
        <span className="faint num">{pct === undefined ? '—' : formatPercent(Math.round(pct))}</span>
      </div>
    </div>
  )
}

function GPUMemoryChart({ stats, collectors, onRetry, retrying }: { stats?: import('@/lib/api').GPUMemStats; collectors?: Collectors; onRetry: () => void; retrying?: boolean }) {
  const transfers = [
    { label: 'H2D', sublabel: 'Host to device', bytes: stats?.h2dBytes ?? 0, count: stats?.h2dCount ?? 0 },
    { label: 'D2H', sublabel: 'Device to host', bytes: stats?.d2hBytes ?? 0, count: stats?.d2hCount ?? 0 },
    { label: 'D2D', sublabel: 'Device to device', bytes: stats?.d2dBytes ?? 0, count: stats?.d2dCount ?? 0 },
  ]
  const maxBytes = Math.max(...transfers.map((t) => t.bytes), 0)

  if (transfers.every((t) => t.bytes === 0 && t.count === 0)) {
    return <CollectorEmpty what="GPU memory transfers" collectors={collectors} onRetry={onRetry} retrying={retrying} />
  }

  return (
    <div className="stack">
      <CollectorNote collectors={collectors} />
      <small className="faint">Totals since the collectors started; they reset when a collector restarts.</small>
      {transfers.map((t) => (
        <div key={t.label}>
          <div className="list-row">
            <div className="grow">
              <b>{t.label}</b>
              <small>
                {t.sublabel} · {t.count.toLocaleString()} transfer{t.count === 1 ? '' : 's'}
              </small>
            </div>
            <span className="mono muted num">{formatBytes(t.bytes)}</span>
          </div>
          <Progress value={t.bytes} max={maxBytes} label={`${t.label} bytes relative to the largest transfer type`} />
        </div>
      ))}
    </div>
  )
}
