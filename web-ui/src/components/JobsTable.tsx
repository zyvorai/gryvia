import { Link, useNavigate } from 'react-router-dom'
import { phaseTone } from '@/lib/phase'
import { formatDate, formatRelative } from '@/lib/format'
import type { FabricAIJob } from '@/types'
import { jobFramework, jobGpus } from '@/lib/jobs'
import { EmptyState } from '@/components/StateViews'

interface JobsTableProps {
  jobs: FabricAIJob[]
  compact?: boolean
}

export default function JobsTable({ jobs, compact = false }: JobsTableProps) {
  const navigate = useNavigate()

  if (jobs.length === 0) {
    return (
      <EmptyState
        title="No jobs yet"
        action={
          <Link to="/jobs/new" className="buttonlike primary">
            Submit a job
          </Link>
        }
      >
        Jobs appear here once you submit a FabricAIJob, from this UI or with kubectl.
      </EmptyState>
    )
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Framework</th>
            <th className="num">GPUs</th>
            <th>Status</th>
            {!compact && <th>Age</th>}
          </tr>
        </thead>
        <tbody>
          {jobs.map((job) => (
            <tr key={job.metadata.name} data-clickable="" onClick={() => navigate(`/jobs/${job.metadata.name}`)}>
              <td>
                <Link to={`/jobs/${job.metadata.name}`} className="card-link" onClick={(e) => e.stopPropagation()}>
                  {job.metadata.name}
                </Link>
              </td>
              <td>{jobFramework(job)}</td>
              <td className="num">{jobGpus(job)}</td>
              <td>
                <span className={`pill ${phaseTone(job.status?.phase)}`}>{job.status?.phase || 'Unknown'}</span>
              </td>
              {!compact && (
                <td className="faint" title={formatDate(job.metadata.creationTimestamp)}>
                  {formatRelative(job.metadata.creationTimestamp)}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
