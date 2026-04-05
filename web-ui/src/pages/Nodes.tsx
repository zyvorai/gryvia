import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import LoadingSpinner from '@/components/LoadingSpinner'
import { Server, Cpu, HardDrive, Thermometer, Activity } from 'lucide-react'

export default function Nodes() {
  const { data: nodes, isLoading } = useQuery({
    queryKey: ['nodes'],
    queryFn: api.getNodes,
    refetchInterval: 10000,
  })

  if (isLoading) {
    return <LoadingSpinner />
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold text-gray-900">GPU Nodes</h2>
        <p className="mt-1 text-sm text-gray-500">
          Physical GPU nodes and real-time metrics
        </p>
      </div>

      {/* Overview Stats */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-4">
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">Total Nodes</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">{nodes?.length || 0}</dd>
          </div>
        </div>
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">Total GPUs</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">
              {nodes?.reduce((sum, node) => sum + node.spec.gpuCount, 0) || 0}
            </dd>
          </div>
        </div>
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">RDMA Enabled</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">
              {nodes?.filter(node => node.spec.rdmaEnabled).length || 0}
            </dd>
          </div>
        </div>
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">Ready Nodes</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">
              {nodes?.filter(node => node.status?.phase === 'Ready').length || 0}
            </dd>
          </div>
        </div>
      </div>

      {/* Nodes List */}
      <div className="grid grid-cols-1 gap-6">
        {nodes?.map((node) => (
          <div key={node.metadata.name} className="bg-white shadow rounded-lg overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 bg-gray-50">
              <div className="flex items-center justify-between">
                <div className="flex items-center">
                  <Server className="h-5 w-5 text-gray-400 mr-2" />
                  <h3 className="text-lg font-medium text-gray-900">{node.spec.nodeName}</h3>
                </div>
                <div className="flex items-center space-x-2">
                  <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    node.status?.phase === 'Ready' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                  }`}>
                    {node.status?.phase || 'Unknown'}
                  </span>
                  {node.spec.rdmaEnabled && (
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                      RDMA
                    </span>
                  )}
                </div>
              </div>
            </div>

            <div className="px-6 py-4">
              {/* Node Info */}
              <div className="grid grid-cols-3 gap-4 mb-6">
                <div>
                  <dt className="text-xs font-medium text-gray-500 uppercase">GPU Type</dt>
                  <dd className="mt-1 text-sm font-semibold text-gray-900">{node.spec.gpuType}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium text-gray-500 uppercase">GPU Count</dt>
                  <dd className="mt-1 text-sm font-semibold text-gray-900">{node.spec.gpuCount}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium text-gray-500 uppercase">Memory</dt>
                  <dd className="mt-1 text-sm font-semibold text-gray-900">{node.spec.memory}</dd>
                </div>
              </div>

              {/* GPU Metrics */}
              {node.status?.gpus && node.status.gpus.length > 0 && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-3">GPU Metrics</h4>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {node.status.gpus.map((gpu) => (
                      <div key={gpu.index} className="border border-gray-200 rounded-lg p-4">
                        <div className="flex items-center justify-between mb-3">
                          <div className="flex items-center">
                            <Cpu className="h-4 w-4 text-gray-400 mr-2" />
                            <span className="text-sm font-medium text-gray-900">GPU {gpu.index}</span>
                          </div>
                          <span className="text-xs text-gray-500 font-mono">{gpu.uuid.substring(0, 12)}...</span>
                        </div>

                        {/* Temperature */}
                        <div className="mb-3">
                          <div className="flex items-center justify-between mb-1">
                            <div className="flex items-center">
                              <Thermometer className="h-3 w-3 text-gray-400 mr-1" />
                              <span className="text-xs text-gray-600">Temperature</span>
                            </div>
                            <span className="text-xs font-medium text-gray-900">{gpu.temperature}°C</span>
                          </div>
                          <div className="w-full bg-gray-200 rounded-full h-1.5">
                            <div
                              className={`h-1.5 rounded-full ${
                                gpu.temperature > 80 ? 'bg-red-600' : gpu.temperature > 70 ? 'bg-yellow-500' : 'bg-green-500'
                              }`}
                              style={{ width: `${Math.min(gpu.temperature, 100)}%` }}
                            />
                          </div>
                        </div>

                        {/* Utilization */}
                        <div className="mb-3">
                          <div className="flex items-center justify-between mb-1">
                            <div className="flex items-center">
                              <Activity className="h-3 w-3 text-gray-400 mr-1" />
                              <span className="text-xs text-gray-600">Utilization</span>
                            </div>
                            <span className="text-xs font-medium text-gray-900">{gpu.utilization}%</span>
                          </div>
                          <div className="w-full bg-gray-200 rounded-full h-1.5">
                            <div
                              className="h-1.5 rounded-full bg-blue-600"
                              style={{ width: `${gpu.utilization}%` }}
                            />
                          </div>
                        </div>

                        {/* Memory */}
                        <div>
                          <div className="flex items-center justify-between mb-1">
                            <div className="flex items-center">
                              <HardDrive className="h-3 w-3 text-gray-400 mr-1" />
                              <span className="text-xs text-gray-600">Memory</span>
                            </div>
                            <span className="text-xs font-medium text-gray-900">
                              {(gpu.memoryUsed / 1024).toFixed(1)} / {(gpu.memoryTotal / 1024).toFixed(1)} GB
                            </span>
                          </div>
                          <div className="w-full bg-gray-200 rounded-full h-1.5">
                            <div
                              className="h-1.5 rounded-full bg-purple-600"
                              style={{ width: `${gpu.memoryTotal > 0 ? (gpu.memoryUsed / gpu.memoryTotal) * 100 : 0}%` }}
                            />
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
