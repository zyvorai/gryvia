import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

export default function Nodes() {
  const { data: nodes, isLoading, isError, dataUpdatedAt } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 10000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Nodes" title="Nodes unavailable." tint="red" />
        <p className="warning" role="alert">Failed to load GPU nodes. Please check your API connection.</p>
      </>
    )
  }

  const notReady = nodes?.filter((n) => n.status?.phase !== 'Ready').length || 0

  return (
    <>
      <PageHero eyebrow="Nodes" title="Every GPU, accounted for." lede="Physical GPU nodes and real-time metrics" />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={notReady > 0 ? `${notReady} node${notReady === 1 ? '' : 's'} not ready.` : `All ${nodes?.length || 0} nodes ready.`}
          tone={notReady > 0 ? 'warn' : 'ok'}
          figures={[
            { label: 'Total nodes', value: nodes?.length || 0 },
            { label: 'Total GPUs', value: nodes?.reduce((sum, n) => sum + (n.spec?.gpuCount || 0), 0) || 0 },
            { label: 'RDMA enabled', value: nodes?.filter((n) => n.spec?.rdma).length || 0 },
            { label: 'Ready', value: nodes?.filter((n) => n.status?.phase === 'Ready').length || 0 },
            { label: 'Not ready', value: notReady, tone: countTone(notReady) },
          ]}
        />

        {nodes?.map((node) => {
          const phase = node.status?.phase
          const phaseCls = phase === 'Ready' ? 'ok' : phase === 'Degraded' ? 'warn' : 'bad'
          return (
            <section key={node.metadata?.name} className="card span3">
              <p className="eyebrow">NODE</p>
              <h2 className="card-title">{node.spec?.nodeName || node.metadata?.name}</h2>
              <div className="row">
                <span className={`pill ${phaseCls}`}>{phase || 'Unknown'}</span>
                {node.spec?.rdma && <span className="pill info">RDMA</span>}
              </div>

              <div className="apple-metric-band">
                <div>
                  <span>GPU Type</span>
                  <b>{node.spec?.gpuType}</b>
                </div>
                <div>
                  <span>GPU Count</span>
                  <b>{node.spec?.gpuCount}</b>
                </div>
                <div>
                  <span>Memory</span>
                  <b>{node.spec?.memoryGB ? `${node.spec.memoryGB} GB` : 'N/A'}</b>
                </div>
              </div>

              {node.status?.gpuStatus && node.status.gpuStatus.length > 0 && (
                <div className="formgrid">
                  {node.status.gpuStatus.map((gpu) => {
                    const memUsedGB = Number(gpu.memoryUsed) / 1024 || 0
                    const memTotalGB = Number(gpu.memoryTotal) / 1024 || 0
                    const temperature = gpu.temperature ?? 0
                    const utilization = gpu.utilization ?? 0
                    const memPercent = memTotalGB > 0 ? Math.min(100, (memUsedGB / memTotalGB) * 100) : 0
                    return (
                      <div key={gpu.index} className="stack">
                        <div className="row">
                          <b>GPU {gpu.index}</b>
                          <span className="faint mono">{gpu.uuid?.substring(0, 12)}...</span>
                        </div>
                        <MetricBar
                          label="Temperature"
                          value={`${temperature}°C`}
                          percent={Math.min(temperature, 100)}
                          tone={temperature > 80 ? 'bad' : temperature > 70 ? 'warn' : ''}
                        />
                        <MetricBar label="Utilization" value={`${utilization}%`} percent={Math.min(utilization, 100)} />
                        <MetricBar
                          label="Memory"
                          value={`${memUsedGB.toFixed(1)} / ${memTotalGB.toFixed(1)} GB`}
                          percent={memPercent}
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

function MetricBar({ label, value, percent, tone = '' }: {
  label: string; value: string; percent: number; tone?: '' | 'warn' | 'bad';
}) {
  return (
    <div>
      <div className="row">
        <span className="faint">{label}</span>
        <span>{value}</span>
      </div>
      <div className={`progress ${tone}`}>
        <span style={{ width: `${percent}%` }} />
      </div>
    </div>
  )
}
