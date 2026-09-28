import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'

export default function NetworkCost() {
  const { data: costData, isLoading, isError } = useQuery({
    queryKey: ['networkCosts'],
    queryFn: api.getNetworkCosts,
    refetchInterval: 60000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Network" title="Cost data unavailable." tint="red" />
        <p className="login-error" role="alert">
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
    <div className="apple-story-stack">
      <PageHero eyebrow="Network" title="What your traffic costs." lede="Cross-zone and egress network cost tracking" />

      <div className="apple-metric-band">
        <div>
          <span>Total Network Spend</span>
          <b>{`$${totalCost.toFixed(2)}`}</b>
        </div>
        <div>
          <span>Same-Zone Traffic</span>
          <b>{formatBytes(sameZoneTotal)}</b>
        </div>
        <div>
          <span>Cross-Zone Traffic</span>
          <b>{formatBytes(crossZoneTotal)}</b>
        </div>
        <div>
          <span>Internet Egress</span>
          <b>{formatBytes(externalTotal)}</b>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <section className="card">
          <h2>Cost by Zone Type</h2>
          <CostByZoneChart
            sameZone={sameZoneTotal}
            crossZone={crossZoneTotal}
            external={externalTotal}
            costPerGB={costData?.costPerGB}
          />
        </section>

        <section className="card">
          <h2>Cost Trend</h2>
          <CostTrendChart data={trendData} />
        </section>

        <section className="card">
          <h2>Cost by Team</h2>
          {teamList.length === 0 ? (
            <div className="list-empty">
              No cost data available. Configure FabricNetworkCost CRs to start tracking.
            </div>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Team</th>
                    <th style={{ textAlign: 'right' }}>Cost (USD)</th>
                    <th style={{ textAlign: 'right' }}>Traffic</th>
                    <th style={{ textAlign: 'right' }}>% of total</th>
                  </tr>
                </thead>
                <tbody>
                  {teamList.map((team) => (
                    <tr key={team.team}>
                      <td>{team.team}</td>
                      <td className="mono" style={{ textAlign: 'right' }}>${team.cost.toFixed(2)}</td>
                      <td className="muted" style={{ textAlign: 'right' }}>{formatBytes(team.bytes)}</td>
                      <td className="faint" style={{ textAlign: 'right' }}>
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
          <h2>Top Cost Contributors</h2>
          {topContributors.length === 0 ? (
            <div className="list-empty">No namespace cost data available.</div>
          ) : (
            <div className="stack">
              {topContributors.map(([ns, cost]) => {
                const pct = totalCost > 0 ? (cost / totalCost) * 100 : 0
                return (
                  <div key={ns}>
                    <div className="stat-head" style={{ marginBottom: 6 }}>
                      <span>{ns}</span>
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
    </div>
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
            <div className="stat-head" style={{ marginBottom: 6 }}>
              <span>{z.label}</span>
              <span>
                <span className="faint">{formatBytes(z.bytes)}</span> <span className="mono">${cost.toFixed(2)}</span>
              </span>
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
      <div className="list-empty">
        Not enough data to show trend. Costs will appear after the first reporting interval.
      </div>
    )
  }

  const maxCost = Math.max(...data.map(([, v]) => v), 0.01)
  const chartHeight = 120

  return (
    <div>
      <div className="flex items-end gap-1" style={{ height: chartHeight }}>
        {data.map(([period, cost]) => {
          const height = (cost / maxCost) * chartHeight
          return (
            <div
              key={period}
              className="flex-1"
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
      <div className="stat-head faint" style={{ marginTop: 8, fontSize: 11 }}>
        <span>{data[0]?.[0]?.slice(5) || ''}</span>
        <span>{data[data.length - 1]?.[0]?.slice(5) || ''}</span>
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
