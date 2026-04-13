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
        <h2 className="text-2xl font-bold text-gradient-copper">Team Quotas</h2>
        <p className="text-sm text-[#5a7a9e] mt-1">GPU allocations and budget tracking per team</p>
      </div>

      <div className="grid grid-cols-1 gap-4">
        {quotas?.map((quota) => {
          const usage = quota.status?.currentUsage
          const budget = quota.status?.budgetStatus
          const gpuUtilization = usage && quota.spec.gpuQuota.maxGPUs > 0 ? (usage.allocatedGPUs / quota.spec.gpuQuota.maxGPUs) * 100 : 0
          const budgetUtilization = budget ? budget.percentUsed : 0

          return (
            <div key={quota.metadata.name} className="rounded-xl overflow-hidden" style={{
              background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
              border: '1px solid rgba(192,204,224,0.06)',
              boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
            }}>
              <div className="px-5 py-4" style={{ borderBottom: '1px solid rgba(192,204,224,0.04)' }}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Users className="h-4 w-4 text-[#7ecbf5]" />
                    <h3 className="text-sm font-semibold text-[#e8ecf1]">{quota.spec.team}</h3>
                  </div>
                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-full"
                    style={{ background: 'rgba(95,168,211,0.08)', color: '#7ecbf5' }}
                  >
                    Priority {quota.spec.priority}
                  </span>
                </div>
              </div>

              <div className="px-5 py-4">
                {/* GPU Usage */}
                <div className="mb-5">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <Cpu className="h-3.5 w-3.5 text-[#5a7a9e]" />
                      <span className="text-xs font-medium text-[#8ba4c0]">GPU Allocation</span>
                    </div>
                    <span className="text-xs text-[#5a7a9e]">
                      {usage?.allocatedGPUs || 0} / {quota.spec.gpuQuota.maxGPUs} GPUs
                    </span>
                  </div>
                  <div className="h-1.5 rounded-full overflow-hidden" style={{
                    background: 'rgba(10,14,20,0.6)',
                    boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.4)',
                  }}>
                    <div className="h-full rounded-full" style={{
                      width: `${Math.min(gpuUtilization, 100)}%`,
                      background: gpuUtilization > 90
                        ? 'linear-gradient(90deg, #7f1d1d, #ef4444)'
                        : gpuUtilization > 70
                        ? 'linear-gradient(90deg, #7c3a1a, #fbbf24)'
                        : 'linear-gradient(90deg, #166534, #4ade80)',
                      boxShadow: '0 0 6px rgba(192,204,224,0.08)',
                    }} />
                  </div>
                </div>

                {/* Budget Usage */}
                {quota.spec.budget && budget && (
                  <div className="mb-5">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <DollarSign className="h-3.5 w-3.5 text-[#5a7a9e]" />
                        <span className="text-xs font-medium text-[#8ba4c0]">Budget Usage</span>
                      </div>
                      <span className="text-xs text-[#5a7a9e]">
                        ${budget.spentThisMonth.toFixed(2)} / ${quota.spec.budget.monthlyBudget.toFixed(2)}
                      </span>
                    </div>
                    <div className="h-1.5 rounded-full overflow-hidden" style={{
                      background: 'rgba(10,14,20,0.6)',
                      boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.4)',
                    }}>
                      <div className="h-full rounded-full" style={{
                        width: `${Math.min(budgetUtilization, 100)}%`,
                        background: budgetUtilization > 90
                          ? 'linear-gradient(90deg, #7f1d1d, #ef4444)'
                          : budgetUtilization > 70
                          ? 'linear-gradient(90deg, #7c3a1a, #fbbf24)'
                          : 'linear-gradient(90deg, #166534, #4ade80)',
                        boxShadow: '0 0 6px rgba(192,204,224,0.08)',
                      }} />
                    </div>
                    {budgetUtilization > quota.spec.budget.alertThreshold && (
                      <div className="mt-2 flex items-center text-xs text-[#fbbf24]">
                        <AlertTriangle className="h-3.5 w-3.5 mr-1" />
                        Budget alert threshold exceeded
                      </div>
                    )}
                  </div>
                )}

                {/* Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-4">
                  <div className="p-2.5 rounded-lg" style={{
                    background: 'rgba(10,14,20,0.5)',
                    border: '1px solid rgba(192,204,224,0.04)',
                    boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                  }}>
                    <div className="text-[10px] text-[#5a7a9e] uppercase">Running Jobs</div>
                    <div className="text-lg font-bold text-[#e8ecf1]">{usage?.runningJobs || 0}</div>
                  </div>
                  <div className="p-2.5 rounded-lg" style={{
                    background: 'rgba(10,14,20,0.5)',
                    border: '1px solid rgba(192,204,224,0.04)',
                    boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                  }}>
                    <div className="text-[10px] text-[#5a7a9e] uppercase">Queued Jobs</div>
                    <div className="text-lg font-bold text-[#e8ecf1]">{usage?.queuedJobs || 0}</div>
                  </div>
                  <div className="p-2.5 rounded-lg" style={{
                    background: 'rgba(10,14,20,0.5)',
                    border: '1px solid rgba(192,204,224,0.04)',
                    boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                  }}>
                    <div className="text-[10px] text-[#5a7a9e] uppercase">GPU Hours</div>
                    <div className="text-lg font-bold text-[#e8ecf1]">{usage?.gpuHours?.toFixed(1) || 0}</div>
                  </div>
                  {budget && (
                    <div className="p-2.5 rounded-lg" style={{
                      background: 'rgba(10,14,20,0.5)',
                      border: '1px solid rgba(192,204,224,0.04)',
                      boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                    }}>
                      <div className="text-[10px] text-[#5a7a9e] uppercase">Projected</div>
                      <div className="text-lg font-bold text-[#e8ecf1]">${budget.projectedSpend.toFixed(0)}</div>
                    </div>
                  )}
                </div>

                {/* Allowed GPUs */}
                <div className="mt-4 pt-4" style={{ borderTop: '1px solid rgba(192,204,224,0.04)' }}>
                  <span className="text-xs font-medium text-[#8090a8]">Allowed GPU Types</span>
                  <div className="flex flex-wrap gap-2 mt-2">
                    {quota.spec.gpuQuota?.allowedGPUTypes?.map((gpuType) => (
                      <span key={gpuType} className="text-[10px] px-2 py-0.5 rounded-full text-[#b0c4d8] font-mono"
                        style={{
                          background: 'rgba(10,14,20,0.5)',
                          border: '1px solid rgba(192,204,224,0.06)',
                        }}
                      >
                        {gpuType}
                      </span>
                    )) || <span className="text-[10px] text-[#5a7a9e]">All</span>}
                  </div>
                  <div className="mt-3 grid grid-cols-2 gap-3 text-xs">
                    <div>
                      <span className="text-[#5a7a9e]">Max GPUs/Job:</span>{' '}
                      <span className="font-medium text-[#b0c4d8]">{quota.spec.gpuQuota.maxGPUsPerJob}</span>
                    </div>
                    <div>
                      <span className="text-[#5a7a9e]">Max Running:</span>{' '}
                      <span className="font-medium text-[#b0c4d8]">{quota.spec.gpuQuota.maxRunningJobs}</span>
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
