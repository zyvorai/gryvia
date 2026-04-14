import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Activity, Network, Shield, AlertTriangle, Radio,
  Clock, Eye,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { ServiceGraphNode, ServiceGraphEdge } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function NetworkOverview() {
  const { data: insights, isLoading: insightsLoading, isError: insightsError } = useQuery({
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
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load network insights. Please check your API connection.
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Network Intelligence</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">Service mesh observability and flow management</p>
        </div>
        <div className="flex items-center gap-3">
          <Link to="/network/flows" className="btn-chrome text-xs px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5">
            <Eye className="h-3.5 w-3.5" />
            View Flows
          </Link>
          <Link to="/network/policies" className="btn-copper text-xs px-3 py-1.5 rounded-lg inline-flex items-center gap-1.5">
            <Shield className="h-3.5 w-3.5" />
            Policies
          </Link>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={Activity}
          title="Active Flows"
          value={insights?.activeFlows ?? 0}
          color="blue"
        />
        <StatCard
          icon={Shield}
          title="Policies Enforced"
          value={insights?.policiesEnforced ?? 0}
          color="green"
        />
        <StatCard
          icon={AlertTriangle}
          title="Anomalies Detected"
          value={insights?.anomaliesDetected ?? 0}
          color={((insights?.anomaliesDetected ?? 0) > 0) ? 'orange' : 'cyan'}
        />
        <StatCard
          icon={Radio}
          title="Trace Sessions"
          value={insights?.traceSessions ?? 0}
          color="purple"
        />
      </div>

      {/* Main content grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Service Graph */}
        <div className="lg:col-span-2 space-y-6">
          {/* Service Graph */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Network className="h-4 w-4 text-[#7ecbf5]" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Service Dependency Graph</h3>
            </div>
            <ServiceGraphView nodes={graph?.nodes || []} edges={graph?.edges || []} />
          </div>

          {/* Recent Anomalies */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-[#fbbf24]" />
                <h3 className="text-sm font-semibold text-[#e8ecf1]">Recent Anomalies</h3>
              </div>
              <span className="text-xs text-[#5a7a9e]">{anomalies?.length || 0} total</span>
            </div>
            {(!anomalies || anomalies.length === 0) ? (
              <div className="text-center py-6 text-sm text-[#344e6a]">No anomalies detected</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">SEVERITY</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">SERVICE</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">TYPE</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">TIME</th>
                    </tr>
                  </thead>
                  <tbody>
                    {anomalies.slice(0, 8).map((anomaly) => (
                      <tr key={anomaly.metadata.name} className="table-row-hover" style={{ borderBottom: '1px solid rgba(192,204,224,0.03)' }}>
                        <td className="py-2 px-2">
                          <SeverityBadge severity={anomaly.spec.severity} />
                        </td>
                        <td className="py-2 px-2 text-[#c0cce0]">{anomaly.spec.service}</td>
                        <td className="py-2 px-2 text-[#8ba4c0]">{anomaly.spec.type}</td>
                        <td className="py-2 px-2 text-[#5a7a9e]">
                          <div className="flex items-center gap-1">
                            <Clock className="h-3 w-3" />
                            {new Date(anomaly.spec.detectedAt).toLocaleTimeString()}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>

        {/* Right sidebar */}
        <div className="space-y-6">
          {/* Active Trace Sessions */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Radio className="h-4 w-4 text-purple-400" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Active Traces</h3>
            </div>
            {(!traces || traces.length === 0) ? (
              <div className="text-center py-6 text-sm text-[#344e6a]">No active trace sessions</div>
            ) : (
              <div className="space-y-2">
                {traces.slice(0, 6).map((trace) => {
                  const isActive = trace.status?.phase === 'Running' || trace.status?.phase === 'Active'
                  return (
                    <div key={trace.metadata.name} className="p-2.5 rounded-lg" style={{
                      background: 'rgba(10,14,20,0.5)',
                      border: '1px solid rgba(192,204,224,0.04)',
                    }}>
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-xs font-medium text-[#c0cce0] truncate">{trace.spec.targetService}</span>
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
                          background: isActive ? 'rgba(34,197,94,0.08)' : 'rgba(95,168,211,0.08)',
                          color: isActive ? '#4ade80' : '#7ecbf5',
                        }}>
                          {trace.status?.phase || 'Pending'}
                        </span>
                      </div>
                      <div className="flex items-center gap-3 text-[10px] text-[#5a7a9e]">
                        <span>Level: {trace.spec.captureLevel}</span>
                        <span>Duration: {trace.spec.duration}</span>
                      </div>
                      {trace.status?.flowsCaptured !== undefined && (
                        <div className="text-[10px] text-[#5a7a9e] mt-1">
                          {trace.status.flowsCaptured} flows captured
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Quick Links */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Activity className="h-4 w-4 text-emerald-400" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Quick Actions</h3>
            </div>
            <div className="space-y-2">
              <Link to="/network/flows" className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-all duration-200 text-[#e8a87c] hover:text-[#f0c4a0]"
                style={{
                  background: 'rgba(212,118,78,0.08)',
                  border: '1px solid rgba(212,118,78,0.15)',
                }}
              >
                <Eye className="h-4 w-4" />
                View Network Flows
              </Link>
              <Link to="/network/policies" className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-[#8ba4c0] hover:text-[#c0cce0] transition-all duration-200"
                style={{
                  background: 'rgba(10,14,20,0.4)',
                  border: '1px solid rgba(192,204,224,0.04)',
                }}
              >
                <Shield className="h-4 w-4 text-emerald-400" />
                Manage Policies
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// --- Sub-components ---

function ServiceGraphView({ nodes, edges }: { nodes: ServiceGraphNode[]; edges: ServiceGraphEdge[] }) {
  if (nodes.length === 0) {
    return (
      <div className="text-center py-12 text-sm text-[#344e6a]">
        No service graph data available. Deploy services with network policies to see the graph.
      </div>
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

  const healthColors: Record<string, { fill: string; stroke: string; text: string; glow: string }> = {
    healthy: { fill: '#0e2a1a', stroke: 'rgba(34,197,94,0.35)', text: '#4ade80', glow: 'rgba(34,197,94,0.15)' },
    warning: { fill: '#2a1a0e', stroke: 'rgba(251,191,36,0.35)', text: '#fbbf24', glow: 'rgba(251,191,36,0.15)' },
    critical: { fill: '#2a0e0e', stroke: 'rgba(239,68,68,0.35)', text: '#f87171', glow: 'rgba(239,68,68,0.15)' },
  }

  return (
    <div className="overflow-x-auto">
      <svg width={svgWidth} height={svgHeight} className="mx-auto" style={{ minWidth: svgWidth }}>
        <defs>
          <marker id="arrowhead" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#5a7a9e" />
          </marker>
          <marker id="arrowhead-green" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#4ade80" />
          </marker>
          <marker id="arrowhead-red" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" fill="#f87171" />
          </marker>
        </defs>

        {/* Edges */}
        {edges.map((edge, i) => {
          const from = getNodeCenter(edge.source)
          const to = getNodeCenter(edge.target)
          if (from.x === 0 && from.y === 0) return null
          if (to.x === 0 && to.y === 0) return null

          const isForwarded = edge.verdict === 'FORWARDED' || edge.verdict === 'ALLOW'
          const edgeColor = isForwarded ? 'rgba(34,197,94,0.4)' : 'rgba(239,68,68,0.4)'
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
                  fill="#5a7a9e"
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
                strokeWidth="1"
                filter={`drop-shadow(0 0 6px ${colors.glow})`}
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
                fill="#5a7a9e"
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
  const styles: Record<string, { bg: string; color: string }> = {
    critical: { bg: 'rgba(239,68,68,0.1)', color: '#f87171' },
    high: { bg: 'rgba(239,68,68,0.08)', color: '#fb923c' },
    medium: { bg: 'rgba(251,191,36,0.08)', color: '#fbbf24' },
    low: { bg: 'rgba(6,182,212,0.08)', color: '#22d3ee' },
  }
  const s = styles[severity] || styles.low
  return (
    <span className="text-[10px] font-medium px-2 py-0.5 rounded-full uppercase" style={{
      background: s.bg,
      color: s.color,
    }}>
      {severity}
    </span>
  )
}
