import { useMemo } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ef4444', '#06b6d4', '#ec4899', '#84cc16']

interface GPUChartProps {
  data?: any[]
}

export default function GPUChart({ data }: GPUChartProps) {
  const chartData = data || []

  // Dynamically extract node keys from the data (all keys except 'time')
  const nodeKeys = useMemo(() => {
    if (chartData.length === 0) return []
    return Object.keys(chartData[0]).filter(k => k !== 'time')
  }, [chartData])

  if (chartData.length === 0) {
    return (
      <div className="flex items-center justify-center h-[300px] text-gray-400">
        No GPU utilization data available
      </div>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="time" />
        <YAxis domain={[0, 100]} />
        <Tooltip />
        <Legend />
        {nodeKeys.map((key, idx) => (
          <Line
            key={key}
            type="monotone"
            dataKey={key}
            stroke={COLORS[idx % COLORS.length]}
            name={key}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  )
}
