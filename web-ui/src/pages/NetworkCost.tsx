import { useQuery } from '@tanstack/react-query'
import {
  DollarSign, TrendingUp, BarChart3, Clock, ArrowUpRight,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { NetworkCostReport } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function NetworkCost() {
  const { data: costData, isLoading, isError } = useQuery({
    queryKey: ['networkCosts'],
    queryFn: api.getNetworkCosts,
    refetchInterval: 60000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load network cost data. Please check your API connection.
      </div>
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
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Network Costs</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">Cross-zone and egress network cost tracking</p>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={DollarSign}
          title="Total Network Spend"
          value={`$${totalCost.toFixed(2)}`}
          color="orange"
        />
        <StatCard
          icon={TrendingUp}
          title="Same-Zone Traffic"
          value={formatBytes(sameZoneTotal)}
          color="green"
        />
        <StatCard
          icon={ArrowUpRight}
          title="Cross-Zone Traffic"
          value={formatBytes(crossZoneTotal)}
          color="blue"
        />
        <StatCard
          icon={BarChart3}
          title="Internet Egress"
          value={formatBytes(externalTotal)}
          color="red"
        />
      </div>

      {/* Main content grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Cost by Zone Type */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <BarChart3 className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Cost by Zone Type</h3>
          </div>
          <CostByZoneChart
            sameZone={sameZoneTotal}
            crossZone={crossZoneTotal}
            external={externalTotal}
            costPerGB={costData?.costPerGB}
          />
        </div>

        {/* Cost Trend */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <TrendingUp className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Cost Trend</h3>
          </div>
          <CostTrendChart data={trendData} />
        </div>

        {/* Per-Team Cost Breakdown */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <DollarSign className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Cost by Team</h3>
          </div>
          {teamList.length === 0 ? (
            <div className="text-center py-8 text-sm text-[#344e6a]">
              No cost data available. Configure FabricNetworkCost CRs to start tracking.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
                    <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">TEAM</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">COST (USD)</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">TRAFFIC</th>
                    <th className="text-right py-2 px-2 text-[#5a7a9e] font-medium">% OF TOTAL</th>
                  </tr>
                </thead>
                <tbody>
                  {teamList.map((team) => (
                    <tr key={team.team} className="table-row-hover" style={{ borderBottom: '1px solid rgba(192,204,224,0.03)' }}>
                      <td className="py-2 px-2 text-[#c0cce0]">{team.team}</td>
                      <td className="py-2 px-2 text-right text-[#e8a87c] font-mono">${team.cost.toFixed(2)}</td>
                      <td className="py-2 px-2 text-right text-[#8ba4c0]">{formatBytes(team.bytes)}</td>
                      <td className="py-2 px-2 text-right text-[#5a7a9e]">
                        {totalCost > 0 ? ((team.cost / totalCost) * 100).toFixed(1) : '0'}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Top Cost Contributors */}
        <div className="rounded-xl p-5" style={{
          background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
          border: '1px solid rgba(192,204,224,0.06)',
          boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
        }}>
          <div className="flex items-center gap-2 mb-4">
            <ArrowUpRight className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">Top Cost Contributors</h3>
          </div>
          {topContributors.length === 0 ? (
            <div className="text-center py-8 text-sm text-[#344e6a]">
              No namespace cost data available.
            </div>
          ) : (
            <div className="space-y-3">
              {topContributors.map(([ns, cost]) => {
                const pct = totalCost > 0 ? (cost / totalCost) * 100 : 0
                return (
                  <div key={ns}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-xs text-[#c0cce0]">{ns}</span>
                      <span className="text-xs font-mono text-[#e8a87c]">${cost.toFixed(2)}</span>
                    </div>
                    <div className="h-2 rounded-full overflow-hidden" style={{ background: 'rgba(10,14,20,0.5)' }}>
                      <div
                        className="h-full rounded-full transition-all duration-500"
                        style={{
                          width: `${Math.max(pct, 2)}%`,
                          background: 'linear-gradient(90deg, rgba(212,118,78,0.6), rgba(232,168,124,0.9))',
                          boxShadow: '0 0 6px rgba(212,118,78,0.2)',
                        }}
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
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
    { label: 'Same Zone', bytes: sameZone, rate: costPerGB?.sameZone ?? 0.01, color: '#4ade80' },
    { label: 'Cross Zone', bytes: crossZone, rate: costPerGB?.crossZone ?? 0.02, color: '#60a5fa' },
    { label: 'Internet Egress', bytes: external, rate: costPerGB?.internetEgress ?? 0.09, color: '#f87171' },
  ]

  const maxBytes = Math.max(...zones.map(z => z.bytes), 1)

  return (
    <div className="space-y-4">
      {zones.map(z => {
        const cost = (z.bytes / (1024 * 1024 * 1024)) * z.rate
        return (
          <div key={z.label}>
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-[#c0cce0]">{z.label}</span>
              <div className="flex items-center gap-3">
                <span className="text-[10px] text-[#5a7a9e]">{formatBytes(z.bytes)}</span>
                <span className="text-xs font-mono" style={{ color: z.color }}>${cost.toFixed(2)}</span>
              </div>
            </div>
            <div className="h-3 rounded-full overflow-hidden" style={{ background: 'rgba(10,14,20,0.5)' }}>
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{
                  width: `${Math.max((z.bytes / maxBytes) * 100, 2)}%`,
                  background: `linear-gradient(90deg, ${z.color}60, ${z.color})`,
                  boxShadow: `0 0 6px ${z.color}20`,
                }}
              />
            </div>
            <div className="text-[10px] text-[#5a7a9e] mt-0.5">${z.rate}/GB</div>
          </div>
        )
      })}
    </div>
  )
}

function CostTrendChart({ data }: { data: [string, number][] }) {
  if (data.length === 0) {
    return (
      <div className="text-center py-8 text-sm text-[#344e6a]">
        Not enough data to show trend. Costs will appear after the first reporting interval.
      </div>
    )
  }

  const maxCost = Math.max(...data.map(([, v]) => v), 0.01)
  const chartHeight = 120

  return (
    <div>
      <div className="flex items-end gap-1" style={{ height: chartHeight }}>
        {data.map(([period, cost], i) => {
          const height = (cost / maxCost) * chartHeight
          return (
            <div
              key={period}
              className="flex-1 rounded-t transition-all duration-300 hover:opacity-80"
              style={{
                height: `${Math.max(height, 2)}px`,
                background: 'linear-gradient(180deg, rgba(232,168,124,0.8), rgba(212,118,78,0.4))',
                boxShadow: '0 0 4px rgba(212,118,78,0.15)',
              }}
              title={`${period}: $${cost.toFixed(2)}`}
            />
          )
        })}
      </div>
      <div className="flex justify-between mt-2 text-[9px] text-[#5a7a9e]">
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
