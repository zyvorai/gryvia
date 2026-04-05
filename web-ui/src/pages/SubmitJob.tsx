import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { FabricAIJob } from '@/types'
import { ArrowLeft, Plus, Trash2 } from 'lucide-react'

export default function SubmitJob() {
  const navigate = useNavigate()
  const [formData, setFormData] = useState({
    name: '',
    framework: 'pytorch',
    gpuType: 'H100',
    gpuCount: 1,
    memory: '32Gi',
    cpu: 8,
    image: 'nvcr.io/nvidia/pytorch:24.01-py3',
    command: '',
    distributedEnabled: false,
    distributedStrategy: 'ddp',
    worldSize: 1,
    env: [] as Array<{ name: string; value: string }>,
  })

  const createJobMutation = useMutation({
    mutationFn: (job: Partial<FabricAIJob>) => api.createJob(job),
    onSuccess: () => {
      navigate('/jobs')
    },
  })

  const [error, setError] = useState<string | null>(null)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    // Validate Kubernetes resource name
    const k8sNameRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
    if (!k8sNameRegex.test(formData.name)) {
      setError('Job name must consist of lowercase alphanumeric characters or hyphens, and must start and end with an alphanumeric character')
      return
    }

    const job: Partial<FabricAIJob> = {
      apiVersion: 'kubefabric.ai/v1',
      kind: 'FabricAIJob',
      metadata: {
        name: formData.name,
        namespace: 'default',
      },
      spec: {
        framework: formData.framework,
        resources: {
          gpuType: formData.gpuType,
          gpuCount: formData.gpuCount,
          memory: formData.memory,
          cpu: formData.cpu,
        },
        image: formData.image,
        command: formData.command.split(' ').filter(s => s.length > 0),
        distributed: formData.distributedEnabled ? {
          enabled: true,
          strategy: formData.distributedStrategy,
          worldSize: formData.worldSize,
        } : undefined,
        env: formData.env.length > 0 ? formData.env : undefined,
      },
    }

    createJobMutation.mutate(job)
  }

  const addEnvVar = () => {
    setFormData({
      ...formData,
      env: [...formData.env, { name: '', value: '' }],
    })
  }

  const removeEnvVar = (index: number) => {
    setFormData({
      ...formData,
      env: formData.env.filter((_, i) => i !== index),
    })
  }

  const updateEnvVar = (index: number, field: 'name' | 'value', value: string) => {
    const newEnv = [...formData.env]
    newEnv[index][field] = value
    setFormData({ ...formData, env: newEnv })
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

      <div className="bg-white shadow rounded-lg">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-2xl font-bold text-gray-900">Submit New Job</h2>
          <p className="mt-1 text-sm text-gray-500">
            Configure and submit an AI training or inference job
          </p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-6 space-y-6">
          {/* Basic Info */}
          <div>
            <h3 className="text-lg font-medium text-gray-900 mb-4">Basic Information</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-700">Job Name</label>
                <input
                  type="text"
                  required
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                  placeholder="my-training-job"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">Framework</label>
                <select
                  value={formData.framework}
                  onChange={(e) => setFormData({ ...formData, framework: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                >
                  <option value="pytorch">PyTorch</option>
                  <option value="tensorflow">TensorFlow</option>
                  <option value="jax">JAX</option>
                  <option value="mxnet">MXNet</option>
                </select>
              </div>

              <div className="md:col-span-2">
                <label className="block text-sm font-medium text-gray-700">Container Image</label>
                <input
                  type="text"
                  required
                  value={formData.image}
                  onChange={(e) => setFormData({ ...formData, image: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm font-mono text-xs"
                  placeholder="nvcr.io/nvidia/pytorch:24.01-py3"
                />
              </div>
            </div>
          </div>

          {/* Resources */}
          <div>
            <h3 className="text-lg font-medium text-gray-900 mb-4">Resource Requirements</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-sm font-medium text-gray-700">GPU Type</label>
                <select
                  value={formData.gpuType}
                  onChange={(e) => setFormData({ ...formData, gpuType: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                >
                  <option value="H100">H100</option>
                  <option value="A100-80G">A100-80G</option>
                  <option value="A100-40G">A100-40G</option>
                  <option value="L40">L40</option>
                  <option value="V100">V100</option>
                  <option value="T4">T4</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">GPU Count</label>
                <input
                  type="number"
                  min="1"
                  max="512"
                  required
                  value={formData.gpuCount}
                  onChange={(e) => setFormData({ ...formData, gpuCount: parseInt(e.target.value) || 1 })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">Memory</label>
                <input
                  type="text"
                  required
                  value={formData.memory}
                  onChange={(e) => setFormData({ ...formData, memory: e.target.value })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                  placeholder="32Gi"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700">CPU Cores</label>
                <input
                  type="number"
                  min="1"
                  max="128"
                  required
                  value={formData.cpu}
                  onChange={(e) => setFormData({ ...formData, cpu: parseInt(e.target.value) || 1 })}
                  className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                />
              </div>
            </div>
          </div>

          {/* Distributed Training */}
          <div>
            <h3 className="text-lg font-medium text-gray-900 mb-4">Distributed Training</h3>
            <div className="space-y-4">
              <div className="flex items-center">
                <input
                  type="checkbox"
                  checked={formData.distributedEnabled}
                  onChange={(e) => setFormData({ ...formData, distributedEnabled: e.target.checked })}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                />
                <label className="ml-2 block text-sm text-gray-900">
                  Enable distributed training
                </label>
              </div>

              {formData.distributedEnabled && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-sm font-medium text-gray-700">Strategy</label>
                    <select
                      value={formData.distributedStrategy}
                      onChange={(e) => setFormData({ ...formData, distributedStrategy: e.target.value })}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                    >
                      <option value="ddp">DDP (Distributed Data Parallel)</option>
                      <option value="fsdp">FSDP (Fully Sharded Data Parallel)</option>
                      <option value="horovod">Horovod</option>
                      <option value="deepspeed">DeepSpeed</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700">World Size</label>
                    <input
                      type="number"
                      min="1"
                      max="64"
                      value={formData.worldSize}
                      onChange={(e) => setFormData({ ...formData, worldSize: parseInt(e.target.value) || 1 })}
                      className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                    />
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Command */}
          <div>
            <h3 className="text-lg font-medium text-gray-900 mb-4">Command</h3>
            <textarea
              required
              rows={3}
              value={formData.command}
              onChange={(e) => setFormData({ ...formData, command: e.target.value })}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm font-mono text-xs"
              placeholder="python train.py --epochs 100 --batch-size 32"
            />
          </div>

          {/* Environment Variables */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-medium text-gray-900">Environment Variables</h3>
              <button
                type="button"
                onClick={addEnvVar}
                className="inline-flex items-center px-3 py-1 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              >
                <Plus className="h-4 w-4 mr-1" />
                Add Variable
              </button>
            </div>
            <div className="space-y-3">
              {formData.env.map((env, idx) => (
                <div key={idx} className="flex items-center space-x-3">
                  <input
                    type="text"
                    value={env.name}
                    onChange={(e) => updateEnvVar(idx, 'name', e.target.value)}
                    placeholder="VARIABLE_NAME"
                    className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm font-mono"
                  />
                  <input
                    type="text"
                    value={env.value}
                    onChange={(e) => updateEnvVar(idx, 'value', e.target.value)}
                    placeholder="value"
                    className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm font-mono"
                  />
                  <button
                    type="button"
                    onClick={() => removeEnvVar(idx)}
                    className="p-2 text-red-600 hover:text-red-800"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          </div>

          {/* Submit Buttons */}
          <div className="flex justify-end space-x-3 pt-6 border-t border-gray-200">
            <button
              type="button"
              onClick={() => navigate('/jobs')}
              className="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={createJobMutation.isPending}
              className="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50"
            >
              {createJobMutation.isPending ? 'Submitting...' : 'Submit Job'}
            </button>
          </div>

          {error && (
            <div className="rounded-md bg-yellow-50 p-4">
              <p className="text-sm text-yellow-800">{error}</p>
            </div>
          )}

          {createJobMutation.isError && (
            <div className="rounded-md bg-red-50 p-4">
              <p className="text-sm text-red-800">
                Error submitting job: {(createJobMutation.error as Error).message}
              </p>
            </div>
          )}
        </form>
      </div>
    </div>
  )
}
