import { Link } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { FabricAIJob } from '@/types'

interface JobsTableProps {
  jobs: FabricAIJob[]
  compact?: boolean
}

const statusColors: Record<string, string> = {
  Running: 'bg-blue-500/10 text-blue-400',
  Pending: 'bg-yellow-500/10 text-yellow-400',
  Queued: 'bg-yellow-500/10 text-yellow-400',
  Completed: 'bg-green-500/10 text-green-400',
  Succeeded: 'bg-green-500/10 text-green-400',
  Failed: 'bg-red-500/10 text-red-400',
}

export default function JobsTable({ jobs, compact = false }: JobsTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full">
        <thead>
          <tr className="border-b border-slate-700/50">
            <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Name</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Framework</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">GPUs</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Status</th>
            {!compact && (
              <th className="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Age</th>
            )}
          </tr>
        </thead>
        <tbody>
          {jobs.length === 0 ? (
            <tr>
              <td colSpan={compact ? 4 : 5} className="px-4 py-8 text-center text-sm text-slate-500">
                No jobs found
              </td>
            </tr>
          ) : (
            jobs.map((job) => (
              <tr key={job.metadata.name} className="table-row-hover border-b border-slate-700/30">
                <td className="px-4 py-3 whitespace-nowrap">
                  <Link
                    to={`/jobs/${job.metadata.name}`}
                    className="text-sm font-medium text-blue-400 hover:text-blue-300"
                  >
                    {job.metadata.name}
                  </Link>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-slate-300">
                  {job.spec.framework}
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-slate-300">
                  {job.spec.resources.gpuCount} x {job.spec.resources.gpuType}
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span className={`px-2 py-0.5 text-[10px] font-medium rounded-full ${
                    statusColors[job.status?.phase] || 'bg-slate-500/10 text-slate-400'
                  }`}>
                    {job.status?.phase || 'Unknown'}
                  </span>
                </td>
                {!compact && (
                  <td className="px-4 py-3 whitespace-nowrap text-xs text-slate-500">
                    {job.metadata.creationTimestamp
                      ? formatDistanceToNow(new Date(job.metadata.creationTimestamp), { addSuffix: true })
                      : '-'}
                  </td>
                )}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
