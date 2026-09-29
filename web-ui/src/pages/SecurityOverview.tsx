import { useQuery } from '@tanstack/react-query'
import apiClient, { api } from '@/lib/api'
import type { SecurityAlert, SecurityPolicy } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'
import { phaseTone } from '@/lib/phase'
import { formatDate, formatRelative } from '@/lib/format'
import { errorMessage } from '@/lib/errors'
import { severityTone } from '@/lib/network'
import { RULE_CATALOG, activeRuleTypes, countActivePolicies, policyCounters, ruleName } from '@/lib/security'

type AlertsResponse = { items: SecurityAlert[]; eventSource?: boolean }

// The shared api.getSecurityAlerts drops `eventSource`, so read the full response here.
async function fetchAlerts(): Promise<AlertsResponse> {
  const { data } = await apiClient.get('/security/alerts')
  return { items: data?.items ?? [], eventSource: data?.eventSource }
}

export default function SecurityOverview() {
  useDocumentTitle('Security')
  const alertsQ = useQuery({ queryKey: ['securityAlerts'], queryFn: fetchAlerts, refetchInterval: 10000 })
  const policiesQ = useQuery({ queryKey: ['securityPolicies'], queryFn: api.getSecurityPolicies, refetchInterval: 30000 })

  const alerts = alertsQ.data?.items
  const eventSource = alertsQ.data ? alertsQ.data.eventSource !== false : undefined
  const policies = policiesQ.data
  const critical = eventSource && alerts ? alerts.filter((a) => a.severity === 'critical').length : undefined
  const high = eventSource && alerts ? alerts.filter((a) => a.severity === 'high').length : undefined
  const activeRules = policies ? activeRuleTypes(policies) : undefined
  const totalRules = policies ? policies.reduce((n, p) => n + ((p.status?.phase ?? '').toLowerCase() === 'active' ? (p.spec.detectionRules?.filter((r) => r.enabled).length ?? 0) : 0), 0) : undefined

  let headline: string | undefined
  let headlineTone: 'ok' | 'bad' | undefined
  if (alertsQ.data) {
    if (eventSource === false) headline = 'No event source connected: alerts cannot be shown.'
    else if (critical && critical > 0) {
      headline = `Threats detected: ${critical} critical alert${critical === 1 ? '' : 's'}.`
      headlineTone = 'bad'
    } else {
      headline = 'All clear: no critical alerts.'
      headlineTone = 'ok'
    }
  }

  return (
    <>
      <PageHero eyebrow="Security" title="Security detection." lede="eBPF-based threat detection and enforcement" tint={critical && critical > 0 ? 'red' : undefined} />

      <div className="grid">
        {alertsQ.isLoading ? (
          <section className="card span3" aria-label="Live summary">
            <Skeleton rows={2} />
          </section>
        ) : alertsQ.isError && !alertsQ.data ? (
          <div className="span3">
            <ErrorState title="Could not load security alerts." error={alertsQ.error} onRetry={() => alertsQ.refetch()} retrying={alertsQ.isFetching} />
          </div>
        ) : (
          <PagePulse
            updatedAt={alertsQ.dataUpdatedAt}
            error={alertsQ.isError ? errorMessage(alertsQ.error) : undefined}
            headline={headline}
            tone={headlineTone}
            figures={[
              { label: 'critical alerts', value: critical ?? '—', tone: critical && critical > 0 ? 'bad' : undefined },
              { label: 'high alerts', value: high ?? '—', tone: high && high > 0 ? 'warn' : undefined },
              { label: 'active policies', value: policies ? countActivePolicies(policies) : undefined },
              { label: 'active rules', value: totalRules },
            ]}
          />
        )}

        <section className="card span2">
          <p className="eyebrow">ALERTS{alertsQ.data && eventSource ? ` · ${alerts?.length ?? 0} TOTAL` : ''}</p>
          <h2 className="card-title">Recent security alerts</h2>
          {alertsQ.isLoading ? (
            <Skeleton rows={4} />
          ) : alertsQ.isError && !alertsQ.data ? (
            <ErrorState title="Could not load security alerts." error={alertsQ.error} onRetry={() => alertsQ.refetch()} retrying={alertsQ.isFetching} />
          ) : eventSource === false ? (
            <EmptyState
              title="No event source connected"
              action={
                <button type="button" className="btn-secondary" onClick={() => alertsQ.refetch()} disabled={alertsQ.isFetching}>
                  {alertsQ.isFetching ? 'Checking…' : 'Check again'}
                </button>
              }
            >
              Alerts come from the security operator&apos;s eBPF event stream. Until it is connected, an empty list does not mean your workloads are safe.
            </EmptyState>
          ) : !alerts || alerts.length === 0 ? (
            <EmptyState title="No security alerts">The event source is connected and has reported no alerts.</EmptyState>
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
                        <span className={`pill ${severityTone(alert.severity)}`}>{alert.severity}</span>
                      </td>
                      <td title={alert.type}>{ruleName(alert.type)}</td>
                      <td className="mono muted">{alert.process || '—'}</td>
                      <td className="mono faint">{alert.path || '—'}</td>
                      <td className="faint" title={alert.timestamp ? formatDate(alert.timestamp) : undefined}>
                        {alert.timestamp ? `${formatDate(alert.timestamp)} · ${formatRelative(alert.timestamp)}` : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {policies && policies.length > 0 && <CounterSummary policies={policies} />}
        </section>

        <section className="card">
          <p className="eyebrow">POLICIES</p>
          <h2 className="card-title">Security policies</h2>
          {policiesQ.isLoading ? (
            <Skeleton rows={3} />
          ) : policiesQ.isError && !policies ? (
            <ErrorState title="Could not load security policies." error={policiesQ.error} onRetry={() => policiesQ.refetch()} retrying={policiesQ.isFetching} />
          ) : !policies || policies.length === 0 ? (
            <EmptyState title="No security policies configured" action={<button type="button" className="btn-secondary" onClick={() => policiesQ.refetch()}>Refresh</button>}>
              Security policies are FabricSecurityPolicy resources evaluated by the security operator. Without one, no detection rules run.
            </EmptyState>
          ) : (
            policies.map((policy) => <PolicyRow key={policy.metadata.name} policy={policy} />)
          )}
        </section>

        <section className="card span3">
          <p className="eyebrow">COVERAGE</p>
          <h2 className="card-title">Detection rules</h2>
          {policiesQ.isLoading ? (
            <Skeleton rows={5} />
          ) : policiesQ.isError && !policies ? (
            <ErrorState title="Could not load detection coverage." error={policiesQ.error} onRetry={() => policiesQ.refetch()} retrying={policiesQ.isFetching} />
          ) : (
            RULE_CATALOG.map((rule) => {
              const isEnabled = activeRules?.has(rule.type) ?? false
              return (
                <div key={rule.type} className="list-row">
                  <span className={`dot ${isEnabled ? 'ok' : 'warn'}`} aria-hidden="true" />
                  <div className="grow">
                    <b>{rule.name}</b>
                    <small>{rule.description}</small>
                  </div>
                  <span className={`pill ${isEnabled ? 'ok' : 'warn'}`}>{isEnabled ? 'Active' : 'Not covered'}</span>
                </div>
              )
            })
          )}
        </section>
      </div>
    </>
  )
}

function CounterSummary({ policies }: { policies: SecurityPolicy[] }) {
  const c = policyCounters(policies)
  return (
    <p className="faint" style={{ marginTop: 12 }}>
      Policy counters: {c.alerts.toLocaleString()} alert{c.alerts === 1 ? '' : 's'} triggered
      {c.lastAlert && (
        <>
          {' '}
          · last alert <span title={formatDate(c.lastAlert)}>{formatRelative(c.lastAlert)}</span>
        </>
      )}
      {c.byType.length > 0 && <> · {c.byType.map(([k, v]) => `${ruleName(k)} ${v}`).join(', ')}</>}
    </p>
  )
}

function PolicyRow({ policy }: { policy: SecurityPolicy }) {
  const phase = policy.status?.phase || 'Pending'
  const enabled = (policy.spec.detectionRules ?? []).filter((r) => r.enabled)
  const sensitivities = [...new Set(enabled.map((r) => r.sensitivity).filter(Boolean))]
  const namespaces = policy.spec.targetNamespaces ?? []
  const counts = Object.entries(policy.status?.detectionCounts ?? {})
  const last = policy.status?.lastAlert
  return (
    <div className="list-row">
      <div className="grow">
        <b>{policy.metadata.name}</b>
        <small>
          {enabled.length} rule{enabled.length === 1 ? '' : 's'} · {policy.status?.alertsTriggered ?? 0} alerts
          {last && (
            <>
              {' '}
              · last <span title={formatDate(last)}>{formatRelative(last)}</span>
            </>
          )}
        </small>
        <small>
          Namespaces: {namespaces.length > 0 ? namespaces.join(', ') : 'none selected'}
          {sensitivities.length > 0 && ` · Sensitivity: ${sensitivities.join(', ')}`}
        </small>
        {counts.length > 0 && <small>{counts.map(([k, v]) => `${ruleName(k)} ${v}`).join(', ')}</small>}
      </div>
      {policy.spec.autoBlock && (
        <span className="pill warn" title="Matching processes are blocked automatically">
          Auto-block
        </span>
      )}
      <span className={`pill ${phaseTone(phase)}`}>{phase}</span>
    </div>
  )
}
