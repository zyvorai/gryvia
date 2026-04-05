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
  blue: 'text-blue-400',
  green: 'text-green-400',
  purple: 'text-purple-400',
  orange: 'text-orange-400',
  red: 'text-red-400',
  cyan: 'text-cyan-400',
}

const badgeColors: Record<string, string> = {
  blue: 'bg-blue-500/10 text-blue-400',
  green: 'bg-green-500/10 text-green-400',
  purple: 'bg-purple-500/10 text-purple-400',
  orange: 'bg-orange-500/10 text-orange-400',
  red: 'bg-red-500/10 text-red-400',
  cyan: 'bg-cyan-500/10 text-cyan-400',
}

export default function StatCard({ title, value, subtitle, icon: Icon, color }: StatCardProps) {
  return (
    <div className={`${gradientClasses[color]} rounded-xl border border-slate-700/50 p-5 card-glow transition-all hover:scale-[1.02]`}>
      <div className="flex items-center justify-between mb-3">
        <div className="w-10 h-10 rounded-lg bg-slate-900/50 flex items-center justify-center">
          <Icon className={`h-5 w-5 ${iconColors[color]}`} />
        </div>
        {subtitle && (
          <span className={`text-[10px] font-medium px-2 py-0.5 rounded-full ${badgeColors[color]}`}>
            {subtitle}
          </span>
        )}
      </div>
      <div className="text-2xl font-bold text-white">{value}</div>
      <div className="text-xs text-slate-400 mt-1">{title}</div>
    </div>
  )
}
