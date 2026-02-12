import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import { DollarSign, TrendingUp, TrendingDown, Calendar } from 'lucide-react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4']

export default function Costs() {
  const { data: costData, isLoading } = useQuery({
    queryKey: ['costs'],
    queryFn: api.getCostData,
    refetchInterval: 60000,
  })

  if (isLoading) {
    return <LoadingSpinner />
  }

  // Mock data for demonstration
  const monthlyData = costData?.monthly || [
    { month: 'Jan', cost: 12500 },
    { month: 'Feb', cost: 14200 },
    { month: 'Mar', cost: 13800 },
    { month: 'Apr', cost: 15600 },
    { month: 'May', cost: 16800 },
    { month: 'Jun', cost: 18200 },
  ]

  const teamData = costData?.byTeam || [
    { team: 'ML Research', cost: 8500 },
    { team: 'Computer Vision', cost: 5200 },
    { team: 'NLP', cost: 3100 },
    { team: 'Robotics', cost: 1400 },
  ]

  const gpuTypeData = costData?.byGPUType || [
    { type: 'H100', cost: 12400, hours: 1550 },
    { type: 'A100-80G', cost: 4200, hours: 1050 },
    { type: 'L40', cost: 1600, hours: 640 },
  ]

  const currentMonth = monthlyData[monthlyData.length - 1]
  const previousMonth = monthlyData[monthlyData.length - 2]
  const monthOverMonth = ((currentMonth.cost - previousMonth.cost) / previousMonth.cost) * 100

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold text-gray-900">Cost Analysis</h2>
        <p className="mt-1 text-sm text-gray-500">
          GPU compute costs and budget tracking
        </p>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-4">
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <DollarSign className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-3 w-0 flex-1">
                <dt className="text-sm font-medium text-gray-500 truncate">Current Month</dt>
                <dd className="mt-1 text-2xl font-semibold text-gray-900">${currentMonth.cost.toLocaleString()}</dd>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                {monthOverMonth >= 0 ? (
                  <TrendingUp className="h-6 w-6 text-red-400" />
                ) : (
                  <TrendingDown className="h-6 w-6 text-green-400" />
                )}
              </div>
              <div className="ml-3 w-0 flex-1">
                <dt className="text-sm font-medium text-gray-500 truncate">Month over Month</dt>
                <dd className={`mt-1 text-2xl font-semibold ${monthOverMonth >= 0 ? 'text-red-600' : 'text-green-600'}`}>
                  {monthOverMonth >= 0 ? '+' : ''}{monthOverMonth.toFixed(1)}%
                </dd>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <Calendar className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-3 w-0 flex-1">
                <dt className="text-sm font-medium text-gray-500 truncate">Avg Daily Cost</dt>
                <dd className="mt-1 text-2xl font-semibold text-gray-900">
                  ${(currentMonth.cost / 30).toFixed(0)}
                </dd>
              </div>
            </div>
          </div>
        </div>

        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <div className="flex items-center">
              <div className="flex-shrink-0">
                <TrendingUp className="h-6 w-6 text-gray-400" />
              </div>
              <div className="ml-3 w-0 flex-1">
                <dt className="text-sm font-medium text-gray-500 truncate">Projected This Month</dt>
                <dd className="mt-1 text-2xl font-semibold text-gray-900">
                  ${((currentMonth.cost / new Date().getDate()) * 30).toFixed(0)}
                </dd>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Monthly Trend Chart */}
      <div className="bg-white shadow rounded-lg p-6">
        <h3 className="text-lg font-medium text-gray-900 mb-4">Monthly Cost Trend</h3>
        <ResponsiveContainer width="100%" height={300}>
          <BarChart data={monthlyData}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="month" />
            <YAxis />
            <Tooltip formatter={(value) => `$${value.toLocaleString()}`} />
            <Legend />
            <Bar dataKey="cost" fill="#3b82f6" name="Cost ($)" />
          </BarChart>
        </ResponsiveContainer>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Cost by Team */}
        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">Cost by Team</h3>
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie
                data={teamData}
                dataKey="cost"
                nameKey="team"
                cx="50%"
                cy="50%"
                outerRadius={100}
                label={(entry) => `${entry.team}: $${entry.cost.toLocaleString()}`}
              >
                {teamData.map((entry, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip formatter={(value) => `$${value.toLocaleString()}`} />
            </PieChart>
          </ResponsiveContainer>
        </div>

        {/* Cost by GPU Type */}
        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-lg font-medium text-gray-900 mb-4">Cost by GPU Type</h3>
          <div className="space-y-4">
            {gpuTypeData.map((gpu, index) => {
              const totalCost = gpuTypeData.reduce((sum, g) => sum + g.cost, 0)
              const percentage = (gpu.cost / totalCost) * 100
              return (
                <div key={gpu.type}>
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-gray-700">{gpu.type}</span>
                    <div className="text-right">
                      <span className="text-sm font-semibold text-gray-900">${gpu.cost.toLocaleString()}</span>
                      <span className="text-xs text-gray-500 ml-2">({gpu.hours}h)</span>
                    </div>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2">
                    <div
                      className="h-2 rounded-full"
                      style={{
                        width: `${percentage}%`,
                        backgroundColor: COLORS[index % COLORS.length]
                      }}
                    />
                  </div>
                </div>
              )
            })}
          </div>

          {/* GPU Pricing Table */}
          <div className="mt-6 pt-6 border-t border-gray-200">
            <h4 className="text-sm font-medium text-gray-700 mb-3">GPU Pricing</h4>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead>
                  <tr>
                    <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">GPU Type</th>
                    <th className="px-3 py-2 text-right text-xs font-medium text-gray-500 uppercase">$/Hour</th>
                    <th className="px-3 py-2 text-right text-xs font-medium text-gray-500 uppercase">Hours</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {gpuTypeData.map((gpu) => (
                    <tr key={gpu.type}>
                      <td className="px-3 py-2 text-sm text-gray-900">{gpu.type}</td>
                      <td className="px-3 py-2 text-sm text-gray-900 text-right">
                        ${(gpu.cost / gpu.hours).toFixed(2)}
                      </td>
                      <td className="px-3 py-2 text-sm text-gray-900 text-right">{gpu.hours}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
