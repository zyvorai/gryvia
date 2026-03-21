import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import { ArrowLeft, Play, XCircle, Clock, CheckCircle, AlertCircle, Cpu, HardDrive } from 'lucide-react'

export default function JobDetails() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()

  const { data: job, isLoading } = useQuery({
    queryKey: ['job', name],
    queryFn: () => api.getJob(name!),
    enabled: !!name,
    refetchInterval: 5000,
  })

  if (isLoading) {
    return <LoadingSpinner />
  }

  if (!job) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Job not found</p>
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
        return <AlertCircle className="h-5 w-5 text-gray-500" />
    }
  }

  const getStatusColor = (phase?: string) => {
    switch (phase) {
      case 'Running':
        return 'bg-blue-100 text-blue-800'
      case 'Completed':
        return 'bg-green-100 text-green-800'
      case 'Failed':
        return 'bg-red-100 text-red-800'
      case 'Pending':
      case 'Queued':
        return 'bg-yellow-100 text-yellow-800'
      default:
        return 'bg-gray-100 text-gray-800'
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
          className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          Back to Jobs
        </button>
      </div>

      <div className="bg-white shadow rounded-lg overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-gray-200 bg-gray-50">
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              {getStatusIcon(job.status?.phase)}
              <h2 className="ml-2 text-2xl font-bold text-gray-900">{job.metadata.name}</h2>
            </div>
            <span className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${getStatusColor(job.status?.phase)}`}>
              {job.status?.phase || 'Unknown'}
            </span>
          </div>
          {job.status?.message && (
            <p className="mt-2 text-sm text-gray-600">{job.status.message}</p>
          )}
        </div>

        <div className="px-6 py-6">
          {/* Timing Info */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
            <div>
              <dt className="text-sm font-medium text-gray-500">Created</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {job.metadata.creationTimestamp
                  ? new Date(job.metadata.creationTimestamp).toLocaleString()
                  : 'N/A'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Started</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {job.status?.startTime
                  ? new Date(job.status.startTime).toLocaleString()
                  : 'N/A'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Duration</dt>
              <dd className="mt-1 text-sm text-gray-900">
                {formatDuration(job.status?.startTime, job.status?.completionTime)}
              </dd>
            </div>
          </div>

          {/* Resource Allocation */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-gray-900 mb-4">Resource Allocation</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="bg-gray-50 rounded-lg p-4">
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-gray-400 mr-2" />
                  <span className="text-sm font-medium text-gray-700">GPU Type</span>
                </div>
                <p className="text-lg font-semibold text-gray-900">{job.spec.resources.gpuType}</p>
              </div>
              <div className="bg-gray-50 rounded-lg p-4">
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-gray-400 mr-2" />
                  <span className="text-sm font-medium text-gray-700">GPU Count</span>
                </div>
                <p className="text-lg font-semibold text-gray-900">{job.spec.resources.gpuCount}</p>
              </div>
              <div className="bg-gray-50 rounded-lg p-4">
                <div className="flex items-center mb-2">
                  <HardDrive className="h-4 w-4 text-gray-400 mr-2" />
                  <span className="text-sm font-medium text-gray-700">Memory</span>
                </div>
                <p className="text-lg font-semibold text-gray-900">{job.spec.resources.memory}</p>
              </div>
              <div className="bg-gray-50 rounded-lg p-4">
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-gray-400 mr-2" />
                  <span className="text-sm font-medium text-gray-700">CPU Cores</span>
                </div>
                <p className="text-lg font-semibold text-gray-900">{job.spec.resources.cpu}</p>
              </div>
            </div>
          </div>

          {/* Job Configuration */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-gray-900 mb-4">Job Configuration</h3>
            <div className="bg-gray-50 rounded-lg p-4">
              <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <dt className="text-sm font-medium text-gray-500">Framework</dt>
                  <dd className="mt-1 text-sm text-gray-900">{job.spec.framework}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-gray-500">Image</dt>
                  <dd className="mt-1 text-sm text-gray-900 font-mono text-xs">{job.spec.image}</dd>
                </div>
                {job.spec.distributed?.enabled && (
                  <>
                    <div>
                      <dt className="text-sm font-medium text-gray-500">Distributed Training</dt>
                      <dd className="mt-1 text-sm text-gray-900">Enabled</dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-gray-500">Strategy</dt>
                      <dd className="mt-1 text-sm text-gray-900">{job.spec.distributed.strategy}</dd>
                    </div>
                    {job.spec.distributed.worldSize && (
                      <div>
                        <dt className="text-sm font-medium text-gray-500">World Size</dt>
                        <dd className="mt-1 text-sm text-gray-900">{job.spec.distributed.worldSize}</dd>
                      </div>
                    )}
                  </>
                )}
              </dl>
            </div>
          </div>

          {/* Command */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-gray-900 mb-4">Command</h3>
            <div className="bg-gray-900 rounded-lg p-4">
              <code className="text-sm text-green-400 font-mono">
                {job.spec.command.join(' ')}
              </code>
            </div>
          </div>

          {/* Environment Variables */}
          {job.spec.env && job.spec.env.length > 0 && (
            <div className="mb-8">
              <h3 className="text-lg font-medium text-gray-900 mb-4">Environment Variables</h3>
              <div className="bg-gray-50 rounded-lg p-4">
                <dl className="space-y-2">
                  {job.spec.env.map((env, idx) => {
                    const sensitivePatterns = /SECRET|PASSWORD|TOKEN|KEY|CREDENTIAL|API_KEY/i
                    const isSensitive = sensitivePatterns.test(env.name)
                    return (
                      <div key={idx} className="flex items-start">
                        <dt className="text-sm font-medium text-gray-700 w-1/3">{env.name}</dt>
                        <dd className="text-sm text-gray-900 font-mono w-2/3">
                          {isSensitive ? '********' : (env.value ?? (env.valueFrom ? '[from secret/configmap]' : ''))}
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
              <h3 className="text-lg font-medium text-gray-900 mb-4">Labels</h3>
              <div className="flex flex-wrap gap-2">
                {Object.entries(job.metadata.labels).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
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
