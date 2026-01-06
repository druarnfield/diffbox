import { useState, useCallback } from "react";
import { ImageUpload } from "./ImageUpload";
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
  { value: "1024x1024", label: "1024 x 1024 (Square)" },
  { value: "1328x1328", label: "1328 x 1328 (Square HD)" },
  { value: "1024x768", label: "1024 x 768 (4:3)" },
  { value: "768x1024", label: "768 x 1024 (3:4)" },
];

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

        <div className="space-y-2">
          <label className="label flex items-center justify-between">
            <span>Input Images</span>
            <span className="text-xs text-muted-foreground">
              {imageCount}/3 images
            </span>
          </label>
          <div className="grid grid-cols-3 gap-3">
            {[0, 1, 2].map((index) => (
              <ImageUpload
                key={index}
                value={form.images[index]}
                onChange={(file) => updateImage(index, file)}
                label=""
                className="min-h-0"
              />
            ))}
          </div>
          <p className="text-xs text-muted-foreground">
            Upload 1-3 reference images. Multiple images let you combine
            elements.
          </p>
        </div>

        <div className="space-y-2">
          <label className="label">Instruction</label>
          <textarea
            value={form.instruction}
            onChange={(e) =>
              setForm((s) => ({ ...s, instruction: e.target.value }))
            }
            className="input resize-none"
            rows={4}
            placeholder="Describe how you want to edit or combine the images..."
          />
        </div>

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
            <label className="label">Resolution</label>
            <select
              value={form.resolution}
              onChange={(e) =>
                setForm((s) => ({ ...s, resolution: e.target.value }))
              }
              className="input"
            >
              {RESOLUTIONS.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
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
            Advanced Settings
          </summary>
          <div className="grid grid-cols-2 gap-4 mt-4">
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
              Submitting...
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
                  d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                />
              </svg>
              Generate Image
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
            <p className="text-muted-foreground text-sm">
              Output will appear here
            </p>
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
