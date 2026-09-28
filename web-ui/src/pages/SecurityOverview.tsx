import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'

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
      <>
        <PageHero eyebrow="Security" title="Security data unavailable." tint="red" />
        <p className="login-error" role="alert">
          Failed to load security data. Please check your API connection.
        </p>
      </>
    )
  }

  const criticalAlerts = alerts?.filter(a => a.severity === 'critical').length ?? 0
  const highAlerts = alerts?.filter(a => a.severity === 'high').length ?? 0
  const activePolicies = policies?.filter(p => p.status?.phase === 'Active').length ?? 0
  const totalRules = policies?.reduce((sum, p) => sum + (p.spec.detectionRules?.filter(r => r.enabled).length ?? 0), 0) ?? 0

  return (
    <div className="apple-story-stack">
      <PageHero
        eyebrow="Security"
        title="Security detection."
        lede="eBPF-based threat detection and enforcement"
        tint={criticalAlerts > 0 ? 'red' : undefined}
      />

      <div className="row">
        <span className={`pill ${criticalAlerts > 0 ? 'bad' : 'ok'}`}>
          {criticalAlerts > 0 ? 'Threats Detected' : 'All Clear'}
        </span>
      </div>

      <div className="apple-metric-band">
        <div>
          <span>Critical Alerts</span>
          <b className={criticalAlerts > 0 ? 'text-bad' : undefined}>{criticalAlerts}</b>
        </div>
        <div>
          <span>High Alerts</span>
          <b className={highAlerts > 0 ? 'text-warn' : undefined}>{highAlerts}</b>
        </div>
        <div>
          <span>Active Policies</span>
          <b>{activePolicies}</b>
        </div>
        <div>
          <span>Detection Rules</span>
          <b>{totalRules}</b>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 stack">
          <section className="card">
            <div className="stat-head">
              <h2>Recent Security Alerts</h2>
              <span className="faint">{alerts?.length || 0} total</span>
            </div>
            {(!alerts || alerts.length === 0) ? (
              <div className="list-empty">No security alerts detected. All systems secure.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Severity</th>
                      <th>Event type</th>
                      <th>Process</th>
                      <th>Path</th>
                      <th>Time</th>
                    </tr>
                  </thead>
                  <tbody>
                    {alerts.slice(0, 12).map((alert, idx) => (
                      <tr key={`${alert.type}-${idx}`}>
                        <td>
                          <SeverityBadge severity={alert.severity} />
                        </td>
                        <td className="mono">{alert.type}</td>
                        <td className="mono muted">{alert.process || '-'}</td>
                        <td className="mono faint">{alert.path || '-'}</td>
                        <td className="faint">{alert.timestamp ? new Date(alert.timestamp).toLocaleTimeString() : '-'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </div>

        <div className="stack">
          <section className="card">
            <h2>Security Policies</h2>
            {(!policies || policies.length === 0) ? (
              <div className="list-empty">No security policies configured</div>
            ) : (
              policies.map((policy) => {
                const isActive = policy.status?.phase === 'Active'
                const enabledRules = policy.spec.detectionRules?.filter(r => r.enabled).length ?? 0
                return (
                  <div key={policy.metadata.name} className="list-row">
                    <div className="grow">
                      <b>{policy.metadata.name}</b>
                      <small>
                        {enabledRules} rules · {policy.status?.alertsTriggered ?? 0} alerts
                        {policy.spec.autoBlock && <span className="text-bad"> · auto-block</span>}
                      </small>
                    </div>
                    <span className={`pill ${isActive ? 'ok' : 'bad'}`}>{policy.status?.phase || 'Pending'}</span>
                  </div>
                )
              })
            )}
          </section>

          <section className="card">
            <h2>Detection Rules</h2>
            {['escape', 'mining', 'exfiltration', 'privesc', 'driver_fim'].map((ruleType) => {
              const isEnabled = policies?.some(p =>
                p.spec.detectionRules?.some(r => r.type === ruleType && r.enabled)
              ) ?? false
              return (
                <div key={ruleType} className="list-row">
                  <span className={`dot ${isEnabled ? 'ok' : ''}`} />
                  <span className="grow" style={{ textTransform: 'capitalize' }}>{ruleType.replace('_', ' ')}</span>
                  <span className="faint">{isEnabled ? 'Active' : 'Inactive'}</span>
                </div>
              )
            })}
          </section>
        </div>
      </div>
    </div>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const tones: Record<string, string> = {
    critical: 'bad',
    high: 'warn',
    medium: 'info',
    low: '',
  }
  return <span className={`pill ${tones[severity] ?? ''}`}>{severity}</span>
}
