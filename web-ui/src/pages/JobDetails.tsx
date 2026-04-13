import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import { ArrowLeft, Play, XCircle, Clock, CheckCircle, AlertCircle, Cpu, HardDrive } from 'lucide-react'

export default function JobDetails() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()

  const { data: job, isLoading, isError } = useQuery({
    queryKey: ['job', name],
    queryFn: () => api.getJob(name!),
    enabled: !!name,
    refetchInterval: 5000,
  })

  if (isLoading) {
    return <LoadingSpinner />
  }

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-red-400">Failed to load job details. Please try again.</p>
      </div>
    )
  }

  if (!job) {
    return (
      <div className="text-center py-12">
        <p className="text-slate-400">Job not found</p>
      </div>
    )
  }

  const getStatusIcon = (phase?: string) => {
    switch (phase) {
      case 'Running':
        return <Play className="h-5 w-5 text-blue-500" />
      case 'Completed':
        return <CheckCircle className="h-5 w-5 text-green-500" />
      case 'Failed':
        return <XCircle className="h-5 w-5 text-red-500" />
      case 'Pending':
      case 'Queued':
        return <Clock className="h-5 w-5 text-yellow-500" />
      default:
        return <AlertCircle className="h-5 w-5 text-slate-400" />
    }
  }

  const getStatusColor = (phase?: string) => {
    switch (phase) {
      case 'Running':
        return 'bg-blue-500/10 text-blue-400'
      case 'Completed':
      case 'Succeeded':
        return 'bg-green-500/10 text-green-400'
      case 'Failed':
        return 'bg-red-500/10 text-red-400'
      case 'Pending':
      case 'Queued':
        return 'bg-yellow-500/10 text-yellow-400'
      default:
        return 'bg-slate-700/30 text-slate-300'
    }
  }

  const formatDuration = (start?: string, end?: string) => {
    if (!start) return 'N/A'
    const startTime = new Date(start).getTime()
    const endTime = end ? new Date(end).getTime() : Date.now()
    const duration = endTime - startTime
    const hours = Math.floor(duration / 3600000)
    const minutes = Math.floor((duration % 3600000) / 60000)
    return `${hours}h ${minutes}m`
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-4">
        <button
          onClick={() => navigate('/jobs')}
          className="inline-flex items-center text-sm text-slate-400 hover:text-slate-300"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          Back to Jobs
        </button>
      </div>

      <div className="bg-slate-800/50 border border-slate-700/50 rounded-xl overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-slate-700/30 bg-slate-900/30">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              {getStatusIcon(job.status?.phase)}
              <h2 className="ml-2 text-2xl font-bold text-white">{job.metadata.name}</h2>
            </div>
            <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${getStatusColor(job.status?.phase)}`}>
              {job.status?.phase || 'Unknown'}
            </span>
          </div>
          {job.status?.message && (
            <p className="mt-2 text-sm text-slate-400">{job.status.message}</p>
          )}
        </div>

        <div className="px-6 py-6">
          {/* Timing Info */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
            <div>
              <dt className="text-sm font-medium text-slate-400">Created</dt>
              <dd className="mt-1 text-sm text-white">
                {job.metadata.creationTimestamp
                  ? new Date(job.metadata.creationTimestamp).toLocaleString()
                  : 'N/A'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-slate-400">Started</dt>
              <dd className="mt-1 text-sm text-white">
                {job.status?.startTime
                  ? new Date(job.status.startTime).toLocaleString()
                  : 'N/A'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-slate-400">Duration</dt>
              <dd className="mt-1 text-sm text-white">
                {formatDuration(job.status?.startTime, job.status?.completionTime)}
              </dd>
            </div>
          </div>

          {/* Resource Allocation */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-white mb-4">Resource Allocation</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="bg-slate-900/30 rounded-xl p-4">
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-slate-400 mr-2" />
                  <span className="text-sm font-medium text-slate-300">GPU Type</span>
                </div>
                <p className="text-lg font-semibold text-white">{job.spec.resources.gpuType}</p>
              </div>
              <div className="bg-slate-900/30 rounded-xl p-4">
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-slate-400 mr-2" />
                  <span className="text-sm font-medium text-slate-300">GPU Count</span>
                </div>
                <p className="text-lg font-semibold text-white">{job.spec.resources.gpuCount}</p>
              </div>
              <div className="bg-slate-900/30 rounded-xl p-4">
                <div className="flex items-center mb-2">
                  <HardDrive className="h-4 w-4 text-slate-400 mr-2" />
                  <span className="text-sm font-medium text-slate-300">Memory</span>
                </div>
                <p className="text-lg font-semibold text-white">{job.spec.resources.memory}</p>
              </div>
              <div className="bg-slate-900/30 rounded-xl p-4">
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-slate-400 mr-2" />
                  <span className="text-sm font-medium text-slate-300">CPU Cores</span>
                </div>
                <p className="text-lg font-semibold text-white">{job.spec.resources.cpu}</p>
              </div>
            </div>
          </div>

          {/* Job Configuration */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-white mb-4">Job Configuration</h3>
            <div className="bg-slate-900/30 rounded-xl p-4">
              <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <dt className="text-sm font-medium text-slate-400">Framework</dt>
                  <dd className="mt-1 text-sm text-white">{job.spec.framework}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-slate-400">Image</dt>
                  <dd className="mt-1 text-sm text-white font-mono text-xs">{job.spec.image}</dd>
                </div>
                {job.spec.distributed?.enabled && (
                  <>
                    <div>
                      <dt className="text-sm font-medium text-slate-400">Distributed Training</dt>
                      <dd className="mt-1 text-sm text-white">Enabled</dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-slate-400">Strategy</dt>
                      <dd className="mt-1 text-sm text-white">{job.spec.distributed.strategy}</dd>
                    </div>
                    {job.spec.distributed.nodes && (
                      <div>
                        <dt className="text-sm font-medium text-slate-400">Nodes</dt>
                        <dd className="mt-1 text-sm text-white">{job.spec.distributed.nodes}</dd>
                      </div>
                    )}
                    {job.spec.distributed.gpusPerNode && (
                      <div>
                        <dt className="text-sm font-medium text-slate-400">GPUs/Node</dt>
                        <dd className="mt-1 text-sm text-white">{job.spec.distributed.gpusPerNode}</dd>
                      </div>
                    )}
                  </>
                )}
              </dl>
            </div>
          </div>

          {/* Command */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-white mb-4">Command</h3>
            <div className="bg-gray-900 rounded-xl p-4">
              <code className="text-sm text-green-400 font-mono">
                {job.spec.command.join(' ')}
              </code>
            </div>
          </div>

          {/* Environment Variables */}
          {job.spec.env && job.spec.env.length > 0 && (
            <div className="mb-8">
              <h3 className="text-lg font-medium text-white mb-4">Environment Variables</h3>
              <div className="bg-slate-900/30 rounded-xl p-4">
                <dl className="space-y-2">
                  {job.spec.env.map((env, idx) => {
                    const sensitivePatterns = /SECRET|PASSWORD|TOKEN|KEY|CREDENTIAL|API_KEY|PRIVATE|AUTH|ACCESS_ID|DATABASE_URL|CONN/i
                    const isSensitive = sensitivePatterns.test(env.name)
                    return (
                      <div key={idx} className="flex items-start">
                        <dt className="text-sm font-medium text-slate-300 w-1/3">{env.name}</dt>
                        <dd className="text-sm text-white font-mono w-2/3">
                          {isSensitive ? '********' : (env.value || '')}
                        </dd>
                      </div>
                    )
                  })}
                </dl>
              </div>
            </div>
          )}

          {/* Labels */}
          {job.metadata.labels && Object.keys(job.metadata.labels).length > 0 && (
            <div>
              <h3 className="text-lg font-medium text-white mb-4">Labels</h3>
              <div className="flex flex-wrap gap-2">
                {Object.entries(job.metadata.labels).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-slate-700/30 text-slate-300"
                  >
                    {key}: {value}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
