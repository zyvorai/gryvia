import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { api } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import Progress from '@/components/Progress'
import DataTable, { type Column } from '@/components/DataTable'
import { useTableState } from '@/hooks/useTableState'
import { applySearch, type SortAccessor } from '@/lib/tableState'
import VisuallyHidden from '@/components/VisuallyHidden'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { formatBytes, formatMoney, formatPercent } from '@/lib/format'
import { errorMessage } from '@/lib/errors'
import { periodLabel, summarizeCosts, type CostGroup } from '@/lib/networkCost'

const SORTERS: Record<string, SortAccessor<CostGroup>> = {
  name: (g) => g.name.toLowerCase(),
  cost: (g) => g.cost,
  bytes: (g) => g.bytes,
}
const searchText = (g: CostGroup) => g.name

export default function NetworkCost() {
  useDocumentTitle('Network cost')
  const [params, setParams] = useSearchParams()
  const selected = params.get('period') ?? ''
  const setSelected = (p: string) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('period', p)
        next.delete('page')
        return next
      },
      { replace: true },
    )
  const { data: costData, isLoading, isError, error, refetch, isFetching, dataUpdatedAt } = useQuery({
    queryKey: ['networkCosts'],
    queryFn: api.getNetworkCosts,
    refetchInterval: 60000,
  })

  const allReports = useMemo(() => costData?.reports ?? [], [costData])
  const summary = useMemo(() => summarizeCosts(allReports, selected), [allReports, selected])

  return (
    <>
      <PageHero eyebrow="Network" title="What your traffic costs." lede="Cross-zone and egress network cost tracking, as reported per period" />

      <div className="grid">
        {isLoading ? (
          <section className="card span3" aria-label="Live summary">
            <Skeleton rows={3} />
          </section>
        ) : isError && !costData ? (
          <div className="span3">
            <ErrorState title="Could not load network cost data." error={error} onRetry={() => refetch()} retrying={isFetching} />
          </div>
        ) : allReports.length === 0 ? (
          <div className="span3">
            <EmptyState title="No network cost reports yet">
              Cost reports are produced by the network operator from FabricNetworkCost resources. Create a FabricNetworkCost CR for a namespace or team to start tracking; the first report appears after one reporting interval.
            </EmptyState>
          </div>
        ) : (
          <CostBody
            summary={summary}
            costPerGB={costData?.costPerGB}
            periodChoices={summary.periods}
            onSelect={setSelected}
            updatedAt={dataUpdatedAt}
            stale={isError ? errorMessage(error) : undefined}
          />
        )}
      </div>
    </>
  )
}

