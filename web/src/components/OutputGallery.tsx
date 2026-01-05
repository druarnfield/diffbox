import { cn } from '@/lib/utils'
import { Job, useJobStore } from '@/stores/jobStore'
import { Progress, JobStatus } from './Progress'

interface OutputGalleryProps {
  className?: string
}

export function OutputGallery({ className }: OutputGalleryProps) {
  const { jobs, activeJobId, setActiveJob, removeJob } = useJobStore()

  if (jobs.length === 0) {
    return (
      <div className={cn('card p-6', className)}>
        <h3 className="text-sm font-medium text-muted-foreground mb-4">Recent Jobs</h3>
        <div className="text-center py-8">
          <div className="w-12 h-12 mx-auto rounded-full bg-secondary flex items-center justify-center mb-3">
            <svg className="w-6 h-6 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <p className="text-sm text-muted-foreground">No jobs yet</p>
          <p className="text-xs text-muted-foreground/60 mt-1">Submit a job to get started</p>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('card p-6', className)}>
      <h3 className="text-sm font-medium text-muted-foreground mb-4">
        Recent Jobs ({jobs.length})
      </h3>
      <div className="space-y-3">
        {jobs.slice(0, 10).map((job) => (
          <JobCard
            key={job.id}
            job={job}
            isActive={job.id === activeJobId}
            onSelect={() => setActiveJob(job.id)}
            onRemove={() => removeJob(job.id)}
          />
        ))}
      </div>
    </div>
  )
}

interface JobCardProps {
  job: Job
  isActive: boolean
  onSelect: () => void
  onRemove: () => void
}

function JobCard({ job, isActive, onSelect, onRemove }: JobCardProps) {
  const hasOutput = job.status === 'completed' && job.output

  return (
    <div
      onClick={onSelect}
      className={cn(
        'group relative p-3 rounded-lg border cursor-pointer transition-all duration-200',
        isActive
          ? 'border-primary/40 bg-primary/5'
          : 'border-border hover:border-border/80 hover:bg-secondary/30'
      )}
    >
      <div className="flex items-start gap-3">
        {/* Thumbnail */}
        <div className="w-16 h-12 rounded bg-secondary overflow-hidden flex-shrink-0">
          {job.preview ? (
            <img
              src={`data:image/jpeg;base64,${job.preview}`}
              alt=""
              className="w-full h-full object-cover"
            />
          ) : hasOutput ? (
            job.output?.type === 'video' ? (
              <video
                src={`/outputs/${job.id}.mp4`}
                className="w-full h-full object-cover"
                muted
              />
            ) : (
              <img
                src={`/outputs/${job.id}.png`}
                alt=""
                className="w-full h-full object-cover"
              />
            )
          ) : (
            <div className="w-full h-full flex items-center justify-center">
              {job.status === 'running' ? (
                <div className="w-4 h-4 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
              ) : (
                <span className="text-xs text-muted-foreground">
                  {job.type.toUpperCase()}
                </span>
              )}
            </div>
          )}
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium truncate">
              {job.type === 'i2v' ? 'Image to Video' : 'Image Edit'}
            </span>
            <JobStatus status={job.status} />
          </div>

          {job.status === 'running' && (
            <Progress
              value={job.progress}
              stage={job.stage}
              showLabel={false}
              className="mt-2"
            />
          )}

          {job.status === 'failed' && (
            <p className="text-xs text-destructive mt-1 truncate">
              {job.error}
            </p>
          )}

          {job.status === 'completed' && (
            <p className="text-xs text-muted-foreground mt-1">
              {new Date(job.createdAt).toLocaleTimeString()}
            </p>
          )}
        </div>

        {/* Remove button */}
        <button
          onClick={(e) => {
            e.stopPropagation()
            onRemove()
          }}
          className="absolute top-2 right-2 p-1 rounded opacity-0 group-hover:opacity-100 hover:bg-secondary transition-all"
          aria-label="Remove job"
        >
          <svg className="w-3.5 h-3.5 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  )
}
