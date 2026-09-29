import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { SERIES_COLORS } from '@/lib/chartColors'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'

const tooltipStyle = {
  background: 'var(--bg-elevated)',
  border: '1px solid var(--hairline-1)',
  borderRadius: 'var(--radius-md)',
  color: 'var(--text-primary)',
  fontSize: 12,
  boxShadow: 'var(--shadow-2)',
}
const tick = { fontSize: 11, fill: 'var(--text-tertiary)' }
const money = (value: unknown) => `$${Number(value ?? 0).toLocaleString()}`

export default function Costs() {
  const { data: costData, isLoading, isError, dataUpdatedAt } = useQuery({
    queryKey: ['costs'],
    queryFn: api.getCostData,
    refetchInterval: 60000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Costs" title="Costs unavailable." tint="red" />
        <p className="warning" role="alert">Failed to load cost data. Please check your API connection.</p>
      </>
    )
  }

  const monthlyData = costData?.monthly || []
  const teamData = costData?.byTeam || []
  const gpuTypeData = costData?.byGPUType || []
  const hasData = teamData.length > 0 || gpuTypeData.length > 0

  if (!hasData) {
    return (
      <>
        <PageHero eyebrow="Costs" title="Every dollar, tracked." lede="GPU compute costs and budget tracking" />
        <div className="grid">
          <section className="card span3">
            <p className="eyebrow">COSTS</p>
            <h2 className="card-title">No Cost Data Available</h2>
            <p className="empty-state">Cost data will appear here once jobs have been submitted and tracked.</p>
          </section>
        </div>
      </>
    )
  }

  const currentMonthCost = costData?.totalCost || 0
  const currentMonth = monthlyData.length > 0 ? monthlyData[monthlyData.length - 1] : { month: '', cost: currentMonthCost }
  const previousMonth = monthlyData.length > 1 ? monthlyData[monthlyData.length - 2] : null
  const monthOverMonth = previousMonth && previousMonth.cost > 0
    ? ((currentMonth.cost - previousMonth.cost) / previousMonth.cost) * 100
    : 0

  const dayOfMonth = new Date().getDate()
  const daysInMonth = new Date(new Date().getFullYear(), new Date().getMonth() + 1, 0).getDate()
  const projection = dayOfMonth >= 3 ? Math.round((currentMonth.cost / dayOfMonth) * daysInMonth) : null

  return (
    <>
      <PageHero eyebrow="Costs" title="Every dollar, tracked." lede="GPU compute costs and budget tracking" />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={`$${currentMonth.cost.toLocaleString()} spent this month.`}
          tone={previousMonth && monthOverMonth > 0 ? 'warn' : undefined}
          figures={[
            { label: 'Current month', value: `$${currentMonth.cost.toLocaleString()}` },
            {
              label: 'Month over month',
              value: previousMonth ? `${monthOverMonth >= 0 ? '+' : ''}${monthOverMonth.toFixed(1)}%` : 'N/A',
              tone: previousMonth && monthOverMonth > 0 ? 'warn' : undefined,
            },
            { label: 'Avg daily cost', value: `$${dayOfMonth > 0 ? (currentMonth.cost / dayOfMonth).toFixed(0) : '0'}` },
            { label: 'Projected', value: projection !== null ? `$${projection.toLocaleString()}` : 'N/A' },
          ]}
        />

        {!costData?.hasHistoricalData && monthlyData.length === 0 && (
          <p className="warning span3">Historical monthly data requires Prometheus integration. Showing current month only.</p>
        )}

      {monthlyData.length > 0 && (
        <section className="card span3">
          <p className="eyebrow">TREND</p>
          <h2 className="card-title">Monthly Cost Trend</h2>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={monthlyData}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--hairline-1)" />
              <XAxis dataKey="month" tick={tick} stroke="var(--hairline-1)" />
              <YAxis tick={tick} stroke="var(--hairline-1)" />
              <Tooltip contentStyle={tooltipStyle} formatter={money} />
              <Legend wrapperStyle={{ fontSize: 12, color: 'var(--text-secondary)' }} />
              <Bar dataKey="cost" fill="var(--apple-blue)" name="Cost ($)" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </section>
      )}

        {teamData.length > 0 && (
          <section className="card span2">
            <p className="eyebrow">TEAMS</p>
            <h2 className="card-title">Cost by Team</h2>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={teamData}
                  dataKey="cost"
                  nameKey="team"
                  cx="50%"
                  cy="50%"
                  outerRadius={100}
                  label={(props: unknown) => {
                    const p = props as { team: string; cost: number }
                    return `${p.team}: ${money(p.cost)}`
                  }}
                  labelLine={{ stroke: 'var(--hairline-1)' }}
                >
                  {teamData.map((_entry, index) => (
                    <Cell key={`cell-${index}`} fill={SERIES_COLORS[index % SERIES_COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={tooltipStyle} formatter={money} />
              </PieChart>
            </ResponsiveContainer>
          </section>
        )}

        {gpuTypeData.length > 0 && (
          <section className="card">
            <p className="eyebrow">GPU TYPES</p>
            <h2 className="card-title">Cost by GPU Type</h2>
            <div className="stack">
              {gpuTypeData.map((gpu) => {
                const totalCost = gpuTypeData.reduce((sum, g) => sum + g.cost, 0)
                const percentage = totalCost > 0 ? (gpu.cost / totalCost) * 100 : 0
                return (
                  <div key={gpu.type}>
                    <div className="row">
                      <span>{gpu.type}</span>
                      <span>
                        ${gpu.cost.toLocaleString()} <span className="faint">({gpu.hours}h)</span>
                      </span>
                    </div>
                    <div className="progress">
                      <span style={{ width: `${percentage}%` }} />
                    </div>
                  </div>
                )
              })}
            </div>

            <p className="eyebrow">GPU PRICING</p>
            {gpuTypeData.map((gpu) => (
              <div key={gpu.type} className="list-row">
                <span className="grow">{gpu.type}</span>
                <span className="faint">{gpu.hours}h</span>
                <span>${gpu.hours > 0 ? (gpu.cost / gpu.hours).toFixed(2) : '0.00'}/hr</span>
              </div>
            ))}
          </section>
        )}
      </div>
    </>
  )
}
