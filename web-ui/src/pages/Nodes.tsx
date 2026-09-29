import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'

export default function Nodes() {
  const { data: nodes, isLoading, isError } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 10000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Nodes" title="Nodes unavailable." tint="red" />
        <p className="login-error" role="alert">Failed to load GPU nodes. Please check your API connection.</p>
      </>
    )
  }

  const notReady = nodes?.filter((n) => n.status?.phase !== 'Ready').length || 0

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Nodes" title="Every GPU, accounted for." lede="Physical GPU nodes and real-time metrics" />

      <div className="apple-metric-band">
        <div>
          <span>Total Nodes</span>
          <b>{nodes?.length || 0}</b>
        </div>
        <div>
          <span>Total GPUs</span>
          <b>{nodes?.reduce((sum, n) => sum + (n.spec?.gpuCount || 0), 0) || 0}</b>
        </div>
        <div>
          <span>RDMA Enabled</span>
          <b>{nodes?.filter((n) => n.spec?.rdma).length || 0}</b>
        </div>
        <div>
          <span>Ready Nodes · {notReady} degraded</span>
          <b>{nodes?.filter((n) => n.status?.phase === 'Ready').length || 0}</b>
        </div>
      </div>

      <div className="stack">
        {nodes?.map((node) => {
          const phase = node.status?.phase
          const phaseCls = phase === 'Ready' ? 'ok' : phase === 'Degraded' ? 'warn' : 'bad'
          return (
            <section key={node.metadata?.name} className="card">
              <div className="stat-head">
                <h2>{node.spec?.nodeName || node.metadata?.name}</h2>
                <div className="row">
                  <span className={`pill ${phaseCls}`}>{phase || 'Unknown'}</span>
                  {node.spec?.rdma && <span className="pill info">RDMA</span>}
                </div>
              </div>

              <div className="apple-metric-band" style={{ marginBottom: 20 }}>
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
                <div className="card-grid">
                  {node.status.gpuStatus.map((gpu) => {
                    const memUsedGB = Number(gpu.memoryUsed) / 1024 || 0
                    const memTotalGB = Number(gpu.memoryTotal) / 1024 || 0
                    const temperature = gpu.temperature ?? 0
                    const utilization = gpu.utilization ?? 0
                    const memPercent = memTotalGB > 0 ? Math.min(100, (memUsedGB / memTotalGB) * 100) : 0
                    return (
                      <div key={gpu.index}>
                        <div className="stat-head" style={{ marginBottom: 12 }}>
                          <h3>GPU {gpu.index}</h3>
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
    </div>
  )
}

function MetricBar({ label, value, percent, tone = '' }: {
  label: string; value: string; percent: number; tone?: '' | 'warn' | 'bad';
}) {
  return (
    <div style={{ marginBottom: 12 }}>
      <div className="stat-head" style={{ marginBottom: 6 }}>
        <span className="faint">{label}</span>
        <span>{value}</span>
      </div>
      <div className={`progress ${tone}`}>
        <span style={{ width: `${percent}%` }} />
      </div>
    </div>
  )
}
