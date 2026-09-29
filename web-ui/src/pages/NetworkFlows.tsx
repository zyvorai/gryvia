import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '@/lib/api'
import type { NetworkFlow } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { formatDate, formatRelative } from '@/lib/format'
import { errorMessage } from '@/lib/errors'
import { filterFlows, flowFilterOptions, pageWindow, verdictKind, verdictTone, type FlowFilters } from '@/lib/network'

const ITEMS_PER_PAGE = 20

export default function NetworkFlows() {
  useDocumentTitle('Network flows')
  const [params, setParams] = useSearchParams()
  const filters: FlowFilters = {
    service: params.get('service') ?? '',
    namespace: params.get('namespace') ?? '',
    verdict: params.get('verdict') ?? '',
    protocol: params.get('protocol') ?? '',
  }
  const requestedPage = Math.max(1, parseInt(params.get('page') ?? '1', 10) || 1)

  const setFilter = (key: keyof FlowFilters, value: string) => {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(key, value)
        else next.delete(key)
        next.delete('page')
        return next
      },
      { replace: true },
    )
  }
  const setPage = (n: number) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (n > 1) next.set('page', String(n))
        else next.delete('page')
        return next
      },
      { replace: true },
    )
  const clearFilters = () => setParams({}, { replace: true })

  const { data: flows, isLoading, isError, error, refetch, isFetching, dataUpdatedAt } = useQuery({
    queryKey: ['networkFlows'],
    queryFn: api.getNetworkFlows,
    refetchInterval: 5000,
  })

  const all = useMemo(() => flows ?? [], [flows])
  const options = useMemo(() => flowFilterOptions(all), [all])
  const filtered = useMemo(() => filterFlows(all, filters), [all, filters.service, filters.namespace, filters.verdict, filters.protocol]) // eslint-disable-line react-hooks/exhaustive-deps

  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE))
  const page = Math.min(requestedPage, totalPages)
  const paginated = filtered.slice((page - 1) * ITEMS_PER_PAGE, page * ITEMS_PER_PAGE)
  const hasThroughput = filtered.some((f) => f.spec.bytes !== undefined && f.spec.bytes !== null && String(f.spec.bytes).trim() !== '')
  const filtersActive = Object.values(filters).some(Boolean)

  const withCurrent = (list: string[], current: string) => (current && !list.some((v) => v.toLowerCase() === current.toLowerCase()) ? [...list, current] : list)

  const dropped = filtered.filter((f) => verdictKind(f.spec.verdict) === 'drop').length
  const errored = filtered.filter((f) => verdictKind(f.spec.verdict) === 'error').length
  const forwarded = filtered.filter((f) => verdictKind(f.spec.verdict) === 'good').length
  const problems = dropped + errored
  const loaded = flows !== undefined

  return (
    <>
      <PageHero eyebrow="Network" title="Observed connections." lede="Connections observed between services, from the service graph" />

      <div className="grid">
        <div className="toolbar span3">
          <Link to="/network" className="buttonlike btn-secondary">
            Back to network
          </Link>
        </div>

        {isLoading ? (
          <section className="card span3" aria-label="Live summary">
            <Skeleton rows={2} />
          </section>
        ) : isError && !loaded ? (
          <div className="span3">
            <ErrorState title="Could not load network flows." error={error} onRetry={() => refetch()} retrying={isFetching} />
          </div>
        ) : (
          <>
            <PagePulse
              updatedAt={dataUpdatedAt}
              error={isError ? errorMessage(error) : undefined}
              headline={
                problems > 0
                  ? `${problems} connection${problems === 1 ? '' : 's'} dropped, denied or errored.`
                  : 'No dropped, denied or errored connections in view.'
              }
              tone={problems > 0 ? 'warn' : undefined}
              figures={[
                { label: 'connections shown', value: filtered.length },
                { label: 'forwarded', value: forwarded },
                { label: 'dropped / denied / error', value: problems, tone: problems > 0 ? 'warn' : undefined },
                { label: 'total observed', value: all.length },
              ]}
            />

            <section className="card span3">
              <p className="eyebrow">FILTERS</p>
              <h2 className="card-title">Narrow the view</h2>
              <div className="filters">
                <label>
                  Service
                  <input type="text" value={filters.service} onChange={(e) => setFilter('service', e.target.value)} placeholder="Filter by service..." />
                </label>
                <label>
                  Namespace
                  <select value={filters.namespace} onChange={(e) => setFilter('namespace', e.target.value)}>
                    <option value="">All</option>
                    {withCurrent(options.namespaces, filters.namespace).map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Verdict
                  <select value={filters.verdict} onChange={(e) => setFilter('verdict', e.target.value)}>
                    <option value="">All</option>
                    {withCurrent(options.verdicts, filters.verdict).map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Protocol
                  <select value={filters.protocol} onChange={(e) => setFilter('protocol', e.target.value)}>
                    <option value="">All</option>
                    {withCurrent(options.protocols, filters.protocol).map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </label>
                {filtersActive && (
                  <button type="button" className="btn-secondary" onClick={clearFilters}>
                    Clear filters
                  </button>
                )}
              </div>
            </section>

            <section className="card span3">
              <p className="eyebrow">
                PAGE {page} OF {totalPages}
              </p>
              <h2 className="card-title">
                {filtered.length} connection{filtered.length === 1 ? '' : 's'}
              </h2>
              {all.length === 0 ? (
                <EmptyState title="No connections observed yet" action={<Link to="/network" className="buttonlike btn-secondary">Back to network</Link>}>
                  Connections come from the FabricServiceGraph that the network-intelligence operator builds from eBPF flow data. Nothing has been reported yet.
                </EmptyState>
              ) : filtered.length === 0 ? (
                <EmptyState title="No connections match these filters" action={<button type="button" className="btn-secondary" onClick={clearFilters}>Clear filters</button>}>
                  {all.length} connection{all.length === 1 ? ' is' : 's are'} hidden by the current filters.
                </EmptyState>
              ) : (
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Graph updated</th>
                        <th>Source</th>
                        <th>Destination</th>
                        <th>Protocol</th>
                        <th className="num">Port</th>
                        {hasThroughput && <th className="num">Throughput</th>}
                        <th className="num">Latency</th>
                        <th>Verdict</th>
                      </tr>
                    </thead>
                    <tbody>
                      {paginated.map((flow) => (
                        <FlowRow key={flow.metadata.name} flow={flow} showThroughput={hasThroughput} />
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {totalPages > 1 && (
                <nav aria-label="Pagination" className="toolbar">
                  <button type="button" className="btn-secondary" onClick={() => setPage(page - 1)} disabled={page === 1}>
                    Previous
                  </button>
                  {pageWindow(page, totalPages).map((n) => (
                    <button
                      key={n}
                      type="button"
                      className={n === page ? 'primary' : 'btn-secondary'}
                      aria-label={`Page ${n}`}
                      aria-current={n === page ? 'page' : undefined}
                      onClick={() => setPage(n)}
                    >
                      {n}
                    </button>
                  ))}
                  <button type="button" className="btn-secondary" onClick={() => setPage(page + 1)} disabled={page === totalPages}>
                    Next
                  </button>
                </nav>
              )}
            </section>
          </>
        )}
      </div>
    </>
  )
}

function FlowRow({ flow, showThroughput }: { flow: NetworkFlow; showThroughput: boolean }) {
  const ts = flow.spec.timestamp
  const verdict = flow.spec.verdict as string
  return (
    <tr>
      <td className="faint" title={ts ? formatDate(ts) : undefined}>
        {ts ? `${formatDate(ts)} · ${formatRelative(ts)}` : '—'}
      </td>
      <td>{flow.spec.source}</td>
      <td>{flow.spec.destination}</td>
      <td>
        <span className="pill info">{flow.spec.protocol}</span>
      </td>
      <td className="mono muted num">{flow.spec.port}</td>
      {showThroughput && <td className="muted num">{flow.spec.bytes || '—'}</td>}
      <td className="muted num">{flow.spec.latency && flow.spec.latency !== '-' ? flow.spec.latency : '—'}</td>
      <td>
        <span className={`pill ${verdictTone(verdict)}`}>{verdict || 'Unknown'}</span>
      </td>
    </tr>
  )
}
