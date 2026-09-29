import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'

export default function NetworkCost() {
  const { data: costData, isLoading, isError, dataUpdatedAt } = useQuery({
    queryKey: ['networkCosts'],
    queryFn: api.getNetworkCosts,
    refetchInterval: 60000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Network" title="Cost data unavailable." tint="red" />
        <p className="warning" role="alert">
          Failed to load network cost data. Please check your API connection.
        </p>
      </>
    )
  }

  const reports = costData?.reports ?? []
  const totalCost = reports.reduce((sum, r) => sum + r.totalCostUSD, 0)
  const sameZoneTotal = reports.reduce((sum, r) => sum + r.sameZoneBytes, 0)
  const crossZoneTotal = reports.reduce((sum, r) => sum + r.crossZoneBytes, 0)
  const externalTotal = reports.reduce((sum, r) => sum + r.externalBytes, 0)

  // Group reports by team
  const byTeam: Record<string, { team: string; cost: number; bytes: number }> = {}
  for (const r of reports) {
    const team = r.team || 'unassigned'
    if (!byTeam[team]) {
      byTeam[team] = { team, cost: 0, bytes: 0 }
    }
    byTeam[team].cost += r.totalCostUSD
    byTeam[team].bytes += r.sameZoneBytes + r.crossZoneBytes + r.externalBytes
  }
  const teamList = Object.values(byTeam).sort((a, b) => b.cost - a.cost)

  // Group by period for trend
  const byPeriod: Record<string, number> = {}
  for (const r of reports) {
    const day = r.period?.slice(0, 10) || 'unknown'
    byPeriod[day] = (byPeriod[day] || 0) + r.totalCostUSD
  }
  const trendData = Object.entries(byPeriod)
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(-14)

  // Top contributors by namespace
  const byNamespace: Record<string, number> = {}
  for (const r of reports) {
    const ns = r.namespace || 'unknown'
    byNamespace[ns] = (byNamespace[ns] || 0) + r.totalCostUSD
  }
  const topContributors = Object.entries(byNamespace)
    .sort(([, a], [, b]) => b - a)
    .slice(0, 8)

  return (
    <>
      <PageHero eyebrow="Network" title="What your traffic costs." lede="Cross-zone and egress network cost tracking" />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`Network spend is $${totalCost.toFixed(2)} across ${reports.length} report${reports.length === 1 ? '' : 's'}.`}
          figures={[
            { label: 'total network spend', value: `$${totalCost.toFixed(2)}` },
            { label: 'same-zone traffic', value: formatBytes(sameZoneTotal) },
            { label: 'cross-zone traffic', value: formatBytes(crossZoneTotal) },
            { label: 'internet egress', value: formatBytes(externalTotal) },
          ]}
        />

        <section className="card span2">
          <p className="eyebrow">ZONES</p>
          <h2 className="card-title">Cost by zone type</h2>
          <CostByZoneChart
            sameZone={sameZoneTotal}
            crossZone={crossZoneTotal}
            external={externalTotal}
            costPerGB={costData?.costPerGB}
          />
        </section>

        <section className="card">
          <p className="eyebrow">TREND</p>
          <h2 className="card-title">Cost trend</h2>
          <CostTrendChart data={trendData} />
        </section>

        <section className="card span2">
          <p className="eyebrow">TEAMS</p>
          <h2 className="card-title">Cost by team</h2>
          {teamList.length === 0 ? (
            <p className="empty-state">
              No cost data available. Configure FabricNetworkCost CRs to start tracking.
            </p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Team</th>
                    <th>Cost (USD)</th>
                    <th>Traffic</th>
                    <th>% of total</th>
                  </tr>
                </thead>
                <tbody>
                  {teamList.map((team) => (
                    <tr key={team.team}>
                      <td>{team.team}</td>
                      <td className="mono">${team.cost.toFixed(2)}</td>
                      <td className="muted">{formatBytes(team.bytes)}</td>
                      <td className="faint">
                        {totalCost > 0 ? ((team.cost / totalCost) * 100).toFixed(1) : '0'}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="card">
          <p className="eyebrow">NAMESPACES</p>
          <h2 className="card-title">Top cost contributors</h2>
          {topContributors.length === 0 ? (
            <p className="empty-state">No namespace cost data available.</p>
          ) : (
            <div className="stack">
              {topContributors.map(([ns, cost]) => {
                const pct = totalCost > 0 ? (cost / totalCost) * 100 : 0
                return (
                  <div key={ns}>
                    <div className="list-row">
                      <span className="grow">{ns}</span>
                      <span className="mono">${cost.toFixed(2)}</span>
                    </div>
                    <div className="progress">
                      <span style={{ width: `${Math.max(pct, 2)}%` }} />
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </section>
      </div>
    </>
  )
}

// --- Sub-components ---

function CostByZoneChart({ sameZone, crossZone, external, costPerGB }: {
  sameZone: number; crossZone: number; external: number;
  costPerGB?: { sameZone: number; crossZone: number; internetEgress: number }
}) {
  const zones = [
    { label: 'Same Zone', bytes: sameZone, rate: costPerGB?.sameZone ?? 0.01 },
    { label: 'Cross Zone', bytes: crossZone, rate: costPerGB?.crossZone ?? 0.02 },
    { label: 'Internet Egress', bytes: external, rate: costPerGB?.internetEgress ?? 0.09 },
  ]

  const maxBytes = Math.max(...zones.map(z => z.bytes), 1)

  return (
    <div className="stack">
      {zones.map(z => {
        const cost = (z.bytes / (1024 * 1024 * 1024)) * z.rate
        return (
          <div key={z.label}>
            <div className="list-row">
              <span className="grow">{z.label}</span>
              <span className="faint">{formatBytes(z.bytes)}</span>
              <span className="mono">${cost.toFixed(2)}</span>
            </div>
            <div className="progress">
              <span style={{ width: `${Math.max((z.bytes / maxBytes) * 100, 2)}%` }} />
            </div>
            <small className="faint">${z.rate}/GB</small>
          </div>
        )
      })}
    </div>
  )
}

function CostTrendChart({ data }: { data: [string, number][] }) {
  if (data.length === 0) {
    return (
      <p className="empty-state">
        Not enough data to show trend. Costs will appear after the first reporting interval.
      </p>
    )
  }

  const maxCost = Math.max(...data.map(([, v]) => v), 0.01)
  const chartHeight = 120

  return (
    <div>
      <div style={{ display: 'grid', gridAutoFlow: 'column', gridAutoColumns: '1fr', alignItems: 'end', gap: 4, height: chartHeight }}>
        {data.map(([period, cost]) => {
          const height = (cost / maxCost) * chartHeight
          return (
            <div
              key={period}
              style={{
                height: `${Math.max(height, 2)}px`,
                background: 'var(--apple-blue)',
                borderRadius: '3px 3px 0 0',
              }}
              title={`${period}: $${cost.toFixed(2)}`}
            />
          )
        })}
      </div>
      <div className="list-row faint">
        <small className="grow">{data[0]?.[0]?.slice(5) || ''}</small>
        <small>{data[data.length - 1]?.[0]?.slice(5) || ''}</small>
      </div>
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(2)} GB`
  if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}
