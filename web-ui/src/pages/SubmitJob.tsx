import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { FRAMEWORK_LABEL } from '@/lib/jobs'
import { api } from '@/lib/api'
import { errorMessage } from '@/lib/errors'
import type { FabricAIJob } from '@/types'
import { Trash2 } from 'lucide-react'
import PageHero from '@/components/PageHero'
import { notify } from '@/lib/notify'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

const K8S_NAME = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const NAME_HELP = 'Lowercase letters, digits and hyphens; start and end with a letter or digit; at most 63 characters.'
const ENV_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/

/** Inline error for a job name, or undefined when valid (empty is reported only after the field is touched). */
function nameError(name: string): string | undefined {
  if (name.length === 0) return 'Job name is required.'
  if (name.length > 63) return `Job name is ${name.length} characters; the maximum is 63.`
  if (!K8S_NAME.test(name)) return NAME_HELP
  return undefined
}

export default function SubmitJob() {
  useDocumentTitle('Submit job')
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [nameTouched, setNameTouched] = useState(false)
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
    nodes: 1,
    gpusPerNode: 1,
    env: [] as Array<{ id: number; name: string; value: string }>,
  })

  const createJobMutation = useMutation({
    mutationFn: (job: Partial<FabricAIJob>) => api.createJob(job),
    onSuccess: (_created, job) => {
      const jobName = job.metadata?.name ?? formData.name
      notify.success(`Submitted job ${jobName}`)
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
      navigate(`/jobs/${jobName}`)
    },
    onError: (err) => notify.error('Could not submit the job', err),
  })

  const [envIdCounter, setEnvIdCounter] = useState(0)
  const [error, setError] = useState<string | null>(null)

  const nameProblem = nameTouched ? nameError(formData.name) : undefined
  const envProblems = formData.env.map((row, i) => {
    if (row.name.trim() === '') return 'Name is required.'
    if (!ENV_NAME.test(row.name)) return 'Use letters, digits and underscores; do not start with a digit.'
    if (formData.env.some((other, j) => j !== i && other.name === row.name)) return 'Duplicate name.'
    return undefined
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    setNameTouched(true)
    const nameProblem = nameError(formData.name)
    if (nameProblem) {
      setError(nameProblem)
      return
    }
    if (envProblems.some(Boolean)) {
      setError('Fix the environment variable errors below before submitting.')
      return
    }

    const job: Partial<FabricAIJob> = {
      apiVersion: 'gryvia.io/v1',
      kind: 'FabricAIJob',
      metadata: {
        name: formData.name,
        labels: { [FRAMEWORK_LABEL]: formData.framework },
      },
      spec: {
        type: 'training',
        image: formData.image,
        gpus: formData.distributedEnabled ? formData.nodes * formData.gpusPerNode : formData.gpuCount,
        gpuType: formData.gpuType,
        command: splitCommand(formData.command),
        resources: {
          requests: { cpu: String(formData.cpu), memory: formData.memory },
        },
        distributed: formData.distributedEnabled ? {
          enabled: true,
          framework: formData.framework,
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
    setFormData({
      ...formData,
      env: formData.env.map((row, i) => (i === index ? { ...row, [field]: value } : row)),
    })
  }

  return (
    <>
      <PageHero eyebrow="Jobs" title="Launch a job in seconds." lede="Configure and submit an AI training or inference job" />

      <form onSubmit={handleSubmit} className="grid">
        <section className="card span2">
          <p className="eyebrow">IDENTITY</p>
          <h2 className="card-title">Basic Information</h2>
          <div className="formgrid">
            <label className="field">
              <span>Job Name</span>
              <input
                type="text"
                required
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                onBlur={() => setNameTouched(true)}
                aria-invalid={nameProblem ? true : undefined}
                aria-describedby="job-name-help"
                maxLength={253}
                placeholder="my-training-job"
              />
              <small id="job-name-help" className={nameProblem ? 'warning' : 'faint'} role={nameProblem ? 'alert' : undefined}>
                {nameProblem ?? NAME_HELP}
              </small>
            </label>

            <label className="field">
              <span>Framework</span>
              <select
                value={formData.framework}
                onChange={(e) => setFormData({ ...formData, framework: e.target.value })}
              >
                <option value="pytorch">PyTorch</option>
                <option value="tensorflow">TensorFlow</option>
                <option value="jax">JAX</option>
                <option value="mxnet">MXNet</option>
              </select>
            </label>

            <label className="field span-all">
              <span>Container Image</span>
              <input
                type="text"
                required
                className="mono"
                value={formData.image}
                onChange={(e) => setFormData({ ...formData, image: e.target.value })}
                placeholder="nvcr.io/nvidia/pytorch:24.01-py3"
              />
            </label>
          </div>
        </section>

        <section className="card">
          <p className="eyebrow">RESOURCES</p>
          <h2 className="card-title">Resource Requirements</h2>
          <div className="formgrid">
            <label className="field">
              <span>GPU Type</span>
              <select
                value={formData.gpuType}
                onChange={(e) => setFormData({ ...formData, gpuType: e.target.value })}
              >
                <option value="H100">H100</option>
                <option value="A100-80G">A100-80G</option>
                <option value="A100-40G">A100-40G</option>
                <option value="L40">L40</option>
                <option value="V100">V100</option>
                <option value="T4">T4</option>
              </select>
            </label>

            <label className="field">
              <span>GPU Count</span>
              <input
                type="number"
                min="1"
                max="512"
                required
                disabled={formData.distributedEnabled}
                aria-describedby={formData.distributedEnabled ? 'gpu-total' : undefined}
                value={formData.distributedEnabled ? formData.nodes * formData.gpusPerNode : formData.gpuCount}
                onChange={(e) => setFormData({ ...formData, gpuCount: parseInt(e.target.value, 10) || 1 })}
              />
            </label>

            <label className="field">
              <span>Memory</span>
              <input
                type="text"
                required
                value={formData.memory}
                onChange={(e) => setFormData({ ...formData, memory: e.target.value })}
                pattern="^\d+(\.\d+)?(Ki|Mi|Gi|Ti|K|M|G|T)?$"
                title="Enter a valid memory value (e.g. 32Gi, 512Mi, 1Ti)"
                placeholder="32Gi"
              />
            </label>

            <label className="field">
              <span>CPU Cores</span>
              <input
                type="number"
                min="1"
                max="128"
                required
                value={formData.cpu}
                onChange={(e) => setFormData({ ...formData, cpu: parseInt(e.target.value, 10) || 1 })}
              />
            </label>
          </div>
        </section>

        <section className="card span2">
          <p className="eyebrow">SCALE</p>
          <h2 className="card-title">Distributed Training</h2>
          <div className="stack">
            <label className="row">
              <input
                type="checkbox"
                checked={formData.distributedEnabled}
                onChange={(e) => setFormData({ ...formData, distributedEnabled: e.target.checked })}
              />
              <span>Enable distributed training</span>
            </label>

            {formData.distributedEnabled && (
              <p id="gpu-total" className="muted">
                Total GPUs: {formData.nodes} nodes × {formData.gpusPerNode} GPUs per node = <b>{formData.nodes * formData.gpusPerNode}</b>
              </p>
            )}

            {formData.distributedEnabled && (
              <div className="formgrid">
                <label className="field">
                  <span>Nodes</span>
                  <input
                    type="number"
                    min="1"
                    max="64"
                    value={formData.nodes}
                    onChange={(e) => setFormData({ ...formData, nodes: parseInt(e.target.value, 10) || 1 })}
                  />
                </label>
                <label className="field">
                  <span>GPUs/Node</span>
                  <input
                    type="number"
                    min="1"
                    max="8"
                    value={formData.gpusPerNode}
                    onChange={(e) => setFormData({ ...formData, gpusPerNode: parseInt(e.target.value, 10) || 1 })}
                  />
                </label>
              </div>
            )}
          </div>
        </section>

        <section className="card">
          <p className="eyebrow">ENTRYPOINT</p>
          <h2 className="card-title">Command</h2>
          <textarea
            required
            rows={3}
            className="codeedit compact"
            value={formData.command}
            onChange={(e) => setFormData({ ...formData, command: e.target.value })}
            placeholder="python train.py --epochs 100 --batch-size 32"
          />
        </section>

        <section className="card span3">
          <p className="eyebrow">ENVIRONMENT</p>
          <h2 className="card-title">Environment Variables</h2>
          <div className="toolbar">
            <button type="button" className="btn-secondary" onClick={addEnvVar}>
              Add Variable
            </button>
          </div>
          <div className="stack">
            {formData.env.map((env, idx) => (
              <div key={env.id} className="stack">
              <div className="toolbar">
                <input
                  type="text"
                  className="mono"
                  value={env.name}
                  onChange={(e) => updateEnvVar(idx, 'name', e.target.value)}
                  placeholder="VARIABLE_NAME"
                  aria-label={`Variable ${idx + 1} name`}
                  aria-invalid={envProblems[idx] ? true : undefined}
                  aria-describedby={envProblems[idx] ? `env-err-${env.id}` : undefined}
                />
                <input
                  type="text"
                  className="mono"
                  value={env.value}
                  onChange={(e) => updateEnvVar(idx, 'value', e.target.value)}
                  placeholder="value"
                  aria-label={`Variable ${idx + 1} value`}
                />
                <button
                  type="button"
                  className="danger"
                  onClick={() => removeEnvVar(idx)}
                  aria-label={env.name ? `Remove variable ${env.name}` : `Remove variable ${idx + 1}`}
                >
                  <Trash2 className="icon-sm" />
                </button>
              </div>
              {envProblems[idx] && (
                <small id={`env-err-${env.id}`} className="warning" role="alert">
                  {envProblems[idx]}
                </small>
              )}
              </div>
            ))}
          </div>
        </section>

        {error && <p className="warning span3" role="alert">{error}</p>}

        {createJobMutation.isError && (
          <p className="warning span3" role="alert">
            Error submitting job: {errorMessage(createJobMutation.error)}
          </p>
        )}

        <div className="toolbar span3">
          <button type="button" className="btn-secondary" onClick={() => navigate('/jobs')}>
            Cancel
          </button>
          <button type="submit" className="primary" disabled={createJobMutation.isPending}>
            {createJobMutation.isPending ? 'Submitting...' : 'Submit Job'}
          </button>
        </div>
      </form>
    </>
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
