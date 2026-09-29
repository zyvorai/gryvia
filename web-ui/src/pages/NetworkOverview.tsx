import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { ServiceGraphNode, ServiceGraphEdge } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

export default function NetworkOverview() {
  const { data: insights, isLoading: insightsLoading, isError: insightsError, dataUpdatedAt: insightsUpdatedAt } = useQuery({
    queryKey: ['trafficInsights'],
    queryFn: api.getTrafficInsights,
    refetchInterval: 15000,
  })

  const { data: graph } = useQuery({
    queryKey: ['serviceGraph'],
    queryFn: api.getServiceGraph,
    refetchInterval: 30000,
  })

  const { data: anomalies } = useQuery({
    queryKey: ['networkAnomalies'],
    queryFn: api.getNetworkAnomalies,
    refetchInterval: 15000,
  })

  const { data: traces } = useQuery({
    queryKey: ['traceSessions'],
    queryFn: api.getTraceSessions,
    refetchInterval: 10000,
  })

  if (insightsLoading) return <LoadingSpinner />

  if (insightsError) {
    return (
      <>
        <PageHero eyebrow="Network" title="Network insights unavailable." tint="red" />
        <p className="warning" role="alert">
          Failed to load network insights. Please check your API connection.
        </p>
      </>
    )
  }

  const anomalyCount = insights?.anomaliesDetected ?? 0

  return (
    <>
      <PageHero eyebrow="Network" title="Network intelligence." lede="Service mesh observability and flow management" />

      <div className="grid">
        <div className="toolbar span3">
          <Link to="/network/policies" className="buttonlike primary">
            Policies
          </Link>
          <Link to="/network/flows" className="buttonlike btn-secondary">
            View Flows
          </Link>
        </div>

        <PagePulse
          updatedAt={insightsUpdatedAt}
          headline={anomalyCount > 0 ? `${anomalyCount} anomal${anomalyCount === 1 ? 'y' : 'ies'} detected across the mesh.` : 'No anomalies detected across the mesh.'}
          tone={anomalyCount > 0 ? 'warn' : undefined}
          figures={[
            { label: 'Active flows', value: insights?.activeFlows ?? 0 },
            { label: 'Policies enforced', value: insights?.policiesEnforced ?? 0 },
            { label: 'Anomalies detected', value: anomalyCount, tone: countTone(anomalyCount) },
            { label: 'Trace sessions', value: insights?.traceSessions ?? 0 },
          ]}
        />

        <section className="card span2">
          <p className="eyebrow">TOPOLOGY</p>
          <h2 className="card-title">Service dependency graph</h2>
          <ServiceGraphView nodes={graph?.nodes || []} edges={graph?.edges || []} />
        </section>

        <section className="card">
          <p className="eyebrow">TRACES</p>
          <h2 className="card-title">Active traces</h2>
          {(!traces || traces.length === 0) ? (
            <p className="empty-state">No active trace sessions</p>
          ) : (
            traces.slice(0, 6).map((trace) => {
              const isActive = trace.status?.phase === 'Running' || trace.status?.phase === 'Active'
              return (
                <div key={trace.metadata.name} className="list-row">
                  <div className="grow">
                    <b>{trace.spec.targetService}</b>
                    <small>
                      Level: {trace.spec.captureLevel} · Duration: {trace.spec.duration}
                      {trace.status?.flowsCaptured !== undefined && ` · ${trace.status.flowsCaptured} flows captured`}
                    </small>
                  </div>
                  <span className={`pill ${isActive ? 'ok' : 'info'}`}>{trace.status?.phase || 'Pending'}</span>
                </div>
              )
            })
          )}
        </section>

        <section className="card span2">
          <p className="eyebrow">ANOMALIES · {anomalies?.length || 0} TOTAL</p>
          <h2 className="card-title">Recent anomalies</h2>
          {(!anomalies || anomalies.length === 0) ? (
            <p className="empty-state">No anomalies detected</p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Severity</th>
                    <th>Service</th>
                    <th>Type</th>
                    <th>Time</th>
                  </tr>
                </thead>
                <tbody>
                  {anomalies.slice(0, 8).map((anomaly) => (
                    <tr key={anomaly.metadata.name}>
                      <td>
                        <SeverityBadge severity={anomaly.spec.severity} />
                      </td>
                      <td>{anomaly.spec.service}</td>
                      <td className="muted">{anomaly.spec.type}</td>
                      <td className="faint">{new Date(anomaly.spec.detectedAt).toLocaleTimeString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="card">
          <p className="eyebrow">SHORTCUTS</p>
          <h2 className="card-title">Quick actions</h2>
          <div className="stack">
            <Link to="/network/flows" className="card-link">
              View Network Flows ›
            </Link>
            <Link to="/network/policies" className="card-link">
              Manage Policies ›
            </Link>
          </div>
        </section>
      </div>
    </>
  )
}

// --- Sub-components ---

function ServiceGraphView({ nodes, edges }: { nodes: ServiceGraphNode[]; edges: ServiceGraphEdge[] }) {
  if (nodes.length === 0) {
    return (
      <p className="empty-state">
        No service graph data available. Deploy services with network policies to see the graph.
      </p>
    )
  }

  // Arrange nodes in a grid layout
  const cols = Math.min(nodes.length, 4)
  const rows = Math.ceil(nodes.length / cols)

  const nodePositions: Record<string, { col: number; row: number }> = {}
  nodes.forEach((node, i) => {
    nodePositions[node.id] = {
      col: i % cols,
      row: Math.floor(i / cols),
    }
  })

  const nodeWidth = 140
  const nodeHeight = 56
  const colGap = 180
  const rowGap = 90
  const paddingX = 30
  const paddingY = 30

  const svgWidth = paddingX * 2 + (cols - 1) * colGap + nodeWidth
  const svgHeight = paddingY * 2 + (rows - 1) * rowGap + nodeHeight

  const getNodeCenter = (id: string) => {
    const pos = nodePositions[id]
    if (!pos) return { x: 0, y: 0 }
    return {
      x: paddingX + pos.col * colGap + nodeWidth / 2,
      y: paddingY + pos.row * rowGap + nodeHeight / 2,
    }
  }

  const healthColors: Record<string, { fill: string; stroke: string; text: string }> = {
    healthy: { fill: 'var(--bg-elevated-2)', stroke: 'var(--accent-green)', text: 'var(--text-primary)' },
    warning: { fill: 'var(--bg-elevated-2)', stroke: 'var(--accent-amber)', text: 'var(--text-primary)' },
    critical: { fill: 'var(--bg-elevated-2)', stroke: 'var(--danger)', text: 'var(--danger)' },
  }

  return (
    <div className="table-wrap">
      <svg width={svgWidth} height={svgHeight} style={{ minWidth: svgWidth }}>
        <defs>
          <marker id="arrowhead" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="var(--text-tertiary)" />
          </marker>
          <marker id="arrowhead-green" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="var(--accent-green)" />
          </marker>
          <marker id="arrowhead-red" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="var(--danger)" />
          </marker>
        </defs>

        {/* Edges */}
        {edges.map((edge, i) => {
          const from = getNodeCenter(edge.source)
          const to = getNodeCenter(edge.target)
          if (from.x === 0 && from.y === 0) return null
          if (to.x === 0 && to.y === 0) return null

          const isForwarded = edge.verdict === 'FORWARDED' || edge.verdict === 'ALLOW'
          const edgeColor = isForwarded ? 'var(--accent-green)' : 'var(--danger)'
          const markerId = isForwarded ? 'arrowhead-green' : 'arrowhead-red'

          // Offset endpoints to edge of node box
          const dx = to.x - from.x
          const dy = to.y - from.y
          const dist = Math.sqrt(dx * dx + dy * dy) || 1
          const nx = dx / dist
          const ny = dy / dist

          const startX = from.x + nx * (nodeWidth / 2)
          const startY = from.y + ny * (nodeHeight / 2)
          const endX = to.x - nx * (nodeWidth / 2 + 8)
          const endY = to.y - ny * (nodeHeight / 2 + 4)

          const midX = (startX + endX) / 2
          const midY = (startY + endY) / 2

          return (
            <g key={`edge-${i}`}>
              <line
                x1={startX} y1={startY}
                x2={endX} y2={endY}
                stroke={edgeColor}
                strokeWidth="1.5"
                markerEnd={`url(#${markerId})`}
              />
              {edge.latency && edge.latency !== '-' && (
                <text
                  x={midX}
                  y={midY - 6}
                  textAnchor="middle"
                  fontSize="9"
                  fill="var(--text-tertiary)"
                  fontFamily="monospace"
                >
                  {edge.latency}
                </text>
              )}
            </g>
          )
        })}

        {/* Nodes */}
        {nodes.map((node) => {
          const pos = nodePositions[node.id]
          if (!pos) return null
          const x = paddingX + pos.col * colGap
          const y = paddingY + pos.row * rowGap
          const colors = healthColors[node.health] || healthColors.healthy

          return (
            <g key={node.id}>
              <rect
                x={x} y={y}
                width={nodeWidth} height={nodeHeight}
                rx="10" ry="10"
                fill={colors.fill}
                stroke={colors.stroke}
                strokeWidth="1.5"
              />
              <text
                x={x + nodeWidth / 2}
                y={y + nodeHeight / 2 - 4}
                textAnchor="middle"
                fontSize="12"
                fontWeight="600"
                fill={colors.text}
              >
                {node.label}
              </text>
              <text
                x={x + nodeWidth / 2}
                y={y + nodeHeight / 2 + 12}
                textAnchor="middle"
                fontSize="9"
                fill="var(--text-tertiary)"
              >
                {node.flowCount} flows
              </text>
            </g>
          )
        })}
      </svg>
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const tones: Record<string, string> = {
    critical: 'bad',
    high: 'bad',
    medium: 'warn',
    low: 'info',
  }
  return <span className={`pill ${tones[severity] ?? 'info'}`}>{severity}</span>
}
