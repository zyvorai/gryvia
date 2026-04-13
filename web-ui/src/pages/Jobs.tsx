import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, RefreshCw, Briefcase, CheckCircle, Clock, XCircle } from 'lucide-react'
import { api } from '@/lib/api'
import JobsTable from '@/components/JobsTable'
import LoadingSpinner from '@/components/LoadingSpinner'

const statusConfig: Record<string, { icon: React.ElementType; gradient: string; iconColor: string }> = {
  Running: { icon: Briefcase, gradient: 'stat-card-blue', iconColor: 'text-[#7ecbf5]' },
  Pending: { icon: Clock, gradient: 'stat-card-orange', iconColor: 'text-[#fbbf24]' },
  Completed: { icon: CheckCircle, gradient: 'stat-card-green', iconColor: 'text-emerald-400' },
  Failed: { icon: XCircle, gradient: 'stat-card-red', iconColor: 'text-red-400' },
}

export default function Jobs() {
  const { data: jobs, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 15000,
  })

  if (isError) return (
    <div className="text-center py-12">
      <p className="text-red-400">Failed to load jobs. Please try again.</p>
    </div>
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gradient-copper">AI Jobs</h2>
          <p className="text-sm text-[#5a7a9e] mt-1">
            Manage training and inference workloads
          </p>
        </div>
        <div className="flex gap-3">
          <button
            onClick={() => refetch()}
            disabled={isRefetching}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-[#8ba4c0] transition-all duration-200 disabled:opacity-50 hover:text-[#c0cce0]"
            style={{
              border: '1px solid rgba(192,204,224,0.1)',
              background: 'rgba(21,29,40,0.5)',
            }}
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isRefetching ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <Link
            to="/jobs/new"
            className="inline-flex items-center gap-1.5 px-3.5 py-1.5 btn-copper text-sm font-medium rounded-lg"
          >
            <Plus className="h-3.5 w-3.5" />
            Submit Job
          </Link>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {Object.entries(statusConfig).map(([status, cfg]) => {
          const Icon = cfg.icon
          const count = jobs?.filter(j => j.status?.phase === status).length || 0
          return (
            <div key={status} className={`${cfg.gradient} rounded-xl p-5 card-glow`}
              style={{
                border: '1px solid rgba(192,204,224,0.06)',
                boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
              }}
            >
              <div className="flex items-center justify-between mb-3">
                <div className="w-10 h-10 rounded-lg flex items-center justify-center"
                  style={{
                    background: 'rgba(10,14,20,0.5)',
                    border: '1px solid rgba(192,204,224,0.04)',
                    boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
                  }}
                >
                  <Icon className={`h-5 w-5 ${cfg.iconColor}`} />
                </div>
              </div>
              <div className="text-2xl font-bold text-[#e8ecf1]">{count}</div>
              <div className="text-xs text-[#5a7a9e] mt-1">{status}</div>
            </div>
          )
        })}
      </div>

      {/* Jobs Table */}
      <div className="rounded-xl" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        <div className="px-5 py-4" style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
          <div className="flex items-center gap-2">
            <Briefcase className="h-4 w-4 text-[#e8a87c]" />
            <h3 className="text-sm font-semibold text-[#e8ecf1]">All Jobs</h3>
            <span className="text-xs text-[#5a7a9e] ml-auto">{jobs?.length || 0} total</span>
          </div>
        </div>
        <div className="p-4">
          {isLoading ? (
            <LoadingSpinner />
          ) : (
            <JobsTable jobs={jobs || []} />
          )}
        </div>
      </div>
    </div>
  )
}
