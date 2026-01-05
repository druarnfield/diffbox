import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { cn } from '@/lib/utils'

type WorkflowType = 'i2v' | 'svi' | 'qwen'

const workflows: { id: WorkflowType; name: string; description: string }[] = [
  { id: 'i2v', name: 'Wan 2.2 I2V', description: 'Image to Video generation' },
  { id: 'svi', name: 'SVI 2.0 Pro', description: 'Infinite video streaming' },
  { id: 'qwen', name: 'Qwen Edit', description: 'Image editing & inpainting' },
]

export default function WorkflowPage() {
  const { type } = useParams<{ type?: string }>()
  const [activeWorkflow, setActiveWorkflow] = useState<WorkflowType>(
    (type as WorkflowType) || 'i2v'
  )

  return (
    <div className="space-y-6">
      {/* Workflow selector */}
      <div className="flex gap-2">
        {workflows.map((workflow) => (
          <button
            key={workflow.id}
            onClick={() => setActiveWorkflow(workflow.id)}
            className={cn(
              'px-4 py-2 rounded-lg border transition-colors',
              activeWorkflow === workflow.id
                ? 'border-primary bg-primary/10'
                : 'border-border hover:border-primary/50'
            )}
          >
            <div className="font-medium">{workflow.name}</div>
            <div className="text-xs text-muted-foreground">
              {workflow.description}
            </div>
          </button>
        ))}
      </div>

      {/* Workflow form */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Input panel */}
        <div className="space-y-4 p-6 border rounded-lg">
          <h2 className="text-lg font-semibold">Input</h2>

          {/* Image upload */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Input Image</label>
            <div className="border-2 border-dashed rounded-lg p-8 text-center">
              <p className="text-muted-foreground">
                Drop an image here or click to upload
              </p>
            </div>
          </div>

          {/* Prompt */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Prompt</label>
            <textarea
              className="w-full px-3 py-2 border rounded-md resize-none"
              rows={3}
              placeholder="Describe what you want to generate..."
            />
          </div>

          {/* Negative prompt */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Negative Prompt</label>
            <textarea
              className="w-full px-3 py-2 border rounded-md resize-none"
              rows={2}
              placeholder="What to avoid..."
            />
          </div>

          {/* Basic settings */}
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Seed</label>
              <input
                type="number"
                className="w-full px-3 py-2 border rounded-md"
                placeholder="Random"
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Resolution</label>
              <select className="w-full px-3 py-2 border rounded-md">
                <option value="480x832">480 x 832</option>
                <option value="720x1280">720 x 1280</option>
                <option value="544x960">544 x 960</option>
              </select>
            </div>
          </div>

          {/* Generate button */}
          <button className="w-full py-3 bg-primary text-primary-foreground rounded-md font-medium hover:bg-primary/90 transition-colors">
            Generate
          </button>
        </div>

        {/* Output panel */}
        <div className="space-y-4 p-6 border rounded-lg">
          <h2 className="text-lg font-semibold">Output</h2>
          <div className="aspect-video bg-muted rounded-lg flex items-center justify-center">
            <p className="text-muted-foreground">Output will appear here</p>
          </div>

          {/* Progress */}
          <div className="space-y-2">
            <div className="flex justify-between text-sm">
              <span>Progress</span>
              <span>0%</span>
            </div>
            <div className="h-2 bg-muted rounded-full overflow-hidden">
              <div className="h-full bg-primary w-0 transition-all" />
            </div>
          </div>
        </div>
      </div>

      {/* Advanced settings (collapsed) */}
      <details className="border rounded-lg">
        <summary className="px-6 py-4 cursor-pointer font-medium">
          Advanced Settings
        </summary>
        <div className="px-6 pb-6 grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Steps</label>
            <input
              type="number"
              defaultValue={50}
              className="w-full px-3 py-2 border rounded-md"
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">CFG Scale</label>
            <input
              type="number"
              defaultValue={5.0}
              step={0.1}
              className="w-full px-3 py-2 border rounded-md"
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Frames</label>
            <input
              type="number"
              defaultValue={81}
              className="w-full px-3 py-2 border rounded-md"
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium">Denoising</label>
            <input
              type="number"
              defaultValue={1.0}
              step={0.1}
              max={1}
              className="w-full px-3 py-2 border rounded-md"
            />
          </div>
        </div>
      </details>
    </div>
  )
}
