import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import JobsTable from '@/components/JobsTable'
import PageHero from '@/components/PageHero'
import PagePulse from '@/components/kit/PagePulse'
import { countTone } from '@/components/kit/tone'
import { ErrorState, Skeleton } from '@/components/StateViews'
import { countByGroup } from '@/lib/phase'
import { errorMessage } from '@/lib/errors'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

export default function Jobs() {
  useDocumentTitle('Jobs')
  const { data: jobs, isLoading, isError, error, refetch, isRefetching, dataUpdatedAt } = useQuery({
    queryKey: ['jobs'],
    queryFn: api.getJobs,
    refetchInterval: 15000,
  })

  const counts = countByGroup(jobs, (j) => j.status?.phase)
  const failed = counts.failed
  const loadFailed = isError && !jobs

  return (
    <>
      <PageHero eyebrow="Jobs" title="Every job, in one place." lede="Manage training and inference workloads" />

      <div className="grid">
        <PagePulse
          updatedAt={dataUpdatedAt}
          error={isError ? errorMessage(error) : undefined}
          headline={
            jobs
              ? failed > 0
                ? `${failed} job${failed === 1 ? '' : 's'} failed.`
                : `${jobs.length} job${jobs.length === 1 ? '' : 's'}, none failed.`
              : undefined
          }
          tone={jobs && failed > 0 ? 'warn' : undefined}
          figures={[
            { label: 'Running', value: jobs ? counts.running : undefined },
            { label: 'Pending', value: jobs ? counts.pending : undefined },
            { label: 'Completed', value: jobs ? counts.completed : undefined },
            { label: 'Failed', value: jobs ? failed : undefined, tone: jobs ? countTone(failed) : undefined },
          ]}
        />

        <section className="card span3">
          <p className="eyebrow">ALL JOBS · {jobs ? `${jobs.length} TOTAL` : '…'}</p>
          <h2 className="card-title">All jobs</h2>
          <div className="toolbar">
            <Link to="/jobs/new" className="buttonlike primary">
              Submit job
            </Link>
            <button className="btn-refresh" onClick={() => refetch()} disabled={isRefetching}>
              {isRefetching ? 'Refreshing…' : 'Refresh'}
            </button>
          </div>
          {isError && jobs && <ErrorState title="Could not refresh jobs; showing the last data." error={error} onRetry={() => refetch()} retrying={isRefetching} />}
          {loadFailed ? (
            <ErrorState title="Could not load jobs." error={error} onRetry={() => refetch()} retrying={isRefetching} />
          ) : isLoading ? (
            <Skeleton rows={5} />
          ) : (
            <JobsTable jobs={jobs || []} />
          )}
        </section>
      </div>
    </>
  )
}
