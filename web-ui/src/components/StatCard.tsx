import type { LucideIcon } from 'lucide-react'

interface StatCardProps {
  title: string
  value: string | number
  subtitle?: string
  /** Kept for call-site compatibility; the design contract uses no per-card icon. */
  icon?: LucideIcon
  /** Kept for call-site compatibility. Color signals deviation only, so use `tone`. */
  color?: string
  tone?: 'warn' | 'bad'
}

export default function StatCard({ title, value, subtitle, tone }: StatCardProps) {
  return (
    <div className={tone ? `stat ${tone}` : 'stat'}>
      {subtitle && <small>{subtitle}</small>}
      <b>{value}</b>
      <span>{title}</span>
    </div>
  )
}
