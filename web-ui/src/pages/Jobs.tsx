import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import JobsTable from '@/components/JobsTable'
import LoadingSpinner from '@/components/LoadingSpinner'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'

const STATUSES = ['Running', 'Pending', 'Completed', 'Failed']

export default function Jobs() {
  const { data: jobs, isLoading, isError, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 15000,
  })

  if (isError) {
    return (
      <>
        <PageHero eyebrow="Jobs" title="Jobs unavailable." tint="red" />
        <p className="warning" role="alert">Failed to load jobs. Please try again.</p>
      </>
    )
  }

  const count = (status: string) => jobs?.filter((j) => j.status?.phase === status).length || 0
  const failed = count('Failed')

  return (
    <>
      <PageHero eyebrow="Jobs" title="Every job, in one place." lede="Manage training and inference workloads" />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          headline={
            jobs
              ? failed > 0
                ? `${failed} job${failed === 1 ? '' : 's'} failed.`
                : `${jobs.length} job${jobs.length === 1 ? '' : 's'}, none failed.`
              : undefined
          }
          tone={jobs ? (failed > 0 ? 'warn' : undefined) : undefined}
          figures={STATUSES.map((status) => ({
            label: status,
            value: jobs ? count(status) : undefined,
            tone: status === 'Failed' && jobs ? countTone(failed) : undefined,
          }))}
        />

        <section className="card span3">
          <p className="eyebrow">ALL JOBS · {jobs?.length || 0} TOTAL</p>
          <h2 className="card-title">All Jobs</h2>
          <div className="toolbar">
            <Link to="/jobs/new" className="buttonlike primary">
              Submit Job
            </Link>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              Refresh
            </button>
          </div>
          {isLoading ? <LoadingSpinner /> : <JobsTable jobs={jobs || []} />}
        </section>
      </div>
    </>
  )
}
