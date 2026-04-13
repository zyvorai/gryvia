import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import { Users, Cpu, DollarSign, AlertTriangle } from 'lucide-react'

export default function Quotas() {
  const { data: quotas, isLoading, isError } = useQuery({
    queryKey: ['quotas'],
    queryFn: api.getQuotas,
    refetchInterval: 30000,
  })

  if (isLoading) return <LoadingSpinner />
  if (isError) return (
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load quotas. Please try again.</p>
    </div>
  )

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-gradient-blue">Team Quotas</h2>
        <p className="text-sm text-slate-400 mt-1">GPU allocations and budget tracking per team</p>
      </div>

      <div className="grid grid-cols-1 gap-4">
        {quotas?.map((quota) => {
          const usage = quota.status?.currentUsage
          const budget = quota.status?.budgetStatus
          const gpuUtilization = usage && quota.spec.gpuQuota.maxGPUs > 0 ? (usage.allocatedGPUs / quota.spec.gpuQuota.maxGPUs) * 100 : 0
          const budgetUtilization = budget ? budget.percentUsed : 0

          return (
            <div key={quota.metadata.name} className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
              <div className="px-5 py-4 border-b border-slate-700/30">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Users className="h-4 w-4 text-blue-400" />
                    <h3 className="text-sm font-semibold text-white">{quota.spec.team}</h3>
                  </div>
                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400">
                    Priority {quota.spec.priority}
                  </span>
                </div>
              </div>

              <div className="px-5 py-4">
                {/* GPU Usage */}
                <div className="mb-5">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <Cpu className="h-3.5 w-3.5 text-slate-400" />
                      <span className="text-xs font-medium text-slate-300">GPU Allocation</span>
                    </div>
                    <span className="text-xs text-slate-400">
                      {usage?.allocatedGPUs || 0} / {quota.spec.gpuQuota.maxGPUs} GPUs
                    </span>
                  </div>
                  <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full ${gpuUtilization > 90 ? 'bg-red-500' : gpuUtilization > 70 ? 'bg-yellow-500' : 'bg-green-500'}`}
                      style={{ width: `${Math.min(gpuUtilization, 100)}%` }}
                    />
                  </div>
                </div>

                {/* Budget Usage */}
                {quota.spec.budget && budget && (
                  <div className="mb-5">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <DollarSign className="h-3.5 w-3.5 text-slate-400" />
                        <span className="text-xs font-medium text-slate-300">Budget Usage</span>
                      </div>
                      <span className="text-xs text-slate-400">
                        ${budget.spentThisMonth.toFixed(2)} / ${quota.spec.budget.monthlyBudget.toFixed(2)}
                      </span>
                    </div>
                    <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full rounded-full ${budgetUtilization > 90 ? 'bg-red-500' : budgetUtilization > 70 ? 'bg-yellow-500' : 'bg-green-500'}`}
                        style={{ width: `${Math.min(budgetUtilization, 100)}%` }}
                      />
                    </div>
                    {budgetUtilization > quota.spec.budget.alertThreshold && (
                      <div className="mt-2 flex items-center text-xs text-yellow-400">
                        <AlertTriangle className="h-3.5 w-3.5 mr-1" />
                        Budget alert threshold exceeded
                      </div>
                    )}
                  </div>
                )}

                {/* Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-4">
                  <div className="p-2.5 bg-slate-900/30 rounded-lg">
                    <div className="text-[10px] text-slate-500 uppercase">Running Jobs</div>
                    <div className="text-lg font-bold text-white">{usage?.runningJobs || 0}</div>
                  </div>
                  <div className="p-2.5 bg-slate-900/30 rounded-lg">
                    <div className="text-[10px] text-slate-500 uppercase">Queued Jobs</div>
                    <div className="text-lg font-bold text-white">{usage?.queuedJobs || 0}</div>
                  </div>
                  <div className="p-2.5 bg-slate-900/30 rounded-lg">
                    <div className="text-[10px] text-slate-500 uppercase">GPU Hours</div>
                    <div className="text-lg font-bold text-white">{usage?.gpuHours?.toFixed(1) || 0}</div>
                  </div>
                  {budget && (
                    <div className="p-2.5 bg-slate-900/30 rounded-lg">
                      <div className="text-[10px] text-slate-500 uppercase">Projected</div>
                      <div className="text-lg font-bold text-white">${budget.projectedSpend.toFixed(0)}</div>
                    </div>
                  )}
                </div>

                {/* Allowed GPUs */}
                <div className="mt-4 pt-4 border-t border-slate-700/30">
                  <span className="text-xs font-medium text-slate-400">Allowed GPU Types</span>
                  <div className="flex flex-wrap gap-2 mt-2">
                    {quota.spec.gpuQuota?.allowedGPUTypes?.map((gpuType) => (
                      <span key={gpuType} className="text-[10px] px-2 py-0.5 bg-slate-900/50 border border-slate-700/30 rounded-full text-slate-300 font-mono">
                        {gpuType}
                      </span>
                    )) || <span className="text-[10px] text-slate-400">All</span>}
                  </div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-xs">
                    <div>
                      <span className="text-slate-500">Max GPUs/Job:</span>{' '}
                      <span className="font-medium text-slate-300">{quota.spec.gpuQuota.maxGPUsPerJob}</span>
                    </div>
                    <div>
                      <span className="text-slate-500">Max Running:</span>{' '}
                      <span className="font-medium text-slate-300">{quota.spec.gpuQuota.maxRunningJobs}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
