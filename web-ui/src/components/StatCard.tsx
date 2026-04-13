import { LucideIcon } from 'lucide-react'

interface StatCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon: LucideIcon
  color: 'blue' | 'green' | 'purple' | 'orange' | 'red' | 'cyan'
}

const gradientClasses: Record<string, string> = {
  blue: 'stat-card-blue',
  green: 'stat-card-green',
  purple: 'stat-card-purple',
  orange: 'stat-card-orange',
  red: 'stat-card-red',
  cyan: 'stat-card-cyan',
}

const iconColors: Record<string, string> = {
  blue: 'text-[#7ecbf5]',
  green: 'text-emerald-400',
  purple: 'text-purple-400',
  orange: 'text-[#e8a87c]',
  red: 'text-red-400',
  cyan: 'text-cyan-400',
}

const badgeStyles: Record<string, { bg: string; text: string }> = {
  blue: { bg: 'rgba(95,168,211,0.1)', text: '#7ecbf5' },
  green: { bg: 'rgba(34,197,94,0.1)', text: '#4ade80' },
  purple: { bg: 'rgba(168,85,247,0.1)', text: '#c084fc' },
  orange: { bg: 'rgba(212,118,78,0.1)', text: '#e8a87c' },
  red: { bg: 'rgba(239,68,68,0.1)', text: '#f87171' },
  cyan: { bg: 'rgba(6,182,212,0.1)', text: '#22d3ee' },
}

export default function StatCard({ title, value, subtitle, icon: Icon, color }: StatCardProps) {
  const badge = badgeStyles[color]
  return (
    <div className={`${gradientClasses[color]} rounded-xl p-5 card-glow transition-all duration-300 hover:scale-[1.02]`}
      style={{
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04), 0 2px 8px rgba(0,0,0,0.2)',
      }}
    >
      <div className="flex items-center justify-between mb-3">
        <div className="w-10 h-10 rounded-lg flex items-center justify-center"
          style={{
            background: 'rgba(10,14,20,0.5)',
            border: '1px solid rgba(192,204,224,0.04)',
            boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
          }}
        >
          <Icon className={`h-5 w-5 ${iconColors[color]}`} />
        </div>
        {subtitle && (
          <span className="text-[10px] font-medium px-2 py-0.5 rounded-full"
            style={{ background: badge.bg, color: badge.text }}
          >
            {subtitle}
          </span>
        )}
      </div>
      <div className="text-2xl font-bold text-[#e8ecf1]">{value}</div>
      <div className="text-xs text-[#5a7a9e] mt-1">{title}</div>
    </div>
  )
}
