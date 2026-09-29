import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { ServiceGraphNode, ServiceGraphEdge, NetworkAnomaly } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import VisuallyHidden from '@/components/VisuallyHidden'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { phaseTone } from '@/lib/phase'
import { formatDate, formatRelative } from '@/lib/format'
import { errorMessage } from '@/lib/errors'
import { edgeGeometry, formatCaptureLevel, healthKind, severityTone, twoWayEdges, verdictKind, type HealthKind, type VerdictKind } from '@/lib/network'

export default function NetworkOverview() {
  useDocumentTitle('Network')
  const insightsQ = useQuery({ queryKey: ['trafficInsights'], queryFn: api.getTrafficInsights, refetchInterval: 15000 })
  const graphQ = useQuery({ queryKey: ['serviceGraph'], queryFn: api.getServiceGraph, refetchInterval: 30000 })
  const anomaliesQ = useQuery({ queryKey: ['networkAnomalies'], queryFn: api.getNetworkAnomalies, refetchInterval: 15000 })
  const tracesQ = useQuery({ queryKey: ['traceSessions'], queryFn: api.getTraceSessions, refetchInterval: 10000 })

  const insights = insightsQ.data
  const anomalyCount = insights?.anomaliesDetected
  const pulseError = insightsQ.isError ? errorMessage(insightsQ.error) : undefined

  return (
    <>
      <PageHero eyebrow="Network" title="Network intelligence." lede="Service mesh observability and flow management" />

      <div className="grid">
        <div className="toolbar span3">
          <Link to="/network/policies" className="buttonlike primary">
            Policies
          </Link>
          <Link to="/network/flows" className="buttonlike btn-secondary">
            View flows
          </Link>
        </div>

        {insightsQ.isLoading ? (
          <section className="card span3" aria-label="Live summary">
            <Skeleton rows={2} />
          </section>
        ) : insightsQ.isError && !insights ? (
          <div className="span3">
            <ErrorState title="Could not load network insights." error={insightsQ.error} onRetry={() => insightsQ.refetch()} retrying={insightsQ.isFetching} />
          </div>
        ) : (
          <PagePulse
            updatedAt={insightsQ.dataUpdatedAt}
            error={pulseError}
            headline={
              anomalyCount === undefined
                ? undefined
                : anomalyCount > 0
                  ? `${anomalyCount} anomal${anomalyCount === 1 ? 'y' : 'ies'} detected across the mesh.`
                  : 'No anomalies detected across the mesh.'
            }
            tone={anomalyCount && anomalyCount > 0 ? 'warn' : undefined}
            figures={[
              { label: 'Observed connections', value: insights?.activeFlows },
              { label: 'Policies enforced', value: insights?.policiesEnforced },
              { label: 'Anomalies detected', value: anomalyCount, tone: anomalyCount && anomalyCount > 0 ? 'warn' : undefined },
              { label: 'Trace sessions', value: insights?.traceSessions },
            ]}
          />
        )}

        <section className="card span2">
          <p className="eyebrow">TOPOLOGY</p>
          <h2 className="card-title">Service dependency graph</h2>
          {graphQ.isLoading ? (
            <Skeleton rows={4} />
          ) : graphQ.isError && !graphQ.data ? (
            <ErrorState title="Could not load the service graph." error={graphQ.error} onRetry={() => graphQ.refetch()} retrying={graphQ.isFetching} />
          ) : (
            <>
              {graphQ.isError && <p className="faint" role="status">Showing the last loaded graph; refresh failed: {errorMessage(graphQ.error)}</p>}
              <ServiceGraphView nodes={graphQ.data?.nodes || []} edges={graphQ.data?.edges || []} />
            </>
          )}
        </section>

        <section className="card">
          <p className="eyebrow">TRACES</p>
          <h2 className="card-title">Active traces</h2>
          {tracesQ.isLoading ? (
            <Skeleton rows={3} />
          ) : tracesQ.isError && !tracesQ.data ? (
            <ErrorState title="Could not load trace sessions." error={tracesQ.error} onRetry={() => tracesQ.refetch()} retrying={tracesQ.isFetching} />
          ) : !tracesQ.data || tracesQ.data.length === 0 ? (
            <EmptyState title="No active trace sessions" action={<Link to="/network/flows" className="buttonlike btn-secondary">View flows</Link>}>
              Trace sessions are created as FabricTraceSession resources and run by the network-intelligence operator.
            </EmptyState>
          ) : (
            tracesQ.data.slice(0, 6).map((trace) => {
              const phase = trace.status?.phase || 'Pending'
              return (
                <div key={trace.metadata.name} className="list-row">
                  <div className="grow">
                    <b>{trace.spec.targetService}</b>
                    <small>
                      Level: {formatCaptureLevel(trace.spec.captureLevel)} · Duration: {trace.spec.duration}
                      {trace.status?.flowsCaptured !== undefined && ` · ${trace.status.flowsCaptured} flows captured`}
                    </small>
                  </div>
                  <span className={`pill ${phaseTone(phase)}`}>{phase}</span>
                </div>
              )
            })
          )}
        </section>

        <section className="card span3">
          <p className="eyebrow">ANOMALIES{anomaliesQ.data ? ` · ${anomaliesQ.data.length} TOTAL` : ''}</p>
          <h2 className="card-title">Recent anomalies</h2>
          {anomaliesQ.isLoading ? (
            <Skeleton rows={3} />
          ) : anomaliesQ.isError && !anomaliesQ.data ? (
            <ErrorState title="Could not load anomalies." error={anomaliesQ.error} onRetry={() => anomaliesQ.refetch()} retrying={anomaliesQ.isFetching} />
          ) : !anomaliesQ.data || anomaliesQ.data.length === 0 ? (
            <EmptyState title="No anomalies detected" action={<Link to="/network/flows" className="buttonlike btn-secondary">View flows</Link>}>
              Anomalies are raised by the network-intelligence operator from observed traffic.
            </EmptyState>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Severity</th>
                    <th>Service</th>
                    <th>Type</th>
                    <th>Description</th>
                    <th>Detected</th>
                  </tr>
                </thead>
                <tbody>
                  {anomaliesQ.data.slice(0, 8).map((a) => (
                    <AnomalyRow key={a.metadata.name} anomaly={a} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
    </>
  )
}

function AnomalyRow({ anomaly }: { anomaly: NetworkAnomaly }) {
  const at = anomaly.spec.detectedAt
  return (
    <tr>
      <td>
        <SeverityBadge severity={anomaly.spec.severity} />
      </td>
      <td>{anomaly.spec.service}</td>
      <td className="muted">{anomaly.spec.type}</td>
      <td className="muted">{anomaly.spec.description || '—'}</td>
      <td className="faint" title={at ? formatDate(at) : undefined}>
        {at ? formatRelative(at) : '—'}
      </td>
    </tr>
  )
}

// --- Graph ---

const NODE_W = 140
const NODE_H = 56
const COL_GAP = 180
const ROW_GAP = 90
const PAD = 30
const EDGE_OFFSET = 7

const HEALTH_STROKE: Record<HealthKind, string> = {
  healthy: 'var(--accent-green)',
  warning: 'var(--accent-amber)',
  critical: 'var(--danger)',
  unknown: 'var(--text-faint)',
}
const HEALTH_LABEL: Record<HealthKind, string> = { healthy: 'Healthy', warning: 'Warning', critical: 'Critical', unknown: 'Unknown' }

const EDGE_STYLE: Record<VerdictKind, { color: string; marker: string; dash?: string; label: string }> = {
  good: { color: 'var(--accent-green)', marker: 'arrow-good', label: 'Forwarded / allowed' },
  drop: { color: 'var(--danger)', marker: 'arrow-drop', label: 'Dropped / denied' },
  error: { color: 'var(--accent-amber)', marker: 'arrow-error', label: 'Error' },
  unknown: { color: 'var(--text-faint)', marker: 'arrow-unknown', dash: '4 3', label: 'Unknown' },
}

function latencyLabel(latency?: string): string | null {
  const l = (latency ?? '').trim()
  return l && l !== '-' ? `p99 ${l}` : null
}

function ServiceGraphView({ nodes, edges }: { nodes: ServiceGraphNode[]; edges: ServiceGraphEdge[] }) {
  if (nodes.length === 0) {
    return (
      <EmptyState
        title="No service graph yet"
        action={
          <Link to="/network/flows" className="buttonlike btn-secondary">
            View flows
          </Link>
        }
      >
        Requires the network-intelligence operator to populate FabricServiceGraph.
      </EmptyState>
    )
  }

  const cols = Math.min(nodes.length, 4)
  const rows = Math.ceil(nodes.length / cols)
  const pos = new Map<string, { col: number; row: number }>()
  nodes.forEach((n, i) => pos.set(n.id, { col: i % cols, row: Math.floor(i / cols) }))

  const svgWidth = PAD * 2 + (cols - 1) * COL_GAP + NODE_W
  const svgHeight = PAD * 2 + (rows - 1) * ROW_GAP + NODE_H
  const center = (id: string) => {
    const p = pos.get(id)
    return p ? { x: PAD + p.col * COL_GAP + NODE_W / 2, y: PAD + p.row * ROW_GAP + NODE_H / 2 } : null
  }

  const nodeById = new Map(nodes.map((n) => [n.id, n]))
  const nodeName = (id: string) => nodeById.get(id)?.label ?? id
  const twoWay = twoWayEdges(edges)
  const counts = { warning: 0, critical: 0 }
  for (const n of nodes) {
    const h = healthKind(n.health)
    if (h === 'warning' || h === 'critical') counts[h] += 1
  }
  const summary = `Service dependency graph: ${nodes.length} service${nodes.length === 1 ? '' : 's'}, ${edges.length} connection${edges.length === 1 ? '' : 's'}; ${counts.warning} warning, ${counts.critical} critical. A table listing every service and connection follows.`

  return (
    <div>
      <div className="table-wrap">
        <svg width={svgWidth} height={svgHeight} style={{ minWidth: svgWidth }} role="img" aria-label={summary}>
          <defs>
            {Object.values(EDGE_STYLE).map((s) => (
              <marker key={s.marker} id={s.marker} markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                <polygon points="0 0, 8 3, 0 6" fill={s.color} />
              </marker>
            ))}
          </defs>

          {edges.map((edge, i) => {
            const from = center(edge.source)
            const to = center(edge.target)
            if (!from || !to || edge.source === edge.target) return null
            const style = EDGE_STYLE[verdictKind(edge.verdict)]
            const { start, end, mid } = edgeGeometry(from, to, NODE_W, NODE_H, twoWay[i] ? EDGE_OFFSET : 0)
            const lat = latencyLabel(edge.latency)
            return (
              <g key={`edge-${i}`}>
                <line x1={start.x} y1={start.y} x2={end.x} y2={end.y} stroke={style.color} strokeWidth="1.5" strokeDasharray={style.dash} markerEnd={`url(#${style.marker})`} />
                {lat && (
                  <text x={mid.x} y={mid.y - 5} textAnchor="middle" fontSize="9" fill="var(--text-tertiary)" fontFamily="monospace">
                    {lat}
                  </text>
                )}
              </g>
            )
          })}

          {nodes.map((node) => {
            const p = pos.get(node.id)!
            const x = PAD + p.col * COL_GAP
            const y = PAD + p.row * ROW_GAP
            const health = healthKind(node.health)
            return (
              <g key={node.id}>
                <rect x={x} y={y} width={NODE_W} height={NODE_H} rx="10" ry="10" fill="var(--bg-elevated-2)" stroke={HEALTH_STROKE[health]} strokeWidth="1.5" strokeDasharray={health === 'unknown' ? '4 3' : undefined} />
                <text x={x + NODE_W / 2} y={y + NODE_H / 2 - 4} textAnchor="middle" fontSize="12" fontWeight="600" fill={health === 'critical' ? 'var(--danger)' : 'var(--text-primary)'}>
                  {node.label}
                </text>
                <text x={x + NODE_W / 2} y={y + NODE_H / 2 + 12} textAnchor="middle" fontSize="9" fill="var(--text-tertiary)">
                  {node.flowCount} connections
                </text>
              </g>
            )
          })}
        </svg>
      </div>

      <div className="row faint" style={{ flexWrap: 'wrap', gap: 16, marginTop: 8 }}>
        <span>Node border:</span>
        {(Object.keys(HEALTH_STROKE) as HealthKind[]).map((h) => (
          <span key={h} style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <i aria-hidden="true" style={{ width: 14, height: 10, borderRadius: 3, border: `2px ${h === 'unknown' ? 'dashed' : 'solid'} ${HEALTH_STROKE[h]}` }} />
            {HEALTH_LABEL[h]}
          </span>
        ))}
      </div>
      <div className="row faint" style={{ flexWrap: 'wrap', gap: 16, marginTop: 4 }}>
        <span>Edge:</span>
        {(Object.keys(EDGE_STYLE) as VerdictKind[]).map((k) => (
          <span key={k} style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <i aria-hidden="true" style={{ width: 18, height: 0, borderTop: `2px ${EDGE_STYLE[k].dash ? 'dashed' : 'solid'} ${EDGE_STYLE[k].color}` }} />
            {EDGE_STYLE[k].label}
          </span>
        ))}
      </div>

      <VisuallyHidden>
        <table>
          <caption>Services and connections in the dependency graph</caption>
          <thead>
            <tr>
              <th>Service</th>
              <th>Health</th>
              <th>Connections</th>
            </tr>
          </thead>
          <tbody>
            {nodes.map((n) => (
              <tr key={n.id}>
                <td>{n.label}</td>
                <td>{HEALTH_LABEL[healthKind(n.health)]}</td>
                <td>{n.flowCount}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <table>
          <thead>
            <tr>
              <th>Connection</th>
              <th>Verdict</th>
              <th>Latency</th>
            </tr>
          </thead>
          <tbody>
            {edges.map((e, i) => (
              <tr key={i}>
                <td>
                  {nodeName(e.source)} to {nodeName(e.target)}
                </td>
                <td>{e.verdict || 'Unknown'}</td>
                <td>{latencyLabel(e.latency) ?? 'n/a'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </VisuallyHidden>
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  return <span className={`pill ${severityTone(severity)}`}>{severity}</span>
}
