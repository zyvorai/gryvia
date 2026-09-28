import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import JobsTable from '@/components/JobsTable'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'

const STATUSES = ['Running', 'Pending', 'Completed', 'Failed']

export default function Jobs() {
  const { data: jobs, isLoading, isError, refetch, isRefetching } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 15000,
  })

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Jobs" title="Jobs unavailable." tint="red" />
        <p className="login-error" role="alert">Failed to load jobs. Please try again.</p>
      </>
    )
  }

  return (
    <div className="apple-story-stack">
      <PageHero eyebrow="Jobs" title="Every job, in one place." lede="Manage training and inference workloads" />

      <div className="page-actions">
        <Link to="/jobs/new" className="buttonlike primary">
          Submit Job
        </Link>
        <button className="btn-secondary" onClick={() => refetch()} disabled={isRefetching}>
          Refresh
        </button>
      </div>

      <div className="apple-metric-band">
        {STATUSES.map((status) => (
          <div key={status}>
            <span>{status}</span>
            <b>{jobs?.filter((j) => j.status?.phase === status).length || 0}</b>
          </div>
        ))}
      </div>

      <section className="card">
        <div className="stat-head">
          <h2>All Jobs</h2>
          <span className="faint">{jobs?.length || 0} total</span>
        </div>
        {isLoading ? <LoadingSpinner /> : <JobsTable jobs={jobs || []} />}
      </section>
    </div>
  )
}
