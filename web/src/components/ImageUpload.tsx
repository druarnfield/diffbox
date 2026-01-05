import { useState, useCallback, useRef } from 'react'
import { cn } from '@/lib/utils'

interface ImageUploadProps {
  value?: File | null
  onChange: (file: File | null) => void
  label?: string
  className?: string
  accept?: string
}

export function ImageUpload({
  value: _value,  // Controlled value for external state
  onChange,
  label = 'Input Image',
  className,
  accept = 'image/*',
}: ImageUploadProps) {
  const [isDragging, setIsDragging] = useState(false)
  const [preview, setPreview] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleFile = useCallback(
    (file: File | null) => {
      onChange(file)
      if (file) {
        const reader = new FileReader()
        reader.onload = (e) => setPreview(e.target?.result as string)
        reader.readAsDataURL(file)
      } else {
        setPreview(null)
      }
    },
    [onChange]
  )

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setIsDragging(false)
      const file = e.dataTransfer.files[0]
      if (file && file.type.startsWith('image/')) {
        handleFile(file)
      }
    },
    [handleFile]
  )

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback(() => {
    setIsDragging(false)
  }, [])

  const handleClick = useCallback(() => {
    inputRef.current?.click()
  }, [])

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0] || null
      handleFile(file)
    },
    [handleFile]
  )

  const handleRemove = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation()
      handleFile(null)
      if (inputRef.current) {
        inputRef.current.value = ''
      }
    },
    [handleFile]
  )

  return (
    <div className={cn('space-y-2', className)}>
      {label && <label className="label">{label}</label>}
      <div
        onClick={handleClick}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        className={cn(
          'upload-zone min-h-[160px]',
          isDragging && 'dragover',
          preview && 'has-file p-0'
        )}
      >
        <input
          ref={inputRef}
          type="file"
          accept={accept}
          onChange={handleInputChange}
          className="hidden"
        />

        {preview ? (
          <div className="relative w-full h-full">
            <img
              src={preview}
              alt="Preview"
              className="w-full h-full object-contain"
            />
            <button
              onClick={handleRemove}
              className="absolute top-2 right-2 p-1.5 bg-black/60 rounded-lg text-white/80 hover:text-white hover:bg-black/80 transition-colors"
              aria-label="Remove image"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        ) : (
          <>
            <div className="w-12 h-12 rounded-full bg-secondary flex items-center justify-center">
              <svg className="w-6 h-6 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
            </div>
            <div className="text-center">
              <p className="text-sm text-foreground/80">
                Drop an image here or <span className="text-primary">click to browse</span>
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                PNG, JPG, WebP up to 10MB
              </p>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
