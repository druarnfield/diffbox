import { useEffect, useRef, useCallback } from 'react'
import { useJobStore } from '@/stores/jobStore'

interface WSMessage {
  type: string
  data: unknown
}

interface JobProgress {
  job_id: string
  progress: number
  stage: string
  preview?: string
}

interface JobComplete {
  job_id: string
  output: {
    type: string
    path: string
    frames?: number
  }
}

interface JobError {
  job_id: string
  error: string
}

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const subscribe = useCallback((jobIds: string[]) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'subscribe',
        data: { job_ids: jobIds }
      }))
    }
  }, [])

  useEffect(() => {
    const connect = () => {
      if (wsRef.current?.readyState === WebSocket.OPEN) return

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/ws`

      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        console.log('WebSocket connected')
      }

      ws.onmessage = (event) => {
        try {
          const message: WSMessage = JSON.parse(event.data)
          // Get latest store actions directly to avoid stale closure
          const { updateJobProgress, completeJob, failJob } = useJobStore.getState()

          switch (message.type) {
            case 'job:progress': {
              const data = message.data as JobProgress
              updateJobProgress(data.job_id, data.progress, data.stage, data.preview)
              break
            }
            case 'job:complete': {
              const data = message.data as JobComplete
              completeJob(data.job_id, {
                type: data.output.type as 'video' | 'image',
                path: data.output.path,
                frames: data.output.frames,
              })
              break
            }
            case 'job:error': {
              const data = message.data as JobError
              failJob(data.job_id, data.error)
              break
            }
          }
        } catch (err) {
          console.error('WebSocket message parse error:', err)
        }
      }

      ws.onclose = () => {
        console.log('WebSocket disconnected, reconnecting...')
        reconnectTimeoutRef.current = setTimeout(connect, 3000)
      }

      ws.onerror = (err) => {
        console.error('WebSocket error:', err)
      }
    }

    connect()

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      wsRef.current?.close()
    }
  }, [])

  return { subscribe }
}
