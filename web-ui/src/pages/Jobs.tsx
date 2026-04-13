import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, RefreshCw, Briefcase, CheckCircle, Clock, XCircle } from 'lucide-react'
import { api } from '@/lib/api'
import JobsTable from '@/components/JobsTable'
import LoadingSpinner from '@/components/LoadingSpinner'

const statusConfig: Record<string, { icon: React.ElementType; gradient: string; iconColor: string }> = {
  Running: { icon: Briefcase, gradient: 'stat-card-blue', iconColor: 'text-blue-400' },
  Pending: { icon: Clock, gradient: 'stat-card-orange', iconColor: 'text-yellow-400' },
  Completed: { icon: CheckCircle, gradient: 'stat-card-green', iconColor: 'text-green-400' },
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
          <h2 className="text-2xl font-bold text-gradient-blue">AI Jobs</h2>
          <p className="text-sm text-slate-400 mt-1">
            Manage training and inference workloads
          </p>
        </div>
        <div className="flex gap-3">
          <button
            onClick={() => refetch()}
            disabled={isRefetching}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 border border-slate-600 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${isRefetching ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <Link
            to="/jobs/new"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors"
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
            <div key={status} className={`${cfg.gradient} rounded-xl border border-slate-700/50 p-5 card-glow`}>
              <div className="flex items-center justify-between mb-3">
                <div className="w-10 h-10 rounded-lg bg-slate-900/50 flex items-center justify-center">
                  <Icon className={`h-5 w-5 ${cfg.iconColor}`} />
                </div>
              </div>
              <div className="text-2xl font-bold text-white">{count}</div>
              <div className="text-xs text-slate-400 mt-1">{status}</div>
            </div>
          )
        })}
      </div>

      {/* Jobs Table */}
      <div className="bg-slate-800/50 rounded-xl border border-slate-700/50">
        <div className="px-5 py-4 border-b border-slate-700/50">
          <div className="flex items-center gap-2">
            <Briefcase className="h-4 w-4 text-blue-400" />
            <h3 className="text-sm font-semibold text-white">All Jobs</h3>
            <span className="text-xs text-slate-500 ml-auto">{jobs?.length || 0} total</span>
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
