import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";

type ModelType = "all" | "checkpoint" | "lora" | "vae" | "controlnet";
type Tab = "browse" | "local" | "downloads";

interface DownloadStatus {
  name: string;
  url: string;
  status:
    | "complete"
    | "downloading"
    | "queued"
    | "missing"
    | "active"
    | "waiting";
  progress: number;
  total_size: number;
  completed_size: number;
  download_speed: number;
  workflow: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
}

function formatSpeed(bytesPerSec: number): string {
  return formatBytes(bytesPerSec) + "/s";
}

// Compact progress indicator for core models
function CoreModelIndicator({ download }: { download: DownloadStatus }) {
  const isComplete = download.status === "complete";
  const isActive =
    download.status === "downloading" || download.status === "active";
  const isWaiting =
    download.status === "waiting" || download.status === "queued";

  return (
    <div className="flex items-center gap-3 min-w-0">
      {/* Status indicator */}
      <div className="relative flex-shrink-0">
        {isComplete ? (
          <div className="w-5 h-5 rounded-full bg-emerald-500/20 flex items-center justify-center">
            <svg
              className="w-3 h-3 text-emerald-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={3}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
        ) : isActive ? (
          <div className="w-5 h-5 relative">
            <svg className="w-5 h-5 -rotate-90" viewBox="0 0 20 20">
              <circle
                cx="10"
                cy="10"
                r="8"
                fill="none"
                stroke="hsl(var(--secondary))"
                strokeWidth="2"
              />
              <circle
                cx="10"
                cy="10"
                r="8"
                fill="none"
                stroke="hsl(var(--primary))"
                strokeWidth="2"
                strokeDasharray={`${download.progress * 0.5} 50`}
                strokeLinecap="round"
                className="transition-all duration-300"
              />
            </svg>
          </div>
        ) : isWaiting ? (
          <div
            className="w-5 h-5 rounded-full border-2 border-dashed border-muted-foreground/40 animate-spin"
            style={{ animationDuration: "3s" }}
          />
        ) : (
          <div className="w-5 h-5 rounded-full border-2 border-dashed border-muted-foreground/30" />
        )}
      </div>

      {/* Model name */}
      <span
        className={cn(
          "text-sm truncate",
          isComplete ? "text-foreground/70" : "text-foreground",
        )}
      >
        {download.name.replace(/\.safetensors|\.bin|\.pth/g, "")}
      </span>

      {/* Progress info for active downloads */}
      {isActive && (
        <span className="text-xs text-primary font-mono ml-auto flex-shrink-0">
          {download.progress.toFixed(0)}%
          {download.download_speed > 0 && (
            <span className="text-muted-foreground ml-2">
              {formatSpeed(download.download_speed)}
            </span>
          )}
        </span>
      )}

      {/* Size for complete */}
      {isComplete && (
        <span className="text-xs text-muted-foreground ml-auto flex-shrink-0">
          {formatBytes(download.completed_size)}
        </span>
      )}
    </div>
  );
}

