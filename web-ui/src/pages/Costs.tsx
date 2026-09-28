import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
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
  const { data: costData, isLoading, isError } = useQuery({
    queryKey: ['costs'],
    queryFn: api.getCostData,
    refetchInterval: 60000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Costs" title="Costs unavailable." tint="red" />
        <p className="login-error" role="alert">Failed to load cost data. Please check your API connection.</p>
      </>
    )
  }

  const monthlyData = costData?.monthly || []
  const teamData = costData?.byTeam || []
  const gpuTypeData = costData?.byGPUType || []
  const hasData = teamData.length > 0 || gpuTypeData.length > 0

  if (!hasData) {
    return (
      <div className="apple-story-stack">
        <PageHero eyebrow="Costs" title="Every dollar, tracked." lede="GPU compute costs and budget tracking" />
        <div className="list-empty">
          <h3>No Cost Data Available</h3>
          <p>Cost data will appear here once jobs have been submitted and tracked.</p>
        </div>
      </div>
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
    <div className="apple-story-stack">
      <PageHero eyebrow="Costs" title="Every dollar, tracked." lede="GPU compute costs and budget tracking" />

      {!costData?.hasHistoricalData && monthlyData.length === 0 && (
        <p className="text-warn">Historical monthly data requires Prometheus integration. Showing current month only.</p>
      )}

      <div className="apple-metric-band">
        <div>
          <span>Current Month</span>
          <b>${currentMonth.cost.toLocaleString()}</b>
        </div>
        <div>
          <span>Month over Month</span>
          <b className={previousMonth && monthOverMonth > 0 ? 'text-bad' : undefined}>
            {previousMonth ? `${monthOverMonth >= 0 ? '+' : ''}${monthOverMonth.toFixed(1)}%` : 'N/A'}
          </b>
        </div>
        <div>
          <span>Avg Daily Cost</span>
          <b>${dayOfMonth > 0 ? (currentMonth.cost / dayOfMonth).toFixed(0) : '0'}</b>
        </div>
        <div>
          <span>Projected</span>
          <b>{projection !== null ? `$${projection.toLocaleString()}` : 'N/A'}</b>
        </div>
      </div>

      {monthlyData.length > 0 && (
        <section className="card">
          <h2>Monthly Cost Trend</h2>
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

      <div className="card-grid">
        {teamData.length > 0 && (
          <section className="card">
            <h2>Cost by Team</h2>
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
            <h2>Cost by GPU Type</h2>
            <div className="stack">
              {gpuTypeData.map((gpu) => {
                const totalCost = gpuTypeData.reduce((sum, g) => sum + g.cost, 0)
                const percentage = totalCost > 0 ? (gpu.cost / totalCost) * 100 : 0
                return (
                  <div key={gpu.type}>
                    <div className="stat-head" style={{ marginBottom: 6 }}>
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

            <h3 style={{ marginTop: 24 }}>GPU Pricing</h3>
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
    </div>
  )
}
