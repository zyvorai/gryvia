import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import {
  Activity, ArrowLeft, Filter, ChevronLeft, ChevronRight, Clock,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { NetworkFlow } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'

const ITEMS_PER_PAGE = 20

export default function NetworkFlows() {
  const [serviceFilter, setServiceFilter] = useState('')
  const [namespaceFilter, setNamespaceFilter] = useState('')
  const [verdictFilter, setVerdictFilter] = useState<string>('all')
  const [protocolFilter, setProtocolFilter] = useState<string>('all')
  const [page, setPage] = useState(1)

  const { data: flows, isLoading, isError } = useQuery({
    queryKey: ['networkFlows'],
    queryFn: api.getNetworkFlows,
    refetchInterval: 5000,
  })

  const filtered = useMemo(() => {
    if (!flows) return []
    return flows.filter((f) => {
      if (serviceFilter && !f.spec.source.includes(serviceFilter) && !f.spec.destination.includes(serviceFilter)) return false
      if (namespaceFilter && f.metadata.namespace && !f.metadata.namespace.includes(namespaceFilter)) return false
      if (verdictFilter !== 'all' && f.spec.verdict !== verdictFilter) return false
      if (protocolFilter !== 'all' && f.spec.protocol !== protocolFilter) return false
      return true
    })
  }, [flows, serviceFilter, namespaceFilter, verdictFilter, protocolFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE))
  const paginated = filtered.slice((page - 1) * ITEMS_PER_PAGE, page * ITEMS_PER_PAGE)

  // Reset to page 1 when filters change
  const updateFilter = <T,>(setter: (v: T) => void) => (v: T) => {
    setter(v)
    setPage(1)
  }

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load network flows. Please check your API connection.
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link to="/network" className="p-1.5 rounded-lg transition-colors hover:bg-[#1a2332]/60 text-[#5a7a9e] hover:text-[#c0cce0]">
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <div>
            <h2 className="text-2xl font-bold text-gradient-copper">Network Flows</h2>
            <p className="text-sm text-[#5a7a9e] mt-1">Real-time network flow monitoring</p>
          </div>
        </div>
        <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg"
          style={{
            background: 'rgba(10,14,20,0.5)',
            border: '1px solid rgba(34,197,94,0.12)',
          }}
        >
          <Activity className="h-3.5 w-3.5 text-emerald-400 animate-pulse-dot" />
          <span className="text-xs text-[#5a7a9e]">Live</span>
        </div>
      </div>

      {/* Filters */}
      <div className="rounded-xl p-4" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="flex items-center gap-2 mb-3">
          <Filter className="h-3.5 w-3.5 text-[#5a7a9e]" />
          <span className="text-xs font-medium text-[#8ba4c0]">Filters</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Service</label>
            <input
              type="text"
              value={serviceFilter}
              onChange={(e) => updateFilter(setServiceFilter)(e.target.value)}
              placeholder="Filter by service..."
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] placeholder-[#344e6a] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.06)',
              }}
            />
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Namespace</label>
            <input
              type="text"
              value={namespaceFilter}
              onChange={(e) => updateFilter(setNamespaceFilter)(e.target.value)}
              placeholder="Filter by namespace..."
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] placeholder-[#344e6a] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.06)',
              }}
            />
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Verdict</label>
            <select
              value={verdictFilter}
              onChange={(e) => updateFilter(setVerdictFilter)(e.target.value)}
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.06)',
              }}
            >
              <option value="all">All</option>
              <option value="FORWARDED">Forwarded</option>
              <option value="ALLOW">Allow</option>
              <option value="DROP">Drop</option>
              <option value="DENIED">Denied</option>
            </select>
          </div>
          <div>
            <label className="text-[10px] text-[#5a7a9e] uppercase mb-1 block">Protocol</label>
            <select
              value={protocolFilter}
              onChange={(e) => updateFilter(setProtocolFilter)(e.target.value)}
              className="w-full text-xs rounded-lg px-3 py-2 text-[#c0cce0] outline-none focus:ring-1 focus:ring-[#d4764e]/30"
              style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.06)',
              }}
            >
              <option value="all">All</option>
              <option value="TCP">TCP</option>
              <option value="UDP">UDP</option>
              <option value="HTTP">HTTP</option>
              <option value="gRPC">gRPC</option>
            </select>
          </div>
        </div>
      </div>

      {/* Flows Table */}
      <div className="rounded-xl overflow-hidden" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-3 flex items-center justify-between" style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
          <span className="text-xs text-[#5a7a9e]">{filtered.length} flows</span>
          <span className="text-xs text-[#5a7a9e]">Page {page} of {totalPages}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">TIMESTAMP</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">SOURCE</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">DESTINATION</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">PROTOCOL</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">PORT</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">BYTES</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">LATENCY</th>
                <th className="text-left py-3 px-4 text-[#5a7a9e] font-medium">VERDICT</th>
              </tr>
            </thead>
            <tbody>
              {paginated.length === 0 ? (
                <tr>
                  <td colSpan={8} className="text-center py-8 text-sm text-[#344e6a]">No flows matching current filters</td>
                </tr>
              ) : (
                paginated.map((flow) => (
                  <FlowRow key={flow.metadata.name} flow={flow} />
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="px-5 py-3 flex items-center justify-between" style={{ borderTop: '1px solid rgba(192,204,224,0.06)' }}>
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="flex items-center gap-1 text-xs text-[#8ba4c0] hover:text-[#c0cce0] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              <ChevronLeft className="h-3.5 w-3.5" />
              Previous
            </button>
            <div className="flex items-center gap-1">
              {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                const pageNum = page <= 3 ? i + 1
                  : page >= totalPages - 2 ? totalPages - 4 + i
                  : page - 2 + i
                if (pageNum < 1 || pageNum > totalPages) return null
                return (
                  <button
                    key={pageNum}
                    onClick={() => setPage(pageNum)}
                    className={`w-7 h-7 rounded-lg text-xs font-medium transition-colors ${
                      pageNum === page
                        ? 'text-[#e8a87c]'
                        : 'text-[#5a7a9e] hover:text-[#c0cce0]'
                    }`}
                    style={pageNum === page ? {
                      background: 'rgba(212,118,78,0.12)',
                    } : undefined}
                  >
                    {pageNum}
                  </button>
                )
              })}
            </div>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page === totalPages}
              className="flex items-center gap-1 text-xs text-[#8ba4c0] hover:text-[#c0cce0] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              Next
              <ChevronRight className="h-3.5 w-3.5" />
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

function FlowRow({ flow }: { flow: NetworkFlow }) {
  const isForwarded = flow.spec.verdict === 'FORWARDED' || flow.spec.verdict === 'ALLOW'
  const isDrop = flow.spec.verdict === 'DROP' || flow.spec.verdict === 'DENIED'

  const verdictStyle = isForwarded
    ? { background: 'rgba(34,197,94,0.08)', color: '#4ade80' }
    : isDrop
    ? { background: 'rgba(239,68,68,0.08)', color: '#f87171' }
    : { background: 'rgba(251,191,36,0.08)', color: '#fbbf24' }

  return (
    <tr className="table-row-hover" style={{ borderBottom: '1px solid rgba(192,204,224,0.03)' }}>
      <td className="py-2.5 px-4 text-[#5a7a9e] font-mono">
        <div className="flex items-center gap-1">
          <Clock className="h-3 w-3" />
          {new Date(flow.spec.timestamp).toLocaleTimeString()}
        </div>
      </td>
      <td className="py-2.5 px-4 text-[#c0cce0] font-medium">{flow.spec.source}</td>
      <td className="py-2.5 px-4 text-[#c0cce0] font-medium">{flow.spec.destination}</td>
      <td className="py-2.5 px-4">
        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
          background: 'rgba(95,168,211,0.08)',
          color: '#7ecbf5',
        }}>
          {flow.spec.protocol}
        </span>
      </td>
      <td className="py-2.5 px-4 text-[#8ba4c0] font-mono">{flow.spec.port}</td>
      <td className="py-2.5 px-4 text-[#8ba4c0]">{flow.spec.bytes}</td>
      <td className="py-2.5 px-4 text-[#8ba4c0]">{flow.spec.latency}</td>
      <td className="py-2.5 px-4">
        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={verdictStyle}>
          {flow.spec.verdict}
        </span>
        {flow.spec.policy && isDrop && (
          <span className="text-[10px] text-[#5a7a9e] ml-1">({flow.spec.policy})</span>
        )}
      </td>
    </tr>
  )
}
