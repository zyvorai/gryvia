import { useMemo } from 'react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

// Series colors come from tokens so light and dark themes both stay legible.
export const SERIES_COLORS = [
  'var(--apple-blue)',
  'var(--accent-green)',
  'var(--accent-purple)',
  'var(--accent-amber)',
  'var(--accent-cyan)',
  'var(--danger)',
  'var(--text-secondary)',
  'var(--text-tertiary)',
]

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
    return Object.keys(chartData[0]).filter((k) => k !== 'time' && k !== 'timestamp')
  }, [chartData])

  if (chartData.length === 0) {
    return <div className="list-empty">No GPU utilization data available</div>
  }

  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--hairline-1)" />
        <XAxis dataKey="time" tick={{ fontSize: 11, fill: 'var(--text-tertiary)' }} stroke="var(--hairline-1)" />
        <YAxis domain={[0, 100]} tick={{ fontSize: 11, fill: 'var(--text-tertiary)' }} stroke="var(--hairline-1)" />
        <Tooltip
          contentStyle={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--hairline-1)',
            borderRadius: 'var(--radius-md)',
            color: 'var(--text-primary)',
            fontSize: 12,
            boxShadow: 'var(--shadow-2)',
          }}
        />
        <Legend wrapperStyle={{ fontSize: 12, color: 'var(--text-secondary)' }} />
        {nodeKeys.map((key, idx) => (
          <Line
            key={key}
            type="monotone"
            dataKey={key}
            stroke={SERIES_COLORS[idx % SERIES_COLORS.length]}
            strokeWidth={2}
            dot={false}
            name={key}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  )
}
