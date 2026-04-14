import { useQuery } from '@tanstack/react-query'
import {
  Shield, AlertTriangle, Eye, Clock, ShieldCheck, ShieldAlert,
} from 'lucide-react'
import { api } from '@/lib/api'
import type { SecurityAlert, SecurityPolicy } from '@/lib/api'
import StatCard from '@/components/StatCard'
import LoadingSpinner from '@/components/LoadingSpinner'

export default function SecurityOverview() {
  const { data: alerts, isLoading: alertsLoading, isError: alertsError } = useQuery({
    queryKey: ['securityAlerts'],
    queryFn: api.getSecurityAlerts,
    refetchInterval: 10000,
  })

  const { data: policies } = useQuery({
    queryKey: ['securityPolicies'],
    queryFn: api.getSecurityPolicies,
    refetchInterval: 30000,
  })

  if (alertsLoading) return <LoadingSpinner />

  if (alertsError) {
    return (
      <div className="p-4 rounded-xl text-sm" style={{
        background: 'rgba(239,68,68,0.06)',
        border: '1px solid rgba(239,68,68,0.15)',
        color: '#f87171',
      }}>
        Failed to load security data. Please check your API connection.
      </div>
    )
  }

  const criticalAlerts = alerts?.filter(a => a.severity === 'critical').length ?? 0
  const highAlerts = alerts?.filter(a => a.severity === 'high').length ?? 0
  const activePolicies = policies?.filter(p => p.status?.phase === 'Active').length ?? 0
  const totalRules = policies?.reduce((sum, p) => sum + (p.spec.detectionRules?.filter(r => r.enabled).length ?? 0), 0) ?? 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">Security Detection</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">eBPF-based threat detection and enforcement</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg" style={{
            background: criticalAlerts > 0 ? 'rgba(239,68,68,0.1)' : 'rgba(34,197,94,0.1)',
            border: criticalAlerts > 0 ? '1px solid rgba(239,68,68,0.2)' : '1px solid rgba(34,197,94,0.2)',
          }}>
            {criticalAlerts > 0 ? (
              <ShieldAlert className="h-4 w-4 text-red-400" />
            ) : (
              <ShieldCheck className="h-4 w-4 text-emerald-400" />
            )}
            <span className="text-xs" style={{ color: criticalAlerts > 0 ? '#f87171' : '#4ade80' }}>
              {criticalAlerts > 0 ? 'Threats Detected' : 'All Clear'}
            </span>
          </div>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={AlertTriangle}
          title="Critical Alerts"
          value={criticalAlerts}
          color="red"
        />
        <StatCard
          icon={ShieldAlert}
          title="High Alerts"
          value={highAlerts}
          color="orange"
        />
        <StatCard
          icon={Shield}
          title="Active Policies"
          value={activePolicies}
          color="blue"
        />
        <StatCard
          icon={Eye}
          title="Detection Rules"
          value={totalRules}
          color="cyan"
        />
      </div>

      {/* Main content grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Recent Alerts */}
        <div className="lg:col-span-2 space-y-6">
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 text-[#fbbf24]" />
                <h3 className="text-sm font-semibold text-[#e8ecf1]">Recent Security Alerts</h3>
              </div>
              <span className="text-xs text-[#5a7a9e]">{alerts?.length || 0} total</span>
            </div>
            {(!alerts || alerts.length === 0) ? (
              <div className="text-center py-8 text-sm text-[#344e6a]">
                No security alerts detected. All systems secure.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">SEVERITY</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">EVENT TYPE</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">PROCESS</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">PATH</th>
                      <th className="text-left py-2 px-2 text-[#5a7a9e] font-medium">TIME</th>
                    </tr>
                  </thead>
                  <tbody>
                    {alerts.slice(0, 12).map((alert, idx) => (
                      <tr key={`${alert.type}-${idx}`} className="table-row-hover" style={{ borderBottom: '1px solid rgba(192,204,224,0.03)' }}>
                        <td className="py-2 px-2">
                          <SeverityBadge severity={alert.severity} />
                        </td>
                        <td className="py-2 px-2 text-[#c0cce0] font-mono text-[11px]">{alert.type}</td>
                        <td className="py-2 px-2 text-[#8ba4c0] font-mono text-[11px]">{alert.process || '-'}</td>
                        <td className="py-2 px-2 text-[#5a7a9e] font-mono text-[11px] max-w-[200px] truncate">{alert.path || '-'}</td>
                        <td className="py-2 px-2 text-[#5a7a9e]">
                          <div className="flex items-center gap-1">
                            <Clock className="h-3 w-3" />
                            {alert.timestamp ? new Date(alert.timestamp).toLocaleTimeString() : '-'}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>

        {/* Right sidebar */}
        <div className="space-y-6">
          {/* Active Security Policies */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Shield className="h-4 w-4 text-[#7ecbf5]" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Security Policies</h3>
            </div>
            {(!policies || policies.length === 0) ? (
              <div className="text-center py-6 text-sm text-[#344e6a]">No security policies configured</div>
            ) : (
              <div className="space-y-2">
                {policies.map((policy) => {
                  const isActive = policy.status?.phase === 'Active'
                  const enabledRules = policy.spec.detectionRules?.filter(r => r.enabled).length ?? 0
                  return (
                    <div key={policy.metadata.name} className="p-2.5 rounded-lg" style={{
                      background: 'rgba(10,14,20,0.5)',
                      border: '1px solid rgba(192,204,224,0.04)',
                    }}>
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-xs font-medium text-[#c0cce0] truncate">{policy.metadata.name}</span>
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full" style={{
                          background: isActive ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)',
                          color: isActive ? '#4ade80' : '#f87171',
                        }}>
                          {policy.status?.phase || 'Pending'}
                        </span>
                      </div>
                      <div className="flex items-center gap-3 text-[10px] text-[#5a7a9e]">
                        <span>{enabledRules} rules</span>
                        <span>{policy.status?.alertsTriggered ?? 0} alerts</span>
                        {policy.spec.autoBlock && (
                          <span className="text-red-400">auto-block</span>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Detection Rule Status */}
          <div className="rounded-xl p-5" style={{
            background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
            border: '1px solid rgba(192,204,224,0.06)',
            boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
          }}>
            <div className="flex items-center gap-2 mb-4">
              <Eye className="h-4 w-4 text-emerald-400" />
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Detection Rules</h3>
            </div>
            <div className="space-y-2">
              {['escape', 'mining', 'exfiltration', 'privesc', 'driver_fim'].map((ruleType) => {
                const isEnabled = policies?.some(p =>
                  p.spec.detectionRules?.some(r => r.type === ruleType && r.enabled)
                ) ?? false
                return (
                  <div key={ruleType} className="flex items-center justify-between px-2 py-1.5">
                    <span className="text-xs text-[#8ba4c0] capitalize">{ruleType.replace('_', ' ')}</span>
                    <div className="flex items-center gap-1.5">
                      <div className="w-2 h-2 rounded-full" style={{
                        background: isEnabled ? '#4ade80' : '#344e6a',
                        boxShadow: isEnabled ? '0 0 4px rgba(34,197,94,0.4)' : 'none',
                      }} />
                      <span className="text-[10px]" style={{ color: isEnabled ? '#4ade80' : '#5a7a9e' }}>
                        {isEnabled ? 'Active' : 'Inactive'}
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const styles: Record<string, { bg: string; color: string }> = {
    critical: { bg: 'rgba(239,68,68,0.1)', color: '#f87171' },
    high: { bg: 'rgba(251,191,36,0.08)', color: '#fbbf24' },
    medium: { bg: 'rgba(96,165,250,0.08)', color: '#60a5fa' },
    low: { bg: 'rgba(148,163,184,0.08)', color: '#94a3b8' },
  }
  const s = styles[severity] || styles.low
  return (
    <span className="text-[10px] font-medium px-2 py-0.5 rounded-full uppercase" style={{
      background: s.bg,
      color: s.color,
    }}>
      {severity}
    </span>
  )
}