function CostBody({ summary, costPerGB, periodChoices, onSelect, updatedAt, stale }: {
  summary: ReturnType<typeof summarizeCosts>
  costPerGB?: { sameZone: number; crossZone: number; internetEgress: number }
  periodChoices: string[]
  onSelect: (p: string) => void
  updatedAt: number
  stale?: string
}) {
  const { period, reports, totalCost, sameZone, crossZone, external, teams, namespaces, namespacesAll, trend } = summary
  const table = useTableState<CostGroup>({ rows: teams, searchText, sortAccessors: SORTERS, pageSize: 10 })
  // One search box (the teams table's) also narrows the namespaces card; unsearched it shows the top 8.
  const shownNamespaces = table.search.trim() ? applySearch(namespacesAll, table.search, searchText) : namespaces
  const teamColumns: Column<CostGroup>[] = [
    { key: 'name', header: 'Team', sortable: true, render: (t) => t.name },
    { key: 'cost', header: 'Cost (USD)', sortable: true, numeric: true, render: (t) => <span className="mono">{formatMoney(t.cost)}</span> },
    { key: 'bytes', header: 'Traffic', sortable: true, numeric: true, render: (t) => <span className="muted">{formatBytes(t.bytes)}</span> },
    { key: 'share', header: '% of total', numeric: true, render: (t) => <span className="faint">{totalCost > 0 ? formatPercent((t.cost / totalCost) * 100) : '—'}</span> },
  ]
  const totalBytes = sameZone + crossZone + external
  const label = periodLabel(period)

  return (
    <>
      <div className="toolbar span3">
        <label className="field" style={{ flexDirection: 'row', alignItems: 'center' }}>
          <span>Period</span>
          <select value={period} onChange={(e) => onSelect(e.target.value)} aria-label="Reporting period">
            {periodChoices.map((p, i) => (
              <option key={p} value={p}>
                {periodLabel(p)}
                {i === 0 ? ' (latest)' : ''}
              </option>
            ))}
          </select>
        </label>
        <span className="faint">
          {reports.length} report{reports.length === 1 ? '' : 's'} in this period
        </span>
      </div>

      <PagePulse
        updatedAt={updatedAt}
        error={stale}
        headline={`Network spend for ${label} is ${formatMoney(totalCost)} across ${reports.length} report${reports.length === 1 ? '' : 's'}.`}
        figures={[
          { label: `spend · ${label}`, value: formatMoney(totalCost) },
          { label: 'same-zone traffic', value: formatBytes(sameZone) },
          { label: 'cross-zone traffic', value: formatBytes(crossZone) },
          { label: 'internet egress', value: formatBytes(external) },
        ]}
      />

      <section className="card span2">
        <p className="eyebrow">ZONES · {label.toUpperCase()}</p>
        <h2 className="card-title">Traffic by zone type</h2>
        <div className="stack">
          {[
            { label: 'Same zone', bytes: sameZone, rate: costPerGB?.sameZone },
            { label: 'Cross zone', bytes: crossZone, rate: costPerGB?.crossZone },
            { label: 'Internet egress', bytes: external, rate: costPerGB?.internetEgress },
          ].map((z) => (
            <div key={z.label}>
              <div className="list-row">
                <span className="grow">{z.label}</span>
                <span className="faint num">{formatBytes(z.bytes)}</span>
                <span className="mono num">{totalBytes > 0 ? formatPercent((z.bytes / totalBytes) * 100) : '—'}</span>
              </div>
              <Progress value={z.bytes} max={totalBytes} label={`${z.label} share of traffic`} />
              {z.rate !== undefined && z.rate !== null && <small className="faint">Configured rate {formatMoney(z.rate)}/GB</small>}
            </div>
          ))}
          <small className="faint">Cost is reported per team and namespace (totals above), not per zone type.</small>
        </div>
      </section>

      <section className="card">
        <p className="eyebrow">TREND</p>
        <h2 className="card-title">Cost trend</h2>
        <CostTrendChart data={trend} />
      </section>

      <section className="card span2">
        <p className="eyebrow">TEAMS · {label.toUpperCase()}</p>
        <h2 className="card-title">Cost by team</h2>
        <DataTable
          caption={`Network cost by team, ${label}`}
          searchLabel="Search teams and namespaces"
          columns={teamColumns}
          state={table}
          rowKey={(t) => t.name}
          empty={<EmptyState title="No team costs in this period" />}
        />
      </section>

      <section className="card">
        <p className="eyebrow">NAMESPACES</p>
        <h2 className="card-title">Top cost contributors</h2>
        <div className="stack">
          {shownNamespaces.length === 0 && <p className="faint">No namespace matches the search.</p>}
          {shownNamespaces.map((n) => (
            <div key={n.name}>
              <div className="list-row">
                <span className="grow">{n.name}</span>
                <span className="mono num">{formatMoney(n.cost)}</span>
              </div>
              <Progress value={n.cost} max={totalCost} label={`${n.name} share of spend`} />
            </div>
          ))}
        </div>
      </section>
    </>
  )
}

function CostTrendChart({ data }: { data: Array<{ period: string; cost: number }> }) {
  if (data.length < 2) {
    return (
      <EmptyState title="Not enough history for a trend">
        A trend needs at least two reporting periods. It appears after the next reporting interval.
      </EmptyState>
    )
  }
  const maxCost = Math.max(...data.map((d) => d.cost), 0)
  const chartHeight = 120
  const summary = `Network cost per period, ${data[0].period} to ${data[data.length - 1].period}: from ${formatMoney(data[0].cost)} to ${formatMoney(data[data.length - 1].cost)}, peak ${formatMoney(maxCost)}.`

  return (
    <div>
      <div role="img" aria-label={summary} style={{ display: 'grid', gridAutoFlow: 'column', gridAutoColumns: '1fr', alignItems: 'end', gap: 4, height: chartHeight }}>
        {data.map((d) => (
          <div
            key={d.period}
            style={{ height: maxCost > 0 ? `${(d.cost / maxCost) * chartHeight}px` : 0, background: 'var(--apple-blue)', borderRadius: '3px 3px 0 0' }}
            title={`${d.period}: ${formatMoney(d.cost)}`}
          />
        ))}
      </div>
      <div className="list-row faint">
        <small className="grow">{data[0].period}</small>
        <small>{data[data.length - 1].period}</small>
      </div>
      <VisuallyHidden>
        <table>
          <caption>Network cost per period</caption>
          <thead>
            <tr>
              <th>Period</th>
              <th>Cost (USD)</th>
            </tr>
          </thead>
          <tbody>
            {data.map((d) => (
              <tr key={d.period}>
                <td>{d.period}</td>
                <td>{formatMoney(d.cost)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </VisuallyHidden>
    </div>
  )
}

