const API_BASE = '/api'

export interface I2VParams {
  input_image: string // base64
  prompt: string
  negative_prompt?: string
  seed?: number
  height?: number
  width?: number
  num_frames?: number
  num_inference_steps?: number
  cfg_scale?: number
  denoising_strength?: number
}

export interface QwenParams {
  edit_images: string[] // base64 array (up to 3)
  instruction: string
  seed?: number
  height?: number
  width?: number
  num_inference_steps?: number
  cfg_scale?: number
}

export interface JobResponse {
  id: string
  status: string
}

export async function submitI2V(params: I2VParams): Promise<JobResponse> {
  const response = await fetch(`${API_BASE}/workflows/i2v`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(error || 'Failed to submit I2V job')
  }

  return response.json()
}

export async function submitQwen(params: QwenParams): Promise<JobResponse> {
  const response = await fetch(`${API_BASE}/workflows/qwen`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })

  if (!response.ok) {
    const error = await response.text()
    throw new Error(error || 'Failed to submit Qwen job')
  }

  return response.json()
}

export async function cancelJob(jobId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/jobs/${jobId}`, {
    method: 'DELETE',
  })

  if (!response.ok) {
    throw new Error('Failed to cancel job')
  }
}
