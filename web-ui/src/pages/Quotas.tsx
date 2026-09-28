import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'

function tone(pct: number) {
  return pct > 90 ? 'bad' : pct > 70 ? 'warn' : ''
}

export default function Quotas() {
  const { data: quotas, isLoading, isError } = useQuery({
    queryKey: ['quotas'],
    queryFn: api.getQuotas,
    refetchInterval: 30000,
  })

  if (isLoading) return <LoadingSpinner />
  if (isError) {
    return (
      <>
        <PageHero eyebrow="Quotas" title="Quotas unavailable." tint="red" />
        <p className="login-error" role="alert">Failed to load quotas. Please try again.</p>
      </>
    )
  }

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Quotas" title="Every team, within budget." lede="GPU allocations and budget tracking per team" />

      <div className="stack">
        {quotas?.map((quota) => {
          const usage = quota.status?.currentUsage
          const budget = quota.status?.budgetStatus
          const gpuUtilization = usage && quota.spec.gpuQuota.maxGPUs > 0 ? (usage.allocatedGPUs / quota.spec.gpuQuota.maxGPUs) * 100 : 0
          const budgetUtilization = budget ? budget.percentUsed : 0

          return (
            <section key={quota.metadata.name} className="card">
              <div className="stat-head">
                <h2>{quota.spec.team}</h2>
                <span className="pill">Priority {quota.spec.priority}</span>
              </div>

              <div className="stack">
                <div>
                  <div className="stat-head">
                    <span className="muted">GPU Allocation</span>
                    <span className="faint">
                      {usage?.allocatedGPUs || 0} / {quota.spec.gpuQuota.maxGPUs} GPUs
                    </span>
                  </div>
                  <div className={`progress ${tone(gpuUtilization)}`} style={{ marginTop: 8 }}>
                    <span style={{ width: `${Math.min(gpuUtilization, 100)}%` }} />
                  </div>
                </div>

                {quota.spec.budget && budget && (
                  <div>
                    <div className="stat-head">
                      <span className="muted">Budget Usage</span>
                      <span className="faint">
                        ${budget.spentThisMonth.toFixed(2)} / ${quota.spec.budget.monthlyBudget.toFixed(2)}
                      </span>
                    </div>
                    <div className={`progress ${tone(budgetUtilization)}`} style={{ marginTop: 8 }}>
                      <span style={{ width: `${Math.min(budgetUtilization, 100)}%` }} />
                    </div>
                    {budgetUtilization > quota.spec.budget.alertThreshold && (
                      <p className="text-warn" style={{ marginTop: 8 }}>Budget alert threshold exceeded</p>
                    )}
                  </div>
                )}

                <div className="apple-metric-band">
                  <div>
                    <span>Running Jobs</span>
                    <b>{usage?.runningJobs || 0}</b>
                  </div>
                  <div>
                    <span>Queued Jobs</span>
                    <b>{usage?.queuedJobs || 0}</b>
                  </div>
                  <div>
                    <span>GPU Hours</span>
                    <b>{usage?.gpuHours?.toFixed(1) || 0}</b>
                  </div>
                  {budget && (
                    <div>
                      <span>Projected</span>
                      <b>${budget.projectedSpend.toFixed(0)}</b>
                    </div>
                  )}
                </div>

                <div>
                  <h3>Allowed GPU Types</h3>
                  <div className="row">
                    {quota.spec.gpuQuota?.allowedGPUTypes?.map((gpuType) => (
                      <span key={gpuType} className="pill mono">{gpuType}</span>
                    )) || <span className="faint">All</span>}
                  </div>
                  <div className="row" style={{ marginTop: 12, gap: 24 }}>
                    <span>
                      <span className="faint">Max GPUs/Job:</span> {quota.spec.gpuQuota.maxGPUsPerJob}
                    </span>
                    <span>
                      <span className="faint">Max Running:</span> {quota.spec.gpuQuota.maxRunningJobs}
                    </span>
                  </div>
                </div>
              </div>
            </section>
          )
        })}
      </div>
    </div>
  )
}
