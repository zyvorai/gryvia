import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

export default function SecurityOverview() {
  const { data: alerts, isLoading: alertsLoading, isError: alertsError, dataUpdatedAt } = useQuery({
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
        <p className="warning" role="alert">
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
    <>
      <PageHero
        eyebrow="Security"
        title="Security detection."
        lede="eBPF-based threat detection and enforcement"
        tint={criticalAlerts > 0 ? 'red' : undefined}
      />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={criticalAlerts > 0 ? `Threats detected: ${criticalAlerts} critical alert${criticalAlerts === 1 ? '' : 's'}.` : 'All clear: no critical alerts.'}
          tone={criticalAlerts > 0 ? 'bad' : 'ok'}
          figures={[
            { label: 'critical alerts', value: criticalAlerts, tone: countTone(criticalAlerts, 1) },
            { label: 'high alerts', value: highAlerts, tone: countTone(highAlerts) },
            { label: 'active policies', value: activePolicies },
            { label: 'detection rules', value: totalRules },
          ]}
        />

        <section className="card span2">
          <p className="eyebrow">ALERTS · {alerts?.length || 0} TOTAL</p>
          <h2 className="card-title">Recent security alerts</h2>
          {(!alerts || alerts.length === 0) ? (
            <p className="empty-state">No security alerts detected. All systems secure.</p>
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

        <section className="card">
          <p className="eyebrow">POLICIES</p>
          <h2 className="card-title">Security policies</h2>
          {(!policies || policies.length === 0) ? (
            <p className="empty-state">No security policies configured</p>
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

        <section className="card span3">
          <p className="eyebrow">COVERAGE</p>
          <h2 className="card-title">Detection rules</h2>
          {['escape', 'mining', 'exfiltration', 'privesc', 'driver_fim'].map((ruleType) => {
            const isEnabled = policies?.some(p =>
              p.spec.detectionRules?.some(r => r.type === ruleType && r.enabled)
            ) ?? false
            return (
              <div key={ruleType} className="list-row">
                <span className={`dot ${isEnabled ? 'ok' : ''}`} />
                <span className="grow">{ruleType.replace('_', ' ')}</span>
                <span className="faint">{isEnabled ? 'Active' : 'Inactive'}</span>
              </div>
            )
          })}
        </section>
      </div>
    </>
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
