import { Link } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { phaseTone } from '@/lib/phase'
import { FabricAIJob } from '@/types'

interface JobsTableProps {
  jobs: FabricAIJob[]
  compact?: boolean
}

export default function JobsTable({ jobs, compact = false }: JobsTableProps) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Framework</th>
            <th>GPUs</th>
            <th>Status</th>
            {!compact && <th>Age</th>}
          </tr>
        </thead>
        <tbody>
          {jobs.length === 0 ? (
            <tr>
              <td colSpan={compact ? 4 : 5} className="faint" style={{ textAlign: 'center' }}>
                No jobs found
              </td>
            </tr>
          ) : (
            jobs.map((job) => (
              <tr key={job.metadata.name}>
                <td>
                  <Link to={`/jobs/${job.metadata.name}`} className="card-link">
                    {job.metadata.name}
                  </Link>
                </td>
                <td>{job.spec.framework}</td>
                <td>
                  {job.spec.resources.gpuCount} × {job.spec.resources.gpuType}
                </td>
                <td>
                  <span className={`pill ${phaseTone(job.status?.phase)}`}>{job.status?.phase || 'Unknown'}</span>
                </td>
                {!compact && (
                  <td className="faint">
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
