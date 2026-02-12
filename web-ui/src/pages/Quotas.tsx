import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import { Users, Cpu, DollarSign, AlertTriangle } from 'lucide-react'

export default function Quotas() {
  const { data: quotas, isLoading } = useQuery({
    queryKey: ['quotas'],
    queryFn: api.getQuotas,
    refetchInterval: 30000,
  })

  if (isLoading) {
    return <LoadingSpinner />
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold text-gray-900">Team Quotas</h2>
        <p className="mt-1 text-sm text-gray-500">
          GPU allocations and budget tracking per team
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6">
        {quotas?.map((quota) => {
          const usage = quota.status?.currentUsage
          const budget = quota.status?.budgetStatus
          const gpuUtilization = usage ? (usage.allocatedGPUs / quota.spec.gpuQuota.maxGPUs) * 100 : 0
          const budgetUtilization = budget ? budget.percentUsed : 0

          return (
            <div key={quota.metadata.name} className="bg-white shadow rounded-lg overflow-hidden">
              <div className="px-6 py-4 border-b border-gray-200 bg-gray-50">
                <div className="flex items-center justify-between">
                  <div className="flex items-center">
                    <Users className="h-5 w-5 text-gray-400 mr-2" />
                    <h3 className="text-lg font-medium text-gray-900">{quota.spec.team}</h3>
                  </div>
                  <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-blue-100 text-blue-800">
                    Priority {quota.spec.priority}
                  </span>
                </div>
              </div>

              <div className="px-6 py-4">
                {/* GPU Usage */}
                <div className="mb-6">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center">
                      <Cpu className="h-4 w-4 text-gray-400 mr-2" />
                      <span className="text-sm font-medium text-gray-700">GPU Allocation</span>
                    </div>
                    <span className="text-sm text-gray-500">
                      {usage?.allocatedGPUs || 0} / {quota.spec.gpuQuota.maxGPUs} GPUs
                    </span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2">
                    <div
                      className={`h-2 rounded-full ${
                        gpuUtilization > 90 ? 'bg-red-600' : gpuUtilization > 70 ? 'bg-yellow-500' : 'bg-green-500'
                      }`}
                      style={{ width: `${Math.min(gpuUtilization, 100)}%` }}
                    />
                  </div>
                </div>

                {/* Budget Usage */}
                {quota.spec.budget && budget && (
                  <div className="mb-6">
                    <div className="flex items-center justify-between mb-2">
                      <div className="flex items-center">
                        <DollarSign className="h-4 w-4 text-gray-400 mr-2" />
                        <span className="text-sm font-medium text-gray-700">Budget Usage</span>
                      </div>
                      <span className="text-sm text-gray-500">
                        ${budget.spentThisMonth.toFixed(2)} / ${quota.spec.budget.monthlyBudget.toFixed(2)}
                      </span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-2">
                      <div
                        className={`h-2 rounded-full ${
                          budgetUtilization > 90 ? 'bg-red-600' : budgetUtilization > 70 ? 'bg-yellow-500' : 'bg-green-500'
                        }`}
                        style={{ width: `${Math.min(budgetUtilization, 100)}%` }}
                      />
                    </div>
                    {budgetUtilization > quota.spec.budget.alertThreshold && (
                      <div className="mt-2 flex items-center text-sm text-yellow-700">
                        <AlertTriangle className="h-4 w-4 mr-1" />
                        Budget alert threshold exceeded
                      </div>
                    )}
                  </div>
                )}

                {/* Stats Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
                  <div>
                    <dt className="text-xs font-medium text-gray-500 uppercase">Running Jobs</dt>
                    <dd className="mt-1 text-2xl font-semibold text-gray-900">{usage?.runningJobs || 0}</dd>
                  </div>
                  <div>
                    <dt className="text-xs font-medium text-gray-500 uppercase">Queued Jobs</dt>
                    <dd className="mt-1 text-2xl font-semibold text-gray-900">{usage?.queuedJobs || 0}</dd>
                  </div>
                  <div>
                    <dt className="text-xs font-medium text-gray-500 uppercase">GPU Hours</dt>
                    <dd className="mt-1 text-2xl font-semibold text-gray-900">{usage?.gpuHours?.toFixed(1) || 0}</dd>
                  </div>
                  {budget && (
                    <div>
                      <dt className="text-xs font-medium text-gray-500 uppercase">Projected Spend</dt>
                      <dd className="mt-1 text-2xl font-semibold text-gray-900">${budget.projectedSpend.toFixed(0)}</dd>
                    </div>
                  )}
                </div>

                {/* Allowed GPUs */}
                <div className="mt-6 pt-6 border-t border-gray-200">
                  <h4 className="text-sm font-medium text-gray-700 mb-2">Allowed GPU Types</h4>
                  <div className="flex flex-wrap gap-2">
                    {quota.spec.gpuQuota.allowedGPUTypes.map((gpuType) => (
                      <span
                        key={gpuType}
                        className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
                      >
                        {gpuType}
                      </span>
                    ))}
                  </div>
                  <div className="mt-4 grid grid-cols-2 gap-4 text-sm">
                    <div>
                      <span className="text-gray-500">Max GPUs per Job:</span>{' '}
                      <span className="font-medium text-gray-900">{quota.spec.gpuQuota.maxGPUsPerJob}</span>
                    </div>
                    <div>
                      <span className="text-gray-500">Max Running Jobs:</span>{' '}
                      <span className="font-medium text-gray-900">{quota.spec.gpuQuota.maxRunningJobs}</span>
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
