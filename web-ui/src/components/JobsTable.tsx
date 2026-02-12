import { Link } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import clsx from 'clsx'
import { FabricAIJob } from '@/types'

interface JobsTableProps {
  jobs: FabricAIJob[]
  compact?: boolean
}

const statusColors = {
  Running: 'bg-green-100 text-green-800',
  Pending: 'bg-yellow-100 text-yellow-800',
  Queued: 'bg-yellow-100 text-yellow-800',
  Completed: 'bg-blue-100 text-blue-800',
  Failed: 'bg-red-100 text-red-800',
}

export default function JobsTable({ jobs, compact = false }: JobsTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Name
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Framework
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              GPUs
            </th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            {!compact && (
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Age
              </th>
            )}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">
          {jobs.length === 0 ? (
            <tr>
              <td colSpan={compact ? 4 : 5} className="px-6 py-4 text-center text-sm text-gray-500">
                No jobs found
              </td>
            </tr>
          ) : (
            jobs.map((job) => (
              <tr key={job.metadata.name} className="hover:bg-gray-50">
                <td className="px-6 py-4 whitespace-nowrap">
                  <Link
                    to={`/jobs/${job.metadata.name}`}
                    className="text-sm font-medium text-blue-600 hover:text-blue-900"
                  >
                    {job.metadata.name}
                  </Link>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                  {job.spec.framework}
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                  {job.spec.resources.gpuCount} × {job.spec.resources.gpuType}
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <span
                    className={clsx(
                      'px-2 inline-flex text-xs leading-5 font-semibold rounded-full',
                      statusColors[job.status?.phase as keyof typeof statusColors] || 'bg-gray-100 text-gray-800'
                    )}
                  >
                    {job.status?.phase || 'Unknown'}
                  </span>
                </td>
                {!compact && (
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
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
