import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import StatCard from '@/components/StatCard'
import { DollarSign, TrendingUp, TrendingDown, Calendar } from 'lucide-react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4']

export default function Costs() {
  const { data: costData, isLoading, isError } = useQuery({
    queryKey: ['costs'],
    queryFn: api.getCostData,
    refetchInterval: 60000,
  })

  if (isLoading) return <LoadingSpinner />

  if (isError) {
    return (
      <div className="p-4 bg-red-500/10 border border-red-500/30 text-red-400 rounded-xl text-sm">
        Failed to load cost data. Please check your API connection.
      </div>
    )
  }

  const monthlyData = costData?.monthly || []
  const teamData = costData?.byTeam || []
  const gpuTypeData = costData?.byGPUType || []
  const hasData = teamData.length > 0 || gpuTypeData.length > 0

  if (!hasData) {
    return (
      <div className="space-y-6">
        <div>
          <h2 className="text-2xl font-bold text-gradient-blue">Cost Analysis</h2>
          <p className="text-sm text-slate-400 mt-1">GPU compute costs and budget tracking</p>
        </div>
        <div className="p-8 text-center bg-slate-800/50 rounded-xl border border-slate-700/50">
          <DollarSign className="h-12 w-12 text-slate-600 mx-auto mb-4" />
          <h3 className="text-sm font-semibold text-white mb-2">No Cost Data Available</h3>
          <p className="text-xs text-slate-400">Cost data will appear here once jobs have been submitted and tracked.</p>
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
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gradient-blue">Cost Analysis</h2>
        <p className="text-sm text-slate-400 mt-1">GPU compute costs and budget tracking</p>
      </div>

      {!costData?.hasHistoricalData && monthlyData.length === 0 && (
        <div className="rounded-xl bg-yellow-500/10 border border-yellow-500/20 p-4">
          <p className="text-xs text-yellow-400">Historical monthly data requires Prometheus integration. Showing current month only.</p>
        </div>
      )}

      {/* Summary Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard icon={DollarSign} title="Current Month" value={`$${currentMonth.cost.toLocaleString()}`} color="blue" />
        <StatCard
          icon={monthOverMonth >= 0 ? TrendingUp : TrendingDown}
          title="Month over Month"
          value={previousMonth ? `${monthOverMonth >= 0 ? '+' : ''}${monthOverMonth.toFixed(1)}%` : 'N/A'}
          color={monthOverMonth >= 0 ? 'red' : 'green'}
        />
        <StatCard
          icon={Calendar}
          title="Avg Daily Cost"
          value={`$${dayOfMonth > 0 ? (currentMonth.cost / dayOfMonth).toFixed(0) : '0'}`}
          color="purple"
        />
        <StatCard
          icon={TrendingUp}
          title="Projected"
          value={projection !== null ? `$${projection.toLocaleString()}` : 'N/A'}
          color="cyan"
        />
      </div>

      {/* Monthly Trend Chart */}
      {monthlyData.length > 0 && (
        <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
          <h3 className="text-sm font-semibold text-white mb-4">Monthly Cost Trend</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={monthlyData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
              <XAxis dataKey="month" tick={{ fontSize: 11, fill: '#94a3b8' }} stroke="#475569" />
              <YAxis tick={{ fontSize: 11, fill: '#94a3b8' }} stroke="#475569" />
              <Tooltip contentStyle={{ backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px', color: '#e2e8f0', fontSize: '12px' }}
                formatter={(value: number) => `$${value.toLocaleString()}`} />
              <Legend wrapperStyle={{ fontSize: '12px', color: '#94a3b8' }} />
              <Bar dataKey="cost" fill="#3b82f6" name="Cost ($)" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Cost by Team */}
        {teamData.length > 0 && (
          <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
            <h3 className="text-sm font-semibold text-white mb-4">Cost by Team</h3>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie data={teamData} dataKey="cost" nameKey="team" cx="50%" cy="50%" outerRadius={100}
                  label={(entry) => `${entry.team}: $${entry.cost.toLocaleString()}`}
                  labelLine={{ stroke: '#475569' }}
                >
                  {teamData.map((_entry, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={{ backgroundColor: '#1e293b', border: '1px solid #334155', borderRadius: '8px', color: '#e2e8f0', fontSize: '12px' }}
                  formatter={(value: number) => `$${value.toLocaleString()}`} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        )}

        {/* Cost by GPU Type */}
        {gpuTypeData.length > 0 && (
          <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
            <h3 className="text-sm font-semibold text-white mb-4">Cost by GPU Type</h3>
            <div className="space-y-4">
              {gpuTypeData.map((gpu, index) => {
                const totalCost = gpuTypeData.reduce((sum, g) => sum + g.cost, 0)
                const percentage = totalCost > 0 ? (gpu.cost / totalCost) * 100 : 0
                return (
                  <div key={gpu.type}>
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-xs font-medium text-slate-300">{gpu.type}</span>
                      <div className="text-right">
                        <span className="text-xs font-semibold text-white">${gpu.cost.toLocaleString()}</span>
                        <span className="text-[10px] text-slate-500 ml-2">({gpu.hours}h)</span>
                      </div>
                    </div>
                    <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
                      <div className="h-full rounded-full" style={{ width: `${percentage}%`, backgroundColor: COLORS[index % COLORS.length] }} />
                    </div>
                  </div>
                )
              })}
            </div>

            {/* GPU Pricing Table */}
            <div className="mt-6 pt-4 border-t border-slate-700/30">
              <span className="text-xs font-medium text-slate-400">GPU Pricing</span>
              <div className="mt-2 space-y-1">
                {gpuTypeData.map((gpu) => (
                  <div key={gpu.type} className="flex items-center justify-between py-1.5">
                    <span className="text-xs text-slate-300">{gpu.type}</span>
                    <div className="flex items-center gap-4">
                      <span className="text-[10px] text-slate-500">{gpu.hours}h</span>
                      <span className="text-xs font-medium text-white">${gpu.hours > 0 ? (gpu.cost / gpu.hours).toFixed(2) : '0.00'}/hr</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
