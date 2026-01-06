import { useState, useCallback, useRef } from "react";
import { Progress } from "./Progress";
import { submitQwen } from "@/api/workflows";
import { useJobStore } from "@/stores/jobStore";
import { fileToBase64, cn } from "@/lib/utils";

interface QwenFormProps {
  className?: string;
}

interface FormState {
  images: (File | null)[];
  instruction: string;
  seed: string;
  resolution: string;
  steps: number;
  cfgScale: number;
}

const RESOLUTIONS = [
  { value: "1024x1024", label: "1024×1024", aspect: "Square" },
  { value: "1328x1328", label: "1328×1328", aspect: "Square HD" },
  { value: "1024x768", label: "1024×768", aspect: "4:3" },
  { value: "768x1024", label: "768×1024", aspect: "3:4" },
];

// Compact image upload slot component
function ImageSlot({
  file,
  index,
  isPrimary,
  onSelect,
  onRemove,
}: {
  file: File | null;
  index: number;
  isPrimary?: boolean;
  onSelect: (file: File) => void;
  onRemove: () => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);

  const handleFile = useCallback(
    (f: File) => {
      onSelect(f);
      const reader = new FileReader();
      reader.onload = (e) => setPreview(e.target?.result as string);
      reader.readAsDataURL(f);
    },
    [onSelect],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setIsDragging(false);
      const f = e.dataTransfer.files[0];
      if (f?.type.startsWith("image/")) handleFile(f);
    },
    [handleFile],
  );

  const handleRemove = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      onRemove();
      setPreview(null);
      if (inputRef.current) inputRef.current.value = "";
    },
    [onRemove],
  );

  // Sync preview with external file state
  const displayPreview = file ? preview : null;

  return (
    <div
      onClick={() => inputRef.current?.click()}
      onDrop={handleDrop}
      onDragOver={(e) => {
        e.preventDefault();
        setIsDragging(true);
      }}
      onDragLeave={() => setIsDragging(false)}
      className={cn(
        "relative rounded-lg border-2 border-dashed transition-all cursor-pointer overflow-hidden group",
        isPrimary ? "aspect-square" : "aspect-square",
        isDragging
          ? "border-primary bg-primary/10 border-solid"
          : displayPreview
            ? "border-primary/30 border-solid"
            : "border-border/60 hover:border-primary/40 hover:bg-primary/5",
      )}
    >
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) handleFile(f);
        }}
        className="hidden"
      />

      {displayPreview ? (
        <>
          <img
            src={displayPreview}
            alt=""
            className="w-full h-full object-cover"
          />
          <div className="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition-colors" />
          <button
            onClick={handleRemove}
            className="absolute top-2 right-2 p-1.5 bg-black/60 rounded-md text-white/80 hover:text-white hover:bg-black/80 transition-all opacity-0 group-hover:opacity-100"
          >
            <svg
              className="w-3.5 h-3.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
          {isPrimary && (
            <div className="absolute bottom-2 left-2 px-2 py-0.5 bg-black/60 rounded text-[10px] text-white/80 font-medium uppercase tracking-wide">
              Primary
            </div>
          )}
        </>
      ) : (
        <div className="absolute inset-0 flex flex-col items-center justify-center p-3">
          {isPrimary ? (
            <>
              <div className="w-10 h-10 rounded-full bg-secondary/80 flex items-center justify-center mb-2">
                <svg
                  className="w-5 h-5 text-muted-foreground"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={1.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
              </div>
              <span className="text-xs text-muted-foreground text-center">
                Drop image or click
              </span>
            </>
          ) : (
            <div className="w-8 h-8 rounded-full bg-secondary/50 flex items-center justify-center">
              <svg
                className="w-4 h-4 text-muted-foreground/60"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={1.5}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 4v16m8-8H4"
                />
              </svg>
            </div>
          )}
        </div>
      )}

      {/* Slot number indicator */}
      {!isPrimary && !displayPreview && (
        <div className="absolute bottom-1.5 right-1.5 w-5 h-5 rounded bg-secondary/80 flex items-center justify-center">
          <span className="text-[10px] text-muted-foreground font-medium">
            {index + 1}
          </span>
        </div>
      )}
    </div>
  );
}

