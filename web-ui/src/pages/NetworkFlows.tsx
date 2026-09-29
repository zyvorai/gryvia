import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import type { NetworkFlow } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

const ITEMS_PER_PAGE = 20

export default function NetworkFlows() {
  const [serviceFilter, setServiceFilter] = useState('')
  const [namespaceFilter, setNamespaceFilter] = useState('')
  const [verdictFilter, setVerdictFilter] = useState<string>('all')
  const [protocolFilter, setProtocolFilter] = useState<string>('all')
  const [page, setPage] = useState(1)

  const { data: flows, isLoading, isError, dataUpdatedAt } = useQuery({
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
      <>
        <PageHero eyebrow="Network" title="Flows unavailable." tint="red" />
        <p className="warning" role="alert">
          Failed to load network flows. Please check your API connection.
        </p>
      </>
    )
  }

  const dropped = filtered.filter((f) => f.spec.verdict === 'DROP' || f.spec.verdict === 'DENIED').length
  const forwarded = filtered.filter((f) => f.spec.verdict === 'FORWARDED' || f.spec.verdict === 'ALLOW').length

  return (
    <>
      <PageHero eyebrow="Network" title="Every flow, live." lede="Real-time network flow monitoring" />

      <div className="grid">
        <div className="toolbar span3">
          <Link to="/network" className="buttonlike btn-secondary">
            Back to network
          </Link>
        </div>

        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={dropped > 0 ? `${dropped} flow${dropped === 1 ? '' : 's'} dropped or denied.` : 'No dropped or denied flows in view.'}
          tone={dropped > 0 ? 'warn' : undefined}
          figures={[
            { label: 'flows shown', value: filtered.length },
            { label: 'forwarded', value: forwarded },
            { label: 'dropped / denied', value: dropped, tone: countTone(dropped) },
            { label: 'total captured', value: flows?.length ?? 0 },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">FILTERS</p>
          <h2 className="card-title">Narrow the view</h2>
          <div className="filters">
            <label>
              Service
              <input
                type="text"
                value={serviceFilter}
                onChange={(e) => updateFilter(setServiceFilter)(e.target.value)}
                placeholder="Filter by service..."
              />
            </label>
            <label>
              Namespace
              <input
                type="text"
                value={namespaceFilter}
                onChange={(e) => updateFilter(setNamespaceFilter)(e.target.value)}
                placeholder="Filter by namespace..."
              />
            </label>
            <label>
              Verdict
              <select value={verdictFilter} onChange={(e) => updateFilter(setVerdictFilter)(e.target.value)}>
                <option value="all">All</option>
                <option value="FORWARDED">Forwarded</option>
                <option value="ALLOW">Allow</option>
                <option value="DROP">Drop</option>
                <option value="DENIED">Denied</option>
              </select>
            </label>
            <label>
              Protocol
              <select value={protocolFilter} onChange={(e) => updateFilter(setProtocolFilter)(e.target.value)}>
                <option value="all">All</option>
                <option value="TCP">TCP</option>
                <option value="UDP">UDP</option>
                <option value="HTTP">HTTP</option>
                <option value="gRPC">gRPC</option>
              </select>
            </label>
          </div>
        </section>

        <section className="card span3">
          <p className="eyebrow">PAGE {page} OF {totalPages}</p>
          <h2 className="card-title">{filtered.length} flows</h2>
          {paginated.length === 0 ? (
            <p className="empty-state">No flows matching current filters</p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Timestamp</th>
                    <th>Source</th>
                    <th>Destination</th>
                    <th>Protocol</th>
                    <th>Port</th>
                    <th>Bytes</th>
                    <th>Latency</th>
                    <th>Verdict</th>
                  </tr>
                </thead>
                <tbody>
                  {paginated.map((flow) => (
                    <FlowRow key={flow.metadata.name} flow={flow} />
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {totalPages > 1 && (
            <div className="toolbar">
              <button
                className="btn-secondary"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                Previous
              </button>
              {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                const pageNum = page <= 3 ? i + 1
                  : page >= totalPages - 2 ? totalPages - 4 + i
                  : page - 2 + i
                if (pageNum < 1 || pageNum > totalPages) return null
                return (
                  <button
                    key={pageNum}
                    className={pageNum === page ? 'primary' : 'btn-secondary'}
                    aria-current={pageNum === page ? 'page' : undefined}
                    onClick={() => setPage(pageNum)}
                  >
                    {pageNum}
                  </button>
                )
              })}
              <button
                className="btn-secondary"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page === totalPages}
              >
                Next
              </button>
            </div>
          )}
        </section>
      </div>
    </>
  )
}

function FlowRow({ flow }: { flow: NetworkFlow }) {
  const isForwarded = flow.spec.verdict === 'FORWARDED' || flow.spec.verdict === 'ALLOW'
  const isDrop = flow.spec.verdict === 'DROP' || flow.spec.verdict === 'DENIED'
  const verdictTone = isForwarded ? 'ok' : isDrop ? 'bad' : 'warn'

  return (
    <tr>
      <td className="mono faint">{new Date(flow.spec.timestamp).toLocaleTimeString()}</td>
      <td>{flow.spec.source}</td>
      <td>{flow.spec.destination}</td>
      <td>
        <span className="pill info">{flow.spec.protocol}</span>
      </td>
      <td className="mono muted">{flow.spec.port}</td>
      <td className="muted">{flow.spec.bytes}</td>
      <td className="muted">{flow.spec.latency}</td>
      <td>
        <span className={`pill ${verdictTone}`}>{flow.spec.verdict}</span>
        {flow.spec.policy && isDrop && <span className="faint"> ({flow.spec.policy})</span>}
      </td>
    </tr>
  )
}
