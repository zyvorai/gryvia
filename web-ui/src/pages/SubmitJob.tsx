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
    nodes: 1,
    gpusPerNode: 1,
    env: [] as Array<{ id: number; name: string; value: string }>,
  })

  const createJobMutation = useMutation({
    mutationFn: (job: Partial<FabricAIJob>) => api.createJob(job),
    onSuccess: () => {
      navigate('/jobs')
    },
  })

  const [envIdCounter, setEnvIdCounter] = useState(0)
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
      apiVersion: 'tensorreaper.ai/v1',
      kind: 'FabricAIJob',
      metadata: {
        name: formData.name,
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
        command: splitCommand(formData.command),
        distributed: formData.distributedEnabled ? {
          enabled: true,
          strategy: formData.distributedStrategy,
          nodes: formData.nodes,
          gpusPerNode: formData.gpusPerNode,
        } : undefined,
        env: formData.env.length > 0 ? formData.env.map(({ name, value }) => ({ name, value })) : undefined,
      },
    }

    createJobMutation.mutate(job)
  }

  const addEnvVar = () => {
    setEnvIdCounter(prev => prev + 1)
    setFormData({
      ...formData,
      env: [...formData.env, { id: envIdCounter + 1, name: '', value: '' }],
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

  const inputStyle = {
    background: 'rgba(10,14,20,0.6)',
    border: '1px solid rgba(192,204,224,0.08)',
    boxShadow: 'inset 0 2px 4px rgba(0,0,0,0.3)',
    color: '#d0dae6',
  }

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
        <div className="px-6 py-4" style={{
          borderBottom: '1px solid rgba(192,204,224,0.04)',
          background: 'rgba(10,14,20,0.3)',
        }}>
          <h2 className="text-lg font-bold text-gradient-copper">Submit New Job</h2>
          <p className="mt-1 text-xs text-[#5a7a9e]">
            Configure and submit an AI training or inference job
          </p>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-6 space-y-6">
          {/* Basic Info */}
          <div>
            <h3 className="text-sm font-semibold text-[#e8ecf1] mb-4">Basic Information</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-xs font-medium text-[#8ba4c0]">Job Name</label>
                <input
                  type="text"
                  required
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="mt-1 block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
                  placeholder="my-training-job"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-[#8ba4c0]">Framework</label>
                <select
                  value={formData.framework}
                  onChange={(e) => setFormData({ ...formData, framework: e.target.value })}
                  className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
                >
                  <option value="pytorch">PyTorch</option>
                  <option value="tensorflow">TensorFlow</option>
                  <option value="jax">JAX</option>
                  <option value="mxnet">MXNet</option>
                </select>
              </div>

              <div className="md:col-span-2">
                <label className="block text-xs font-medium text-[#8ba4c0]">Container Image</label>
                <input
                  type="text"
                  required
                  value={formData.image}
                  onChange={(e) => setFormData({ ...formData, image: e.target.value })}
                  className="mt-1 block w-full rounded-lg text-sm font-mono text-xs placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
                  placeholder="nvcr.io/nvidia/pytorch:24.01-py3"
                />
              </div>
            </div>
          </div>

          {/* Resources */}
          <div>
            <h3 className="text-sm font-semibold text-[#e8ecf1] mb-4">Resource Requirements</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="block text-xs font-medium text-[#8ba4c0]">GPU Type</label>
                <select
                  value={formData.gpuType}
                  onChange={(e) => setFormData({ ...formData, gpuType: e.target.value })}
                  className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
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
                <label className="block text-xs font-medium text-[#8ba4c0]">GPU Count</label>
                <input
                  type="number"
                  min="1"
                  max="512"
                  required
                  value={formData.gpuCount}
                  onChange={(e) => setFormData({ ...formData, gpuCount: parseInt(e.target.value, 10) || 1 })}
                  className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-[#8ba4c0]">Memory</label>
                <input
                  type="text"
                  required
                  value={formData.memory}
                  onChange={(e) => setFormData({ ...formData, memory: e.target.value })}
                  pattern="^\d+(\.\d+)?(Ki|Mi|Gi|Ti|K|M|G|T)?$"
                  title="Enter a valid memory value (e.g. 32Gi, 512Mi, 1Ti)"
                  className="mt-1 block w-full rounded-lg text-sm placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
                  placeholder="32Gi"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-[#8ba4c0]">CPU Cores</label>
                <input
                  type="number"
                  min="1"
                  max="128"
                  required
                  value={formData.cpu}
                  onChange={(e) => setFormData({ ...formData, cpu: parseInt(e.target.value, 10) || 1 })}
                  className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                  style={inputStyle}
                />
              </div>
            </div>
          </div>

          {/* Distributed Training */}
          <div>
            <h3 className="text-sm font-semibold text-[#e8ecf1] mb-4">Distributed Training</h3>
            <div className="space-y-4">
              <div className="flex items-center">
                <input
                  type="checkbox"
                  checked={formData.distributedEnabled}
                  onChange={(e) => setFormData({ ...formData, distributedEnabled: e.target.checked })}
                  className="h-4 w-4 rounded"
                  style={{ accentColor: '#d4764e' }}
                />
                <label className="ml-2 block text-sm text-[#c0cce0]">
                  Enable distributed training
                </label>
              </div>

              {formData.distributedEnabled && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div>
                    <label className="block text-xs font-medium text-[#8ba4c0]">Strategy</label>
                    <select
                      value={formData.distributedStrategy}
                      onChange={(e) => setFormData({ ...formData, distributedStrategy: e.target.value })}
                      className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                      style={inputStyle}
                    >
                      <option value="ddp">DDP (Distributed Data Parallel)</option>
                      <option value="fsdp">FSDP (Fully Sharded Data Parallel)</option>
                      <option value="horovod">Horovod</option>
                      <option value="deepspeed">DeepSpeed</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-[#8ba4c0]">Nodes</label>
                    <input
                      type="number"
                      min="1"
                      max="64"
                      value={formData.nodes}
                      onChange={(e) => setFormData({ ...formData, nodes: parseInt(e.target.value, 10) || 1 })}
                      className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                      style={inputStyle}
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-[#8ba4c0]">GPUs/Node</label>
                    <input
                      type="number"
                      min="1"
                      max="8"
                      value={formData.gpusPerNode}
                      onChange={(e) => setFormData({ ...formData, gpusPerNode: parseInt(e.target.value, 10) || 1 })}
                      className="mt-1 block w-full rounded-lg text-sm focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                      style={inputStyle}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Command */}
          <div>
            <h3 className="text-sm font-semibold text-[#e8ecf1] mb-4">Command</h3>
            <textarea
              required
              rows={3}
              value={formData.command}
              onChange={(e) => setFormData({ ...formData, command: e.target.value })}
              className="mt-1 block w-full rounded-lg text-sm font-mono text-xs placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
              style={inputStyle}
              placeholder="python train.py --epochs 100 --batch-size 32"
            />
          </div>

          {/* Environment Variables */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-[#e8ecf1]">Environment Variables</h3>
              <button
                type="button"
                onClick={addEnvVar}
                className="inline-flex items-center px-3 py-1 rounded-lg text-xs font-medium text-[#8ba4c0] hover:text-[#c0cce0] transition-colors"
                style={{
                  border: '1px solid rgba(192,204,224,0.1)',
                  background: 'rgba(21,29,40,0.5)',
                }}
              >
                <Plus className="h-4 w-4 mr-1" />
                Add Variable
              </button>
            </div>
            <div className="space-y-3">
              {formData.env.map((env, idx) => (
                <div key={env.id} className="flex items-center space-x-3">
                  <input
                    type="text"
                    value={env.name}
                    onChange={(e) => updateEnvVar(idx, 'name', e.target.value)}
                    placeholder="VARIABLE_NAME"
                    className="flex-1 rounded-lg text-sm font-mono placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                    style={inputStyle}
                  />
                  <input
                    type="text"
                    value={env.value}
                    onChange={(e) => updateEnvVar(idx, 'value', e.target.value)}
                    placeholder="value"
                    className="flex-1 rounded-lg text-sm font-mono placeholder-[#344e6a] focus:ring-2 focus:ring-[#d4764e] focus:border-[#d4764e] focus:outline-none"
                    style={inputStyle}
                  />
                  <button
                    type="button"
                    onClick={() => removeEnvVar(idx)}
                    className="p-2 text-red-400 hover:text-red-300 transition-colors"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          </div>

          {error && (
            <div className="rounded-xl p-4" style={{
              background: 'rgba(251,191,36,0.06)',
              border: '1px solid rgba(251,191,36,0.12)',
            }}>
              <p className="text-xs text-[#fbbf24]">{error}</p>
            </div>
          )}

          {createJobMutation.isError && (
            <div className="rounded-xl p-4" style={{
              background: 'rgba(239,68,68,0.06)',
              border: '1px solid rgba(239,68,68,0.12)',
            }}>
              <p className="text-xs text-[#f87171]">
                Error submitting job: {createJobMutation.error instanceof Error ? createJobMutation.error.message : 'Unknown error'}
              </p>
            </div>
          )}

          {/* Submit Buttons */}
          <div className="flex justify-end space-x-3 pt-6" style={{ borderTop: '1px solid rgba(192,204,224,0.04)' }}>
            <button
              type="button"
              onClick={() => navigate('/jobs')}
              className="px-4 py-2 rounded-lg text-sm font-medium text-[#8ba4c0] hover:text-[#c0cce0] transition-all"
              style={{
                border: '1px solid rgba(192,204,224,0.1)',
                background: 'rgba(21,29,40,0.5)',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={createJobMutation.isPending}
              className="px-4 py-2 btn-copper rounded-lg text-sm font-medium disabled:opacity-50"
            >
              {createJobMutation.isPending ? 'Submitting...' : 'Submit Job'}
            </button>
          </div>

        </form>
      </div>
    </div>
  )
}

function splitCommand(cmd: string): string[] {
  const args: string[] = []
  let current = ''
  let inQuote = false
  let quoteChar = ''
  for (const char of cmd) {
    if (inQuote) {
      if (char === quoteChar) { inQuote = false }
      else { current += char }
    } else if (char === '"' || char === "'") {
      inQuote = true
      quoteChar = char
    } else if (char === ' ') {
      if (current) { args.push(current); current = '' }
    } else {
      current += char
    }
  }
  if (current) args.push(current)
  return args
}
