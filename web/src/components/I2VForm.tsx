import { useState, useCallback } from 'react'
import { ImageUpload } from './ImageUpload'
import { Progress } from './Progress'
import { submitI2V } from '@/api/workflows'
import { useJobStore } from '@/stores/jobStore'
import { fileToBase64, cn } from '@/lib/utils'

interface I2VFormProps {
  className?: string
}

interface FormState {
  image: File | null
  prompt: string
  negativePrompt: string
  seed: string
  resolution: string
  steps: number
  cfgScale: number
  frames: number
  denoising: number
}

const RESOLUTIONS = [
  { value: '480x832', label: '480 x 832 (Portrait)' },
  { value: '832x480', label: '832 x 480 (Landscape)' },
  { value: '544x960', label: '544 x 960 (9:16)' },
  { value: '960x544', label: '960 x 544 (16:9)' },
]

export function I2VForm({ className }: I2VFormProps) {
  const { addJob, jobs, activeJobId } = useJobStore()
  const activeJob = jobs.find((j) => j.id === activeJobId)

  const [form, setForm] = useState<FormState>({
    image: null,
    prompt: '',
    negativePrompt: '',
    seed: '',
    resolution: '480x832',
    steps: 50,
    cfgScale: 5.0,
    frames: 81,
    denoising: 1.0,
  })

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = useCallback(async () => {
    if (!form.image) {
      setError('Please select an image')
      return
    }
    if (!form.prompt.trim()) {
      setError('Please enter a prompt')
      return
    }

    setError(null)
    setIsSubmitting(true)

    try {
      const imageBase64 = await fileToBase64(form.image)
      const [width, height] = form.resolution.split('x').map(Number)

      const response = await submitI2V({
        input_image: imageBase64,
        prompt: form.prompt.trim(),
        negative_prompt: form.negativePrompt.trim() || undefined,
        seed: form.seed ? parseInt(form.seed, 10) : undefined,
        width,
        height,
        num_inference_steps: form.steps,
        cfg_scale: form.cfgScale,
        num_frames: form.frames,
        denoising_strength: form.denoising,
      })

      addJob({
        id: response.id,
        type: 'i2v',
        status: 'pending',
        progress: 0,
        stage: 'Queued',
        params: { prompt: form.prompt },
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit job')
    } finally {
      setIsSubmitting(false)
    }
  }, [form, addJob])

  const showingActiveJob = activeJob && (activeJob.status === 'running' || activeJob.status === 'completed')

  return (
    <div className={cn('grid grid-cols-1 lg:grid-cols-2 gap-6', className)}>
      {/* Input Panel */}
      <div className="card p-6 space-y-5">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <svg className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 4v16M17 4v16M3 8h4m10 0h4M3 12h18M3 16h4m10 0h4M4 20h16a1 1 0 001-1V5a1 1 0 00-1-1H4a1 1 0 00-1 1v14a1 1 0 001 1z" />
          </svg>
          Image to Video
        </h2>

        <ImageUpload
          value={form.image}
          onChange={(file) => setForm((s) => ({ ...s, image: file }))}
        />

        <div className="space-y-2">
          <label className="label">Prompt</label>
          <textarea
            value={form.prompt}
            onChange={(e) => setForm((s) => ({ ...s, prompt: e.target.value }))}
            className="input resize-none"
            rows={3}
            placeholder="Describe the motion you want to see..."
          />
        </div>

        <div className="space-y-2">
          <label className="label">Negative Prompt</label>
          <textarea
            value={form.negativePrompt}
            onChange={(e) => setForm((s) => ({ ...s, negativePrompt: e.target.value }))}
            className="input resize-none"
            rows={2}
            placeholder="What to avoid (optional)..."
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <label className="label">Seed</label>
            <input
              type="number"
              value={form.seed}
              onChange={(e) => setForm((s) => ({ ...s, seed: e.target.value }))}
              className="input"
              placeholder="Random"
            />
          </div>
          <div className="space-y-2">
            <label className="label">Resolution</label>
            <select
              value={form.resolution}
              onChange={(e) => setForm((s) => ({ ...s, resolution: e.target.value }))}
              className="input"
            >
              {RESOLUTIONS.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Advanced Settings */}
        <details className="group">
          <summary className="cursor-pointer text-sm font-medium text-muted-foreground hover:text-foreground transition-colors flex items-center gap-2">
            <svg className="w-4 h-4 transition-transform group-open:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
            Advanced Settings
          </summary>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
            <div className="space-y-2">
              <label className="label">Steps</label>
              <input
                type="number"
                value={form.steps}
                onChange={(e) => setForm((s) => ({ ...s, steps: parseInt(e.target.value, 10) || 50 }))}
                className="input"
                min={1}
                max={100}
              />
            </div>
            <div className="space-y-2">
              <label className="label">CFG Scale</label>
              <input
                type="number"
                value={form.cfgScale}
                onChange={(e) => setForm((s) => ({ ...s, cfgScale: parseFloat(e.target.value) || 5 }))}
                className="input"
                step={0.1}
                min={1}
                max={20}
              />
            </div>
            <div className="space-y-2">
              <label className="label">Frames</label>
              <input
                type="number"
                value={form.frames}
                onChange={(e) => setForm((s) => ({ ...s, frames: parseInt(e.target.value, 10) || 81 }))}
                className="input"
                min={1}
                max={200}
              />
            </div>
            <div className="space-y-2">
              <label className="label">Denoising</label>
              <input
                type="number"
                value={form.denoising}
                onChange={(e) => setForm((s) => ({ ...s, denoising: parseFloat(e.target.value) || 1 }))}
                className="input"
                step={0.1}
                min={0}
                max={1}
              />
            </div>
          </div>
        </details>

        {error && (
          <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/30 text-destructive text-sm">
            {error}
          </div>
        )}

        <button
          onClick={handleSubmit}
          disabled={isSubmitting || !form.image}
          className={cn(
            'btn-primary w-full py-3',
            isSubmitting && 'animate-pulse-glow'
          )}
        >
          {isSubmitting ? (
            <>
              <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
              Submitting...
            </>
          ) : (
            <>
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              Generate Video
            </>
          )}
        </button>
      </div>

      {/* Output Panel */}
      <div className="card p-6 space-y-4">
        <h2 className="text-lg font-semibold">Output</h2>

        <div className="output-preview">
          {showingActiveJob && activeJob.output?.type === 'video' ? (
            <video
              src={`/outputs/${activeJob.id}.mp4`}
              className="w-full h-full object-contain"
              controls
              autoPlay
              loop
            />
          ) : showingActiveJob && activeJob.preview ? (
            <img
              src={`data:image/jpeg;base64,${activeJob.preview}`}
              alt="Preview"
              className="w-full h-full object-contain"
            />
          ) : activeJob?.status === 'running' ? (
            <div className="text-center">
              <div className="w-12 h-12 mx-auto border-3 border-primary/30 border-t-primary rounded-full animate-spin mb-3" />
              <p className="text-sm text-muted-foreground">{activeJob.stage}</p>
            </div>
          ) : (
            <p className="text-muted-foreground text-sm">Output will appear here</p>
          )}
        </div>

        {activeJob && (activeJob.status === 'running' || activeJob.status === 'pending') && (
          <Progress value={activeJob.progress} stage={activeJob.stage} />
        )}

        {activeJob?.status === 'completed' && activeJob.output && (
          <a
            href={`/outputs/${activeJob.id}.mp4`}
            download
            className="btn-secondary w-full justify-center"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            Download Video
          </a>
        )}

        {activeJob?.status === 'failed' && (
          <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/30 text-destructive text-sm">
            {activeJob.error || 'Generation failed'}
          </div>
        )}
      </div>
    </div>
  )
}