export function QwenForm({ className }: QwenFormProps) {
  const { addJob, jobs, activeJobId } = useJobStore();
  const activeJob = jobs.find((j) => j.id === activeJobId);

  const [form, setForm] = useState<FormState>({
    images: [null, null, null],
    instruction: "",
    seed: "",
    resolution: "1024x1024",
    steps: 4,
    cfgScale: 1.0,
  });

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateImage = useCallback((index: number, file: File | null) => {
    setForm((s) => {
      const images = [...s.images];
      images[index] = file;
      return { ...s, images };
    });
  }, []);

  const handleSubmit = useCallback(async () => {
    const validImages = form.images.filter((img): img is File => img !== null);
    if (validImages.length === 0) {
      setError("Please select at least one image");
      return;
    }
    if (!form.instruction.trim()) {
      setError("Please enter an instruction");
      return;
    }

    setError(null);
    setIsSubmitting(true);

    try {
      const imagesBase64 = await Promise.all(
        validImages.map((img) => fileToBase64(img)),
      );
      const [width, height] = form.resolution.split("x").map(Number);

      const response = await submitQwen({
        edit_images: imagesBase64,
        instruction: form.instruction.trim(),
        seed: form.seed ? parseInt(form.seed, 10) : undefined,
        width,
        height,
        num_inference_steps: form.steps,
        cfg_scale: form.cfgScale,
      });

      addJob({
        id: response.id,
        type: "qwen",
        status: "pending",
        progress: 0,
        stage: "Queued",
        params: { instruction: form.instruction },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to submit job");
    } finally {
      setIsSubmitting(false);
    }
  }, [form, addJob]);

  const showingActiveJob =
    activeJob &&
    (activeJob.status === "running" || activeJob.status === "completed");
  const imageCount = form.images.filter(Boolean).length;

  return (
    <div className={cn("grid grid-cols-1 lg:grid-cols-2 gap-6", className)}>
      {/* Input Panel */}
      <div className="card p-6 space-y-5">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <svg
            className="w-5 h-5 text-primary"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
            />
          </svg>
          Image Edit
        </h2>

        {/* Redesigned image upload - primary + secondary slots */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <label className="label mb-0">Reference Images</label>
            <span
              className={cn(
                "text-xs px-2 py-0.5 rounded-full transition-colors",
                imageCount > 0
                  ? "bg-primary/10 text-primary"
                  : "bg-secondary text-muted-foreground",
              )}
            >
              {imageCount}/3
            </span>
          </div>

          {/* Image slots in asymmetric grid */}
          <div className="grid grid-cols-[1fr_auto] gap-3">
            {/* Primary large slot */}
            <ImageSlot
              file={form.images[0]}
              index={0}
              isPrimary
              onSelect={(f) => updateImage(0, f)}
              onRemove={() => updateImage(0, null)}
            />

            {/* Secondary slots stacked */}
            <div className="flex flex-col gap-3 w-24">
              <ImageSlot
                file={form.images[1]}
                index={1}
                onSelect={(f) => updateImage(1, f)}
                onRemove={() => updateImage(1, null)}
              />
              <ImageSlot
                file={form.images[2]}
                index={2}
                onSelect={(f) => updateImage(2, f)}
                onRemove={() => updateImage(2, null)}
              />
            </div>
          </div>

          <p className="text-xs text-muted-foreground">
            Add up to 3 images to edit or combine. Primary image has most
            influence.
          </p>
        </div>

        {/* Instruction */}
        <div className="space-y-2">
          <label className="label">Edit Instruction</label>
          <textarea
            value={form.instruction}
            onChange={(e) =>
              setForm((s) => ({ ...s, instruction: e.target.value }))
            }
            className="input resize-none"
            rows={3}
            placeholder='Describe how to edit: "change background to sunset", "remove the person", "combine into one scene"...'
          />
        </div>

        {/* Settings row */}
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
            <label className="label">Output Size</label>
            <select
              value={form.resolution}
              onChange={(e) =>
                setForm((s) => ({ ...s, resolution: e.target.value }))
              }
              className="input"
            >
              {RESOLUTIONS.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label} ({r.aspect})
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Advanced Settings */}
        <details className="group">
          <summary className="cursor-pointer text-sm font-medium text-muted-foreground hover:text-foreground transition-colors flex items-center gap-2">
            <svg
              className="w-4 h-4 transition-transform group-open:rotate-90"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 5l7 7-7 7"
              />
            </svg>
            Advanced
          </summary>
          <div className="grid grid-cols-2 gap-4 mt-4 pt-4 border-t border-border/50">
            <div className="space-y-2">
              <label className="label">Steps</label>
              <input
                type="number"
                value={form.steps}
                onChange={(e) =>
                  setForm((s) => ({
                    ...s,
                    steps: parseInt(e.target.value, 10) || 4,
                  }))
                }
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
                onChange={(e) =>
                  setForm((s) => ({
                    ...s,
                    cfgScale: parseFloat(e.target.value) || 1,
                  }))
                }
                className="input"
                step={0.1}
                min={1}
                max={20}
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
          disabled={isSubmitting || imageCount === 0}
          className={cn(
            "btn-primary w-full py-3",
            isSubmitting && "animate-pulse-glow",
          )}
        >
          {isSubmitting ? (
            <>
              <svg
                className="w-4 h-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              Generating...
            </>
          ) : (
            <>
              <svg
                className="w-5 h-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"
                />
              </svg>
              Generate Edit
            </>
          )}
        </button>
      </div>

      {/* Output Panel */}
      <div className="card p-6 space-y-4">
        <h2 className="text-lg font-semibold">Output</h2>

        <div className="output-preview aspect-square">
          {showingActiveJob && activeJob.output?.type === "image" ? (
            <img
              src={`/outputs/${activeJob.id}.png`}
              alt="Output"
              className="w-full h-full object-contain"
            />
          ) : showingActiveJob && activeJob.preview ? (
            <img
              src={`data:image/jpeg;base64,${activeJob.preview}`}
              alt="Preview"
              className="w-full h-full object-contain"
            />
          ) : activeJob?.status === "running" ? (
            <div className="text-center">
              <div className="w-12 h-12 mx-auto border-3 border-primary/30 border-t-primary rounded-full animate-spin mb-3" />
              <p className="text-sm text-muted-foreground">{activeJob.stage}</p>
            </div>
          ) : (
            <div className="text-center">
              <svg
                className="w-12 h-12 mx-auto text-muted-foreground/30 mb-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={1}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                />
              </svg>
              <p className="text-muted-foreground text-sm">
                Output will appear here
              </p>
            </div>
          )}
        </div>

        {activeJob &&
          (activeJob.status === "running" ||
            activeJob.status === "pending") && (
            <Progress value={activeJob.progress} stage={activeJob.stage} />
          )}

        {activeJob?.status === "completed" && activeJob.output && (
          <a
            href={`/outputs/${activeJob.id}.png`}
            download
            className="btn-secondary w-full justify-center"
          >
            <svg
              className="w-4 h-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
              />
            </svg>
            Download Image
          </a>
        )}

        {activeJob?.status === "failed" && (
          <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/30 text-destructive text-sm">
            {activeJob.error || "Generation failed"}
          </div>
        )}
      </div>
    </div>
  );
}
