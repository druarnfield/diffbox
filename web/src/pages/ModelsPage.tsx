import { useState } from 'react'
import { cn } from '@/lib/utils'

type ModelType = 'all' | 'checkpoint' | 'lora' | 'vae' | 'controlnet'

export default function ModelsPage() {
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<ModelType>('all')

  const types: { id: ModelType; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'checkpoint', label: 'Checkpoints' },
    { id: 'lora', label: 'LoRAs' },
    { id: 'vae', label: 'VAEs' },
    { id: 'controlnet', label: 'ControlNets' },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Models</h1>
        <div className="flex gap-2">
          <button className="px-4 py-2 border rounded-md hover:bg-muted transition-colors">
            Sync Metadata
          </button>
        </div>
      </div>

      {/* Search and filters */}
      <div className="flex gap-4">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search models..."
          className="flex-1 px-4 py-2 border rounded-md"
        />
        <div className="flex gap-1 p-1 bg-muted rounded-md">
          {types.map((type) => (
            <button
              key={type.id}
              onClick={() => setTypeFilter(type.id)}
              className={cn(
                'px-3 py-1 rounded text-sm transition-colors',
                typeFilter === type.id
                  ? 'bg-background shadow-sm'
                  : 'hover:bg-background/50'
              )}
            >
              {type.label}
            </button>
          ))}
        </div>
      </div>

      {/* Tabs: Browse / Local */}
      <div className="border-b">
        <div className="flex gap-4">
          <button className="px-4 py-2 border-b-2 border-primary font-medium">
            Browse
          </button>
          <button className="px-4 py-2 border-b-2 border-transparent text-muted-foreground hover:text-foreground">
            Local
          </button>
          <button className="px-4 py-2 border-b-2 border-transparent text-muted-foreground hover:text-foreground">
            Downloads
          </button>
        </div>
      </div>

      {/* Model grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {/* Placeholder cards */}
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="border rounded-lg overflow-hidden">
            <div className="aspect-square bg-muted" />
            <div className="p-4 space-y-2">
              <h3 className="font-medium">Model Name</h3>
              <p className="text-sm text-muted-foreground">
                Author • LoRA • Wan 2.2
              </p>
              <div className="flex items-center justify-between">
                <span className="text-xs text-muted-foreground">
                  10.5k downloads
                </span>
                <button className="px-3 py-1 text-sm border rounded hover:bg-muted transition-colors">
                  Download
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>

    </div>
  )
}
