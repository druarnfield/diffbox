import { cn } from '@/lib/utils'

interface ProgressProps {
  value: number
  stage?: string
  className?: string
  showLabel?: boolean
}

export function Progress({
  value,
  stage,
  className,
  showLabel = true,
}: ProgressProps) {
  const percentage = Math.round(value * 100)

  return (
    <div className={cn('space-y-2', className)}>
      {showLabel && (
        <div className="flex justify-between items-center text-sm">
          <span className="text-muted-foreground truncate max-w-[70%]">
            {stage || 'Waiting...'}
          </span>
          <span className="font-mono text-primary tabular-nums">{percentage}%</span>
        </div>
      )}
      <div className="progress-bar">
        <div
          className="progress-bar-fill"
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  )
}

interface JobStatusProps {
  status: 'pending' | 'running' | 'completed' | 'failed'
  className?: string
}

export function JobStatus({ status, className }: JobStatusProps) {
  const config = {
    pending: {
      label: 'Queued',
      color: 'text-muted-foreground',
      bg: 'bg-muted',
    },
    running: {
      label: 'Running',
      color: 'text-primary',
      bg: 'bg-primary/20',
    },
    completed: {
      label: 'Complete',
      color: 'text-green-400',
      bg: 'bg-green-500/20',
    },
    failed: {
      label: 'Failed',
      color: 'text-destructive',
      bg: 'bg-destructive/20',
    },
  }

  const { label, color, bg } = config[status]

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium',
        bg,
        color,
        className
      )}
    >
      {status === 'running' && (
        <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
      )}
      {label}
    </span>
  )
}
