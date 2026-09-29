import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import Progress from '@/components/Progress'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { phaseTone } from '@/lib/phase'
import { errorMessage } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

const isNum = (n: unknown): n is number => typeof n === 'number' && Number.isFinite(n)

async function copyUuid(uuid: string) {
  try {
    await navigator.clipboard.writeText(uuid)
    notify.success('GPU UUID copied')
  } catch (err) {
    notify.error('Could not copy the UUID', err)
  }
}

export default function Nodes() {
  useDocumentTitle('Nodes')
  const { data: nodes, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 10000,
  })

  if (isLoading) {
    return (
      <>
        <PageHero eyebrow="Nodes" title="Every GPU, accounted for." lede="Physical GPU nodes and real-time metrics" />
        <Skeleton rows={4} />
      </>
    )
  }

  if (isError && !nodes) {
    return (
      <>
        <PageHero eyebrow="Nodes" title="Nodes unavailable." tint="red" />
        <ErrorState title="Could not load GPU nodes." error={error} onRetry={() => refetch()} retrying={isRefetching} />
      </>
    )
  }

  const list = nodes || []
  const ready = list.filter((n) => phaseTone(n.status?.phase) === 'ok').length
  const notReady = list.length - ready

  return (
    <>
      <PageHero eyebrow="Nodes" title="Every GPU, accounted for." lede="Physical GPU nodes and real-time metrics" />

      <div className="grid">
        {isError && (
          <div className="span3">
            <ErrorState title="Could not refresh nodes; showing the last data." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={
            list.length === 0
              ? 'No GPU nodes registered.'
              : notReady > 0
                ? `${notReady} node${notReady === 1 ? '' : 's'} not ready.`
                : `All ${list.length} nodes ready.`
          }
          tone={list.length === 0 ? undefined : notReady > 0 ? 'warn' : 'ok'}
          figures={[
            { label: 'Total nodes', value: list.length },
            { label: 'Total GPUs', value: list.reduce((sum, n) => sum + (n.spec?.gpuCount || 0), 0) },
            { label: 'RDMA enabled', value: list.filter((n) => n.spec?.rdma).length },
            { label: 'Ready', value: ready },
            { label: 'Not ready', value: notReady, tone: countTone(notReady) },
          ]}
        />

        {list.length === 0 && (
          <section className="card span3">
            <EmptyState title="No GPU nodes registered">
              FabricGpuNode resources are created by the Gryvia GPU operator when it discovers GPUs on a node. Check that the GPU operator is installed and its pods are running.
            </EmptyState>
          </section>
        )}

        {list.map((node) => {
          const phase = node.status?.phase
          return (
            <section key={node.metadata?.name} className="card span3">
              <p className="eyebrow">NODE</p>
              <h2 className="card-title">{node.spec?.nodeName || node.metadata?.name}</h2>
              <div className="row">
                <span className={`pill ${phaseTone(phase)}`}>{phase || 'Unknown'}</span>
                {node.spec?.rdma && <span className="pill info">RDMA</span>}
              </div>

              <div className="apple-metric-band">
                <div>
                  <span>GPU type</span>
                  <b>{node.spec?.gpuType || '—'}</b>
                </div>
                <div>
                  <span>GPU count</span>
                  <b>{node.spec?.gpuCount ?? '—'}</b>
                </div>
                <div>
                  <span>GPU memory (per GPU)</span>
                  <b>{node.spec?.memoryGB ? `${node.spec.memoryGB} GB` : '—'}</b>
                </div>
              </div>

              {node.status?.gpuStatus && node.status.gpuStatus.length > 0 && (
                <div className="formgrid">
                  {node.status.gpuStatus.map((gpu) => {
                    const hasMem = isNum(gpu.memoryUsed) && isNum(gpu.memoryTotal) && gpu.memoryTotal > 0
                    const memUsedGB = hasMem ? (gpu.memoryUsed as number) / 1024 : 0
                    const memTotalGB = hasMem ? (gpu.memoryTotal as number) / 1024 : 0
                    const memPercent = hasMem ? Math.min(100, (memUsedGB / memTotalGB) * 100) : 0
                    const gpuName = `${node.spec?.nodeName || node.metadata?.name} GPU ${gpu.index}`
                    return (
                      <div key={gpu.index} className="stack">
                        <div className="row">
                          <b>GPU {gpu.index}</b>
                          {gpu.health && <span className={`pill ${phaseTone(gpu.health)}`}>{gpu.health}</span>}
                          {gpu.uuid && (
                            <button type="button" className="btn-secondary mono" title={gpu.uuid} aria-label={`Copy UUID of GPU ${gpu.index}`} onClick={() => copyUuid(gpu.uuid!)}>
                              {gpu.uuid.length > 16 ? `${gpu.uuid.slice(0, 12)}…` : gpu.uuid}
                            </button>
                          )}
                        </div>
                        <MetricBar
                          label="Temperature"
                          available={isNum(gpu.temperature)}
                          value={`${gpu.temperature}°C`}
                          percent={Math.min(gpu.temperature ?? 0, 100)}
                          tone={(gpu.temperature ?? 0) > 80 ? 'bad' : (gpu.temperature ?? 0) > 70 ? 'warn' : undefined}
                          barLabel={`${gpuName} temperature`}
                        />
                        <MetricBar
                          label="Utilization"
                          available={isNum(gpu.utilization)}
                          value={`${gpu.utilization}%`}
                          percent={Math.min(gpu.utilization ?? 0, 100)}
                          barLabel={`${gpuName} utilization`}
                        />
                        <MetricBar
                          label="GPU memory"
                          available={hasMem}
                          value={`${memUsedGB.toFixed(1)} / ${memTotalGB.toFixed(1)} GB`}
                          percent={memPercent}
                          barLabel={`${gpuName} memory`}
                        />
                      </div>
                    )
                  })}
                </div>
              )}
            </section>
          )
        })}
      </div>
    </>
  )
}

function MetricBar({ label, value, percent, tone, available, barLabel }: {
  label: string; value: string; percent: number; tone?: 'warn' | 'bad'; available: boolean; barLabel: string
}) {
  return (
    <div>
      <div className="row">
        <span className="faint">{label}</span>
        <span className="num">{available ? value : '—'}</span>
      </div>
      <Progress value={available ? percent : 0} tone={tone} label={available ? barLabel : `${barLabel} (no data)`} />
    </div>
  )
}
