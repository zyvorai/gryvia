import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

function tone(pct: number) {
  return pct > 90 ? 'bad' : pct > 70 ? 'warn' : ''
}

export default function Quotas() {
  const { data: quotas, isLoading, isError, dataUpdatedAt } = useQuery({
    queryKey: ['quotas'],
    queryFn: api.getQuotas,
    refetchInterval: 30000,
  })

  if (isLoading) return <LoadingSpinner />
  if (isError) {
    return (
      <>
        <PageHero eyebrow="Quotas" title="Quotas unavailable." tint="red" />
        <p className="warning" role="alert">Failed to load quotas. Please try again.</p>
      </>
    )
  }

  const list = quotas || []
  const over = list.filter((q) => {
    const b = q.status?.budgetStatus
    return b && q.spec.budget && b.percentUsed > q.spec.budget.alertThreshold
  }).length

  return (
    <>
      <PageHero eyebrow="Quotas" title="Every team, within budget." lede="GPU allocations and budget tracking per team" />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={over > 0 ? `${over} team${over === 1 ? '' : 's'} over the budget alert threshold.` : `${list.length} team${list.length === 1 ? '' : 's'}, all within budget.`}
          tone={over > 0 ? 'warn' : 'ok'}
          figures={[
            { label: 'Teams', value: list.length },
            { label: 'GPUs allocated', value: list.reduce((n, q) => n + (q.status?.currentUsage?.allocatedGPUs || 0), 0) },
            { label: 'Running jobs', value: list.reduce((n, q) => n + (q.status?.currentUsage?.runningJobs || 0), 0) },
            { label: 'Queued jobs', value: list.reduce((n, q) => n + (q.status?.currentUsage?.queuedJobs || 0), 0) },
            { label: 'Over budget alert', value: over, tone: countTone(over) },
          ]}
        />

        {quotas?.map((quota) => {
          const usage = quota.status?.currentUsage
          const budget = quota.status?.budgetStatus
          const gpuUtilization = usage && quota.spec.gpuQuota.maxGPUs > 0 ? (usage.allocatedGPUs / quota.spec.gpuQuota.maxGPUs) * 100 : 0
          const budgetUtilization = budget ? budget.percentUsed : 0

          return (
            <section key={quota.metadata.name} className="card span3">
              <p className="eyebrow">TEAM</p>
              <h2 className="card-title">{quota.spec.team}</h2>
              <p><span className="pill">Priority {quota.spec.priority}</span></p>

              <div className="stack">
                <div>
                  <div className="row">
                    <span className="muted">GPU Allocation</span>
                    <span className="faint">
                      {usage?.allocatedGPUs || 0} / {quota.spec.gpuQuota.maxGPUs} GPUs
                    </span>
                  </div>
                  <div className={`progress ${tone(gpuUtilization)}`}>
                    <span style={{ width: `${Math.min(gpuUtilization, 100)}%` }} />
                  </div>
                </div>

                {quota.spec.budget && budget && (
                  <div>
                    <div className="row">
                      <span className="muted">Budget Usage</span>
                      <span className="faint">
                        ${budget.spentThisMonth.toFixed(2)} / ${quota.spec.budget.monthlyBudget.toFixed(2)}
                      </span>
                    </div>
                    <div className={`progress ${tone(budgetUtilization)}`}>
                      <span style={{ width: `${Math.min(budgetUtilization, 100)}%` }} />
                    </div>
                    {budgetUtilization > quota.spec.budget.alertThreshold && (
                      <p className="warning">Budget alert threshold exceeded</p>
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
                  <p className="eyebrow">ALLOWED GPU TYPES</p>
                  <div className="row">
                    {quota.spec.gpuQuota?.allowedGPUTypes?.map((gpuType) => (
                      <span key={gpuType} className="pill mono">{gpuType}</span>
                    )) || <span className="faint">All</span>}
                  </div>
                  <div className="row">
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
    </>
  )
}