// Grouped core models section
function CoreModelsSection({ downloads }: { downloads: DownloadStatus[] }) {
  const coreModels = downloads.filter(
    (d) =>
      d.workflow === "i2v" ||
      d.workflow === "qwen" ||
      d.workflow === "svi" ||
      d.workflow === "shared",
  );

  if (coreModels.length === 0) return null;

  const completeCount = coreModels.filter(
    (d) => d.status === "complete",
  ).length;
  const activeCount = coreModels.filter(
    (d) => d.status === "downloading" || d.status === "active",
  ).length;
  const totalProgress =
    coreModels.reduce(
      (acc, d) => acc + (d.status === "complete" ? 100 : d.progress),
      0,
    ) / coreModels.length;

  const isAllComplete = completeCount === coreModels.length;

  // Group by workflow
  const grouped = coreModels.reduce(
    (acc, model) => {
      const key = model.workflow;
      if (!acc[key]) acc[key] = [];
      acc[key].push(model);
      return acc;
    },
    {} as Record<string, DownloadStatus[]>,
  );

  const workflowLabels: Record<string, string> = {
    shared: "Shared",
    i2v: "Wan I2V",
    svi: "Wan SVI",
    qwen: "Qwen Edit",
  };

  return (
    <div className="card overflow-hidden">
      {/* Header with overall progress */}
      <div className="px-5 py-4 border-b border-border/50 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div
            className={cn(
              "w-2 h-2 rounded-full",
              isAllComplete
                ? "bg-emerald-400"
                : activeCount > 0
                  ? "bg-primary animate-pulse"
                  : "bg-muted-foreground/40",
            )}
          />
          <h2 className="font-medium">Core Models</h2>
          <span className="text-xs text-muted-foreground">
            {completeCount}/{coreModels.length} ready
          </span>
        </div>

        {!isAllComplete && (
          <div className="flex items-center gap-3">
            <div className="w-24 h-1.5 bg-secondary rounded-full overflow-hidden">
              <div
                className="h-full bg-primary rounded-full transition-all duration-500"
                style={{ width: `${totalProgress}%` }}
              />
            </div>
            <span className="text-xs font-mono text-muted-foreground w-10 text-right">
              {totalProgress.toFixed(0)}%
            </span>
          </div>
        )}
      </div>

      {/* Model list grouped by workflow */}
      <div className="divide-y divide-border/30">
        {Object.entries(grouped).map(([workflow, models]) => (
          <div key={workflow} className="px-5 py-3">
            <div className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">
              {workflowLabels[workflow] || workflow}
            </div>
            <div className="space-y-2">
              {models.map((download) => (
                <CoreModelIndicator key={download.name} download={download} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function ModelsPage() {
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<ModelType>("all");
  const [activeTab, setActiveTab] = useState<Tab>("downloads");
  const [downloads, setDownloads] = useState<DownloadStatus[]>([]);
  const [loading, setLoading] = useState(false);

  // Fetch download status
  useEffect(() => {
    if (activeTab !== "downloads") return;

    const fetchDownloads = async () => {
      try {
        setLoading(true);
        const res = await fetch("/api/downloads");
        const data = await res.json();
        setDownloads(data || []);
      } catch (err) {
        console.error("Failed to fetch downloads:", err);
      } finally {
        setLoading(false);
      }
    };

    fetchDownloads();
    const interval = setInterval(fetchDownloads, 3000); // Poll every 3 seconds

    return () => clearInterval(interval);
  }, [activeTab]);

  const types: { id: ModelType; label: string }[] = [
    { id: "all", label: "All" },
    { id: "checkpoint", label: "Checkpoints" },
    { id: "lora", label: "LoRAs" },
    { id: "vae", label: "VAEs" },
    { id: "controlnet", label: "ControlNets" },
  ];

  const getStatusColor = (status: string) => {
    switch (status) {
      case "complete":
        return "text-emerald-400 bg-emerald-500/10 border-emerald-500/20";
      case "downloading":
      case "active":
        return "text-primary bg-primary/10 border-primary/20";
      case "waiting":
      case "queued":
        return "text-amber-400 bg-amber-500/10 border-amber-500/20";
      case "missing":
        return "text-rose-400 bg-rose-500/10 border-rose-500/20";
      default:
        return "text-muted-foreground bg-secondary border-border";
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Models</h1>
        <div className="flex gap-2">
          <button className="btn-secondary text-sm">Sync Metadata</button>
        </div>
      </div>

      {/* Search and filters - only show on Browse/Local tabs */}
      {activeTab !== "downloads" && (
        <div className="flex gap-4">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search models..."
            className="input flex-1"
          />
          <div className="flex gap-1 p-1 bg-secondary/50 rounded-lg border border-border/50">
            {types.map((type) => (
              <button
                key={type.id}
                onClick={() => setTypeFilter(type.id)}
                className={cn(
                  "px-3 py-1.5 rounded-md text-sm transition-all",
                  typeFilter === type.id
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {type.label}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Tabs: Browse / Local / Downloads */}
      <div className="border-b border-border/50">
        <div className="flex gap-1">
          {[
            { id: "browse", label: "Browse" },
            { id: "local", label: "Local" },
            { id: "downloads", label: "Downloads" },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as Tab)}
              className={cn(
                "px-4 py-2.5 text-sm font-medium transition-all relative",
                activeTab === tab.id
                  ? "text-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {tab.label}
              {activeTab === tab.id && (
                <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary rounded-full" />
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Download status tab */}
      {activeTab === "downloads" && (
        <div className="space-y-6">
          {loading && downloads.length === 0 ? (
            <div className="card p-12 text-center text-muted-foreground">
              <div className="w-6 h-6 mx-auto border-2 border-primary/30 border-t-primary rounded-full animate-spin mb-3" />
              Loading downloads...
            </div>
          ) : downloads.length === 0 ? (
            <div className="card p-12 text-center text-muted-foreground">
              No required models found
            </div>
          ) : (
            <>
              {/* Core models summary section */}
              <CoreModelsSection downloads={downloads} />

              {/* Detailed download list */}
              <div className="space-y-3">
                <h3 className="text-sm font-medium text-muted-foreground">
                  All Downloads
                </h3>
                {downloads.map((download) => (
                  <div key={download.name} className="card p-4 space-y-3">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 min-w-0">
                        <h3 className="font-medium truncate">
                          {download.name}
                        </h3>
                        <p className="text-sm text-muted-foreground mt-0.5">
                          {download.workflow}
                        </p>
                      </div>
                      <span
                        className={cn(
                          "px-2.5 py-1 text-xs font-medium rounded-full border flex-shrink-0",
                          getStatusColor(download.status),
                        )}
                      >
                        {download.status}
                      </span>
                    </div>

                    {/* Progress bar */}
                    {(download.status === "downloading" ||
                      download.status === "active" ||
                      download.progress > 0) && (
                      <div className="space-y-1.5">
                        <div className="flex justify-between text-sm">
                          <span className="text-muted-foreground">
                            {formatBytes(download.completed_size)} /{" "}
                            {formatBytes(download.total_size)}
                          </span>
                          <span className="font-mono text-foreground">
                            {download.progress.toFixed(1)}%
                          </span>
                        </div>
                        <div className="progress-bar">
                          <div
                            className="progress-bar-fill"
                            style={{ width: `${download.progress}%` }}
                          />
                        </div>
                        {download.download_speed > 0 && (
                          <div className="text-xs text-muted-foreground">
                            {formatSpeed(download.download_speed)}
                          </div>
                        )}
                      </div>
                    )}

                    {download.status === "complete" && (
                      <div className="text-sm text-muted-foreground">
                        {formatBytes(download.completed_size)}
                      </div>
                    )}

                    {download.status === "missing" && (
                      <div className="text-sm text-muted-foreground">
                        Not downloaded · {formatBytes(download.total_size)}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {/* Browse/Local tabs - placeholder */}
      {(activeTab === "browse" || activeTab === "local") && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="card overflow-hidden group">
              <div className="aspect-square bg-secondary/50 relative overflow-hidden">
                <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
              </div>
              <div className="p-4 space-y-2">
                <h3 className="font-medium">Model Name</h3>
                <p className="text-sm text-muted-foreground">
                  Author · LoRA · Wan 2.2
                </p>
                <div className="flex items-center justify-between pt-1">
                  <span className="text-xs text-muted-foreground">
                    10.5k downloads
                  </span>
                  <button className="btn-secondary text-xs py-1.5 px-3">
                    Download
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
