import { ReactNode } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { I2VForm } from '@/components/I2VForm'
import { QwenForm } from '@/components/QwenForm'
import { OutputGallery } from '@/components/OutputGallery'
import { useWebSocket } from '@/hooks/useWebSocket'

type WorkflowType = 'i2v' | 'qwen'

const workflows: { id: WorkflowType; name: string; description: string; icon: ReactNode }[] = [
  {
    id: 'i2v',
    name: 'Wan 2.2 I2V',
    description: 'Image to Video',
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 4v16M17 4v16M3 8h4m10 0h4M3 12h18M3 16h4m10 0h4M4 20h16a1 1 0 001-1V5a1 1 0 00-1-1H4a1 1 0 00-1 1v14a1 1 0 001 1z" />
      </svg>
    ),
  },
  {
    id: 'qwen',
    name: 'Qwen Edit',
    description: 'Image Editing',
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
      </svg>
    ),
  },
]

function isValidWorkflow(type: string | undefined): type is WorkflowType {
  return type === 'i2v' || type === 'qwen'
}

export default function WorkflowPage() {
  const { type } = useParams<{ type?: string }>()
  const navigate = useNavigate()

  // Derive active workflow from URL, defaulting to 'i2v'
  const activeWorkflow: WorkflowType = isValidWorkflow(type) ? type : 'i2v'

  // Initialize WebSocket connection
  useWebSocket()

  const handleWorkflowChange = (id: WorkflowType) => {
    navigate(`/workflow/${id}`, { replace: true })
  }

  return (
    <div className="space-y-6">
      {/* Workflow selector */}
      <div className="flex items-center gap-2">
        {workflows.map((workflow) => (
          <button
            key={workflow.id}
            onClick={() => handleWorkflowChange(workflow.id)}
            className={cn(
              'workflow-tab flex items-center gap-3',
              activeWorkflow === workflow.id && 'active'
            )}
          >
            <span
              className={cn(
                'transition-colors',
                activeWorkflow === workflow.id
                  ? 'text-primary'
                  : 'text-muted-foreground'
              )}
            >
              {workflow.icon}
            </span>
            <div className="text-left">
              <div className="font-medium text-sm">{workflow.name}</div>
              <div className="text-xs text-muted-foreground">
                {workflow.description}
              </div>
            </div>
          </button>
        ))}
      </div>

      {/* Main content grid */}
      <div className="grid grid-cols-1 xl:grid-cols-[1fr_320px] gap-6">
        {/* Workflow form */}
        <div>
          {activeWorkflow === 'i2v' && <I2VForm />}
          {activeWorkflow === 'qwen' && <QwenForm />}
        </div>

        {/* Sidebar with job history */}
        <OutputGallery className="hidden xl:block" />
      </div>

      {/* Mobile job history */}
      <OutputGallery className="xl:hidden" />
    </div>
  )
}
