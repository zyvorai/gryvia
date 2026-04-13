import { Link } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { FabricAIJob } from '@/types'

interface JobsTableProps {
  jobs: FabricAIJob[]
  compact?: boolean
}

const statusStyles: Record<string, { bg: string; color: string }> = {
  Running: { bg: 'rgba(95,168,211,0.08)', color: '#7ecbf5' },
  Pending: { bg: 'rgba(251,191,36,0.08)', color: '#fbbf24' },
  Queued: { bg: 'rgba(251,191,36,0.08)', color: '#fbbf24' },
  Completed: { bg: 'rgba(34,197,94,0.08)', color: '#4ade80' },
  Succeeded: { bg: 'rgba(34,197,94,0.08)', color: '#4ade80' },
  Failed: { bg: 'rgba(239,68,68,0.08)', color: '#f87171' },
}

export default function JobsTable({ jobs, compact = false }: JobsTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full">
        <thead>
          <tr style={{ borderBottom: '1px solid rgba(192,204,224,0.06)' }}>
            <th className="px-4 py-3 text-left text-xs font-medium text-[#5a7a9e] uppercase tracking-wider">Name</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-[#5a7a9e] uppercase tracking-wider">Framework</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-[#5a7a9e] uppercase tracking-wider">GPUs</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-[#5a7a9e] uppercase tracking-wider">Status</th>
            {!compact && (
              <th className="px-4 py-3 text-left text-xs font-medium text-[#5a7a9e] uppercase tracking-wider">Age</th>
            )}
          </tr>
        </thead>
        <tbody>
          {jobs.length === 0 ? (
            <tr>
              <td colSpan={compact ? 4 : 5} className="px-4 py-8 text-center text-sm text-[#5a7a9e]">
                No jobs found
              </td>
            </tr>
          ) : (
            jobs.map((job) => {
              const status = statusStyles[job.status?.phase ?? ''] || { bg: 'rgba(90,122,158,0.08)', color: '#5a7a9e' }
              return (
                <tr key={job.metadata.name} className="table-row-hover" style={{ borderBottom: '1px solid rgba(192,204,224,0.04)' }}>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <Link
                      to={`/jobs/${job.metadata.name}`}
                      className="text-sm font-medium text-[#e8a87c] hover:text-[#f0c4a0] transition-colors"
                    >
                      {job.metadata.name}
                    </Link>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-[#8ba4c0]">
                    {job.spec.framework}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-[#8ba4c0]">
                    {job.spec.resources.gpuCount} x {job.spec.resources.gpuType}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <span className="px-2 py-0.5 text-[10px] font-medium rounded-full"
                      style={{ background: status.bg, color: status.color }}
                    >
                      {job.status?.phase || 'Unknown'}
                    </span>
                  </td>
                  {!compact && (
                    <td className="px-4 py-3 whitespace-nowrap text-xs text-[#5a7a9e]">
                      {job.metadata.creationTimestamp
                        ? formatDistanceToNow(new Date(job.metadata.creationTimestamp), { addSuffix: true })
                        : '-'}
                    </td>
                  )}
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </div>
  )
}
