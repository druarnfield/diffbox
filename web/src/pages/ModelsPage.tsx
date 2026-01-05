import { useState, useEffect } from 'react'
import { cn } from '@/lib/utils'

type ModelType = 'all' | 'checkpoint' | 'lora' | 'vae' | 'controlnet'
type Tab = 'browse' | 'local' | 'downloads'

interface DownloadStatus {
  name: string
  url: string
  status: 'complete' | 'downloading' | 'queued' | 'missing' | 'active' | 'waiting'
  progress: number
  total_size: number
  completed_size: number
  download_speed: number
  workflow: string
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

function formatSpeed(bytesPerSec: number): string {
  return formatBytes(bytesPerSec) + '/s'
}

export default function ModelsPage() {
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<ModelType>('all')
  const [activeTab, setActiveTab] = useState<Tab>('downloads')
  const [downloads, setDownloads] = useState<DownloadStatus[]>([])
  const [loading, setLoading] = useState(false)

  // Fetch download status
  useEffect(() => {
    if (activeTab !== 'downloads') return

    const fetchDownloads = async () => {
      try {
        setLoading(true)
        const res = await fetch('/api/downloads')
        const data = await res.json()
        setDownloads(data || [])
      } catch (err) {
        console.error('Failed to fetch downloads:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchDownloads()
    const interval = setInterval(fetchDownloads, 3000) // Poll every 3 seconds

    return () => clearInterval(interval)
  }, [activeTab])

  const types: { id: ModelType; label: string }[] = [
    { id: 'all', label: 'All' },
    { id: 'checkpoint', label: 'Checkpoints' },
    { id: 'lora', label: 'LoRAs' },
    { id: 'vae', label: 'VAEs' },
    { id: 'controlnet', label: 'ControlNets' },
  ]

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'complete':
        return 'text-green-600 bg-green-50 border-green-200'
      case 'downloading':
      case 'active':
        return 'text-blue-600 bg-blue-50 border-blue-200'
      case 'waiting':
      case 'queued':
        return 'text-yellow-600 bg-yellow-50 border-yellow-200'
      case 'missing':
        return 'text-red-600 bg-red-50 border-red-200'
      default:
        return 'text-gray-600 bg-gray-50 border-gray-200'
    }
  }

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

      {/* Search and filters - only show on Browse/Local tabs */}
      {activeTab !== 'downloads' && (
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
      )}

      {/* Tabs: Browse / Local / Downloads */}
      <div className="border-b">
        <div className="flex gap-4">
          <button
            onClick={() => setActiveTab('browse')}
            className={cn(
              'px-4 py-2 border-b-2 transition-colors',
              activeTab === 'browse'
                ? 'border-primary font-medium'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            Browse
          </button>
          <button
            onClick={() => setActiveTab('local')}
            className={cn(
              'px-4 py-2 border-b-2 transition-colors',
              activeTab === 'local'
                ? 'border-primary font-medium'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            Local
          </button>
          <button
            onClick={() => setActiveTab('downloads')}
            className={cn(
              'px-4 py-2 border-b-2 transition-colors',
              activeTab === 'downloads'
                ? 'border-primary font-medium'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            Downloads
          </button>
        </div>
      </div>

      {/* Download status tab */}
      {activeTab === 'downloads' && (
        <div className="space-y-4">
          {loading && downloads.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              Loading downloads...
            </div>
          ) : downloads.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              No required models found
            </div>
          ) : (
            <div className="space-y-3">
              {downloads.map((download) => (
                <div
                  key={download.name}
                  className="border rounded-lg p-4 space-y-3"
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <h3 className="font-medium">{download.name}</h3>
                      <p className="text-sm text-muted-foreground mt-1">
                        Workflow: {download.workflow}
                      </p>
                    </div>
                    <span
                      className={cn(
                        'px-3 py-1 text-xs font-medium rounded-full border',
                        getStatusColor(download.status)
                      )}
                    >
                      {download.status}
                    </span>
                  </div>

                  {/* Progress bar */}
                  {(download.status === 'downloading' ||
                    download.status === 'active' ||
                    download.progress > 0) && (
                    <div className="space-y-1">
                      <div className="flex justify-between text-sm">
                        <span className="text-muted-foreground">
                          {formatBytes(download.completed_size)} /{' '}
                          {formatBytes(download.total_size)}
                        </span>
                        <span className="font-medium">
                          {download.progress.toFixed(1)}%
                        </span>
                      </div>
                      <div className="w-full h-2 bg-muted rounded-full overflow-hidden">
                        <div
                          className="h-full bg-primary transition-all duration-300"
                          style={{ width: `${download.progress}%` }}
                        />
                      </div>
                      {download.download_speed > 0 && (
                        <div className="text-sm text-muted-foreground">
                          {formatSpeed(download.download_speed)}
                        </div>
                      )}
                    </div>
                  )}

                  {download.status === 'complete' && (
                    <div className="text-sm text-muted-foreground">
                      Size: {formatBytes(download.completed_size)}
                    </div>
                  )}

                  {download.status === 'missing' && (
                    <div className="text-sm text-muted-foreground">
                      Not downloaded • {formatBytes(download.total_size)}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Browse/Local tabs - placeholder */}
      {(activeTab === 'browse' || activeTab === 'local') && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
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
      )}
    </div>
  )
}
