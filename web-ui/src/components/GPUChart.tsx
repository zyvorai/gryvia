import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

interface GPUChartProps {
  data?: any[]
}

export default function GPUChart({ data }: GPUChartProps) {
  // Mock data for demonstration
  const mockData = data || [
    { time: '00:00', node1: 45, node2: 62, node3: 78, node4: 52 },
    { time: '00:15', node1: 52, node2: 68, node3: 82, node4: 58 },
    { time: '00:30', node1: 58, node2: 72, node3: 85, node4: 62 },
    { time: '00:45', node1: 62, node2: 75, node3: 88, node4: 68 },
    { time: '01:00', node1: 68, node2: 78, node3: 90, node4: 72 },
  ]

  return (
    <ResponsiveContainer width="100%" height={300}>
      <LineChart data={mockData}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="time" />
        <YAxis domain={[0, 100]} />
        <Tooltip />
        <Legend />
        <Line type="monotone" dataKey="node1" stroke="#3b82f6" name="Node 1" />
        <Line type="monotone" dataKey="node2" stroke="#10b981" name="Node 2" />
        <Line type="monotone" dataKey="node3" stroke="#f59e0b" name="Node 3" />
        <Line type="monotone" dataKey="node4" stroke="#8b5cf6" name="Node 4" />
      </LineChart>
    </ResponsiveContainer>
  )
}
