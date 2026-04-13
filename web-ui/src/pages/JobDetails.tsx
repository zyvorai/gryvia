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
        <p className="text-[#5a7a9e]">Job not found</p>
      </div>
    )
  }

  const getStatusIcon = (phase?: string) => {
    switch (phase) {
      case 'Running':
        return <Play className="h-5 w-5 text-[#7ecbf5]" />
      case 'Completed':
        return <CheckCircle className="h-5 w-5 text-emerald-400" />
      case 'Failed':
        return <XCircle className="h-5 w-5 text-red-400" />
      case 'Pending':
      case 'Queued':
        return <Clock className="h-5 w-5 text-[#fbbf24]" />
      default:
        return <AlertCircle className="h-5 w-5 text-[#5a7a9e]" />
    }
  }

  const getStatusStyle = (phase?: string) => {
    switch (phase) {
      case 'Running':
        return { bg: 'rgba(95,168,211,0.08)', color: '#7ecbf5' }
      case 'Completed':
      case 'Succeeded':
        return { bg: 'rgba(34,197,94,0.08)', color: '#4ade80' }
      case 'Failed':
        return { bg: 'rgba(239,68,68,0.08)', color: '#f87171' }
      case 'Pending':
      case 'Queued':
        return { bg: 'rgba(251,191,36,0.08)', color: '#fbbf24' }
      default:
        return { bg: 'rgba(192,204,224,0.05)', color: '#8ba4c0' }
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

  const statusStyle = getStatusStyle(job.status?.phase)

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-4">
        <button
          onClick={() => navigate('/jobs')}
          className="inline-flex items-center text-sm text-[#8090a8] hover:text-[#c0cce0] transition-colors"
        >
          <ArrowLeft className="h-4 w-4 mr-1" />
          Back to Jobs
        </button>
      </div>

      <div className="rounded-xl overflow-hidden" style={{
        background: 'linear-gradient(165deg, #151d28 0%, #111820 100%)',
        border: '1px solid rgba(192,204,224,0.06)',
        boxShadow: 'inset 0 1px 0 rgba(192,204,224,0.04)',
      }}>
        {/* Header */}
        <div className="px-6 py-4" style={{
          borderBottom: '1px solid rgba(192,204,224,0.04)',
          background: 'rgba(10,14,20,0.3)',
        }}>
          <div className="flex items-center justify-between">
            <div className="flex items-center">
              {getStatusIcon(job.status?.phase)}
              <h2 className="ml-2 text-2xl font-bold text-[#e8ecf1]">{job.metadata.name}</h2>
            </div>
            <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium"
              style={{ background: statusStyle.bg, color: statusStyle.color }}
            >
              {job.status?.phase || 'Unknown'}
            </span>
          </div>
          {job.status?.message && (
            <p className="mt-2 text-sm text-[#5a7a9e]">{job.status.message}</p>
          )}
        </div>

        <div className="px-6 py-6">
          {/* Timing Info */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
            <div>
              <dt className="text-sm font-medium text-[#5a7a9e]">Created</dt>
              <dd className="mt-1 text-sm text-[#c0cce0]">
                {job.metadata.creationTimestamp
                  ? new Date(job.metadata.creationTimestamp).toLocaleString()
                  : 'N/A'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-[#5a7a9e]">Started</dt>
              <dd className="mt-1 text-sm text-[#c0cce0]">
                {job.status?.startTime
                  ? new Date(job.status.startTime).toLocaleString()
                  : 'N/A'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-[#5a7a9e]">Duration</dt>
              <dd className="mt-1 text-sm text-[#c0cce0]">
                {formatDuration(job.status?.startTime, job.status?.completionTime)}
              </dd>
            </div>
          </div>

          {/* Resource Allocation */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-[#e8ecf1] mb-4">Resource Allocation</h3>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="rounded-xl p-4" style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.04)',
                boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
              }}>
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-[#5a7a9e] mr-2" />
                  <span className="text-sm font-medium text-[#8ba4c0]">GPU Type</span>
                </div>
                <p className="text-lg font-semibold text-[#e8ecf1]">{job.spec.resources.gpuType}</p>
              </div>
              <div className="rounded-xl p-4" style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.04)',
                boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
              }}>
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-[#5a7a9e] mr-2" />
                  <span className="text-sm font-medium text-[#8ba4c0]">GPU Count</span>
                </div>
                <p className="text-lg font-semibold text-[#e8ecf1]">{job.spec.resources.gpuCount}</p>
              </div>
              <div className="rounded-xl p-4" style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.04)',
                boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
              }}>
                <div className="flex items-center mb-2">
                  <HardDrive className="h-4 w-4 text-[#5a7a9e] mr-2" />
                  <span className="text-sm font-medium text-[#8ba4c0]">Memory</span>
                </div>
                <p className="text-lg font-semibold text-[#e8ecf1]">{job.spec.resources.memory}</p>
              </div>
              <div className="rounded-xl p-4" style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.04)',
                boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
              }}>
                <div className="flex items-center mb-2">
                  <Cpu className="h-4 w-4 text-[#5a7a9e] mr-2" />
                  <span className="text-sm font-medium text-[#8ba4c0]">CPU Cores</span>
                </div>
                <p className="text-lg font-semibold text-[#e8ecf1]">{job.spec.resources.cpu}</p>
              </div>
            </div>
          </div>

          {/* Job Configuration */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-[#e8ecf1] mb-4">Job Configuration</h3>
            <div className="rounded-xl p-4" style={{
              background: 'rgba(10,14,20,0.5)',
              border: '1px solid rgba(192,204,224,0.04)',
              boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
            }}>
              <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <dt className="text-sm font-medium text-[#5a7a9e]">Framework</dt>
                  <dd className="mt-1 text-sm text-[#c0cce0]">{job.spec.framework}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-[#5a7a9e]">Image</dt>
                  <dd className="mt-1 text-sm text-[#c0cce0] font-mono text-xs">{job.spec.image}</dd>
                </div>
                {job.spec.distributed?.enabled && (
                  <>
                    <div>
                      <dt className="text-sm font-medium text-[#5a7a9e]">Distributed Training</dt>
                      <dd className="mt-1 text-sm text-[#c0cce0]">Enabled</dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-[#5a7a9e]">Strategy</dt>
                      <dd className="mt-1 text-sm text-[#c0cce0]">{job.spec.distributed.strategy}</dd>
                    </div>
                    {job.spec.distributed.nodes && (
                      <div>
                        <dt className="text-sm font-medium text-[#5a7a9e]">Nodes</dt>
                        <dd className="mt-1 text-sm text-[#c0cce0]">{job.spec.distributed.nodes}</dd>
                      </div>
                    )}
                    {job.spec.distributed.gpusPerNode && (
                      <div>
                        <dt className="text-sm font-medium text-[#5a7a9e]">GPUs/Node</dt>
                        <dd className="mt-1 text-sm text-[#c0cce0]">{job.spec.distributed.gpusPerNode}</dd>
                      </div>
                    )}
                  </>
                )}
              </dl>
            </div>
          </div>

          {/* Command */}
          <div className="mb-8">
            <h3 className="text-lg font-medium text-[#e8ecf1] mb-4">Command</h3>
            <div className="rounded-xl p-4" style={{
              background: 'rgba(5,7,10,0.6)',
              border: '1px solid rgba(192,204,224,0.04)',
              boxShadow: 'inset 0 2px 4px rgba(0,0,0,0.4)',
            }}>
              <code className="text-sm text-[#4ade80] font-mono">
                {job.spec.command?.join(' ') || 'N/A'}
              </code>
            </div>
          </div>

          {/* Environment Variables */}
          {job.spec.env && job.spec.env.length > 0 && (
            <div className="mb-8">
              <h3 className="text-lg font-medium text-[#e8ecf1] mb-4">Environment Variables</h3>
              <div className="rounded-xl p-4" style={{
                background: 'rgba(10,14,20,0.5)',
                border: '1px solid rgba(192,204,224,0.04)',
                boxShadow: 'inset 0 1px 3px rgba(0,0,0,0.3)',
              }}>
                <dl className="space-y-2">
                  {job.spec.env.map((env, idx) => {
                    const sensitivePatterns = /SECRET|PASSWORD|TOKEN|KEY|CREDENTIAL|API_KEY|PRIVATE|AUTH|ACCESS_ID|DATABASE_URL|CONN/i
                    const isSensitive = sensitivePatterns.test(env.name)
                    return (
                      <div key={idx} className="flex items-start">
                        <dt className="text-sm font-medium text-[#8ba4c0] w-1/3">{env.name}</dt>
                        <dd className="text-sm text-[#c0cce0] font-mono w-2/3">
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
              <h3 className="text-lg font-medium text-[#e8ecf1] mb-4">Labels</h3>
              <div className="flex flex-wrap gap-2">
                {Object.entries(job.metadata.labels).map(([key, value]) => (
                  <span
                    key={key}
                    className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium text-[#b0c4d8]"
                    style={{
                      background: 'rgba(192,204,224,0.05)',
                      border: '1px solid rgba(192,204,224,0.08)',
                    }}
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
