import { useMemo } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

const COLORS = ['#e8a87c', '#4ade80', '#7ecbf5', '#c084fc', '#f87171', '#22d3ee', '#fbbf24', '#a78bfa']

interface GPUDataPoint {
  time: string
  [nodeKey: string]: string | number
}

interface GPUChartProps {
  data?: GPUDataPoint[]
}

export default function GPUChart({ data }: GPUChartProps) {
  const chartData = data || []

  const nodeKeys = useMemo(() => {
    if (chartData.length === 0) return []
    return Object.keys(chartData[0]).filter(k => k !== 'time' && k !== 'timestamp')
  }, [chartData])

  if (chartData.length === 0) {
    return (
      <div className="flex items-center justify-center h-[300px] text-[#5a7a9e] text-sm">
        No GPU utilization data available
      </div>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" stroke="rgba(192,204,224,0.06)" />
        <XAxis dataKey="time" tick={{ fontSize: 11, fill: '#5a7a9e' }} stroke="rgba(192,204,224,0.08)" />
        <YAxis domain={[0, 100]} tick={{ fontSize: 11, fill: '#5a7a9e' }} stroke="rgba(192,204,224,0.08)" />
        <Tooltip
          contentStyle={{
            backgroundColor: '#111820',
            border: '1px solid rgba(192,204,224,0.1)',
            borderRadius: '10px',
            color: '#d0dae6',
            fontSize: '12px',
            boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
          }}
        />
        <Legend wrapperStyle={{ fontSize: '12px', color: '#8090a8' }} />
        {nodeKeys.map((key, idx) => (
          <Line
            key={key}
            type="monotone"
            dataKey={key}
            stroke={COLORS[idx % COLORS.length]}
            strokeWidth={2}
            dot={false}
            name={key}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  )
}
