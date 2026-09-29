import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import Progress from '@/components/Progress'
import { EmptyState, ErrorState, Skeleton } from '@/components/StateViews'
import { errorMessage } from '@/lib/errors'
import { formatMoney } from '@/lib/format'
import { alertThreshold, allowedTypes, budgetPercent, budgetedCount, limitLabel, overBudgetAlert } from '@/lib/quotas'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

function tone(pct: number) {
  return pct > 90 ? 'bad' : pct > 70 ? 'warn' : undefined
}

export default function Quotas() {
  useDocumentTitle('Quotas')
  const { data: quotas, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['quotas'],
    queryFn: api.getQuotas,
    refetchInterval: 30000,
  })

  if (isLoading) {
    return (
      <>
        <PageHero eyebrow="Quotas" title="Team quotas." lede="GPU allocations and budget tracking per team" />
        <Skeleton rows={4} />
      </>
    )
  }
  if (isError && !quotas) {
    return (
      <>
        <PageHero eyebrow="Quotas" title="Quotas unavailable." tint="red" />
        <ErrorState title="Could not load quotas." error={error} onRetry={() => refetch()} retrying={isRefetching} />
      </>
    )
  }

  const list = quotas || []
  const over = list.filter(overBudgetAlert).length
  const budgeted = budgetedCount(list)
  const plural = (n: number) => (n === 1 ? '' : 's')

  let headline: string
  let headlineTone: 'ok' | 'warn' | undefined
  if (list.length === 0) {
    headline = 'No team quotas defined.'
  } else if (over > 0) {
    headline = `${over} team${plural(over)} over the budget alert threshold.`
    headlineTone = 'warn'
  } else if (budgeted === 0) {
    headline = `${list.length} team${plural(list.length)}, no budgets configured.`
  } else if (budgeted < list.length) {
    headline = `${budgeted} of ${list.length} team${plural(list.length)} have a budget; none over their alert threshold.`
    headlineTone = 'ok'
  } else {
    headline = `${list.length} team${plural(list.length)}, all within budget.`
    headlineTone = 'ok'
  }

  return (
    <>
      <PageHero
        eyebrow="Quotas"
        title={list.length > 0 && budgeted > 0 && over === 0 ? 'Every team, within budget.' : 'Team quotas.'}
        lede="GPU allocations and budget tracking per team"
      />

      <div className="grid">
        {isError && (
          <div className="span3">
            <ErrorState title="Could not refresh quotas; showing the last data." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          </div>
        )}

        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={headline}
          tone={headlineTone}
          figures={[
            { label: 'Teams', value: list.length },
            { label: 'GPUs allocated', value: list.reduce((n, q) => n + (q.status?.currentUsage?.allocatedGPUs || 0), 0) },
            { label: 'Running jobs', value: list.reduce((n, q) => n + (q.status?.currentUsage?.runningJobs || 0), 0) },
            { label: 'Queued jobs', value: list.reduce((n, q) => n + (q.status?.currentUsage?.queuedJobs || 0), 0) },
            { label: 'Over budget alert', value: over, tone: countTone(over) },
          ]}
        />

        {list.length === 0 && (
          <section className="card span3">
            <EmptyState
              title="No quotas yet"
              action={
                <a className="buttonlike btn-secondary" href="https://github.com/zyvorai/gryvia/tree/main/docs" target="_blank" rel="noreferrer">
                  Read the docs
                </a>
              }
            >
              Quotas come from FabricQuota resources, one per team. Create one with <code className="mono">kubectl apply -f fabricquota.yaml</code> and it appears here.
            </EmptyState>
          </section>
        )}

        {list.map((quota) => {
          const usage = quota.status?.currentUsage
          const budget = quota.status?.budgetStatus
          const specBudget = quota.spec.budget
          const gpuQuota = quota.spec.gpuQuota
          const maxGPUs = gpuQuota?.maxGPUs ?? 0
          const allocated = usage?.allocatedGPUs ?? 0
          const gpuUtilization = maxGPUs > 0 ? (allocated / maxGPUs) * 100 : 0
          const budgetUtilization = budgetPercent(quota) ?? 0
          const types = allowedTypes(quota)

          return (
            <section key={quota.metadata.name} className="card span3">
              <p className="eyebrow">TEAM</p>
              <h2 className="card-title">{quota.spec.team}</h2>
              {quota.spec.priority !== undefined && quota.spec.priority !== null && (
                <p><span className="pill">Priority {quota.spec.priority}</span></p>
              )}

              <div className="stack">
                <div>
                  <div className="row">
                    <span className="muted">GPU allocation</span>
                    <span className="faint num">
                      {allocated} / {maxGPUs} GPUs
                    </span>
                  </div>
                  <Progress value={allocated} max={maxGPUs} tone={tone(gpuUtilization)} label={`${quota.spec.team} GPU allocation`} />
                </div>

                {specBudget && (
                  <div>
                    <div className="row">
                      <span className="muted">Budget usage</span>
                      <span className="faint num">
                        {formatMoney(budget?.spentThisMonth ?? 0)} / {formatMoney(specBudget.monthlyBudget)}
                        {specBudget.hardLimit ? ' · hard limit' : ''}
                      </span>
                    </div>
                    <Progress value={budgetUtilization} tone={tone(budgetUtilization)} label={`${quota.spec.team} budget usage`} />
                    {overBudgetAlert(quota) && (
                      <p className="warning">Budget alert threshold ({alertThreshold(quota)}%) exceeded</p>
                    )}
                  </div>
                )}

                <div className="apple-metric-band">
                  <div>
                    <span>Running jobs</span>
                    <b>{usage?.runningJobs ?? 0}</b>
                  </div>
                  <div>
                    <span>Queued jobs</span>
                    <b>{usage?.queuedJobs ?? 0}</b>
                  </div>
                  <div>
                    <span>GPU hours</span>
                    <b>{(usage?.gpuHours ?? 0).toFixed(1)}</b>
                  </div>
                  {budget && (
                    <div>
                      <span>Projected</span>
                      <b>{formatMoney(budget.projectedSpend)}</b>
                    </div>
                  )}
                </div>

                <div>
                  <p className="eyebrow">ALLOWED GPU TYPES</p>
                  <div className="row">
                    {types === 'All' ? <span className="faint">All</span> : types.map((gpuType) => (
                      <span key={gpuType} className="pill mono">{gpuType}</span>
                    ))}
                  </div>
                  <div className="row">
                    <span>
                      <span className="faint">Max GPUs/job:</span> {limitLabel(gpuQuota?.maxGPUsPerJob)}
                    </span>
                    <span>
                      <span className="faint">Max running:</span> {limitLabel(gpuQuota?.maxRunningJobs)}
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
