import { create } from "zustand";

export interface JobOutput {
  type: "video" | "image";
  path: string;
  frames?: number;
  seed?: number;
}

export interface Job {
  id: string;
  type: "i2v" | "qwen";
  status: "pending" | "running" | "completed" | "failed";
  progress: number;
  stage: string;
  preview?: string;
  output?: JobOutput;
  error?: string;
  params: Record<string, unknown>;
  createdAt: Date;
}

interface JobStore {
  jobs: Job[];
  activeJobId: string | null;

  addJob: (job: Omit<Job, "createdAt">) => void;
  updateJobProgress: (
    jobId: string,
    progress: number,
    stage: string,
    preview?: string,
  ) => void;
  completeJob: (jobId: string, output: JobOutput) => void;
  failJob: (jobId: string, error: string) => void;
  removeJob: (jobId: string) => void;
  setActiveJob: (jobId: string | null) => void;
  setJobs: (jobs: Job[]) => void;
  getJob: (jobId: string) => Job | undefined;
}

export const useJobStore = create<JobStore>((set, get) => ({
  jobs: [],
  activeJobId: null,

  addJob: (job) => {
    set((state) => ({
      jobs: [{ ...job, createdAt: new Date() }, ...state.jobs],
      activeJobId: job.id,
    }));
  },

  updateJobProgress: (jobId, progress, stage, preview) => {
    set((state) => ({
      jobs: state.jobs.map((job) =>
        job.id === jobId
          ? { ...job, status: "running", progress, stage, preview }
          : job,
      ),
    }));
  },

  completeJob: (jobId, output) => {
    set((state) => ({
      jobs: state.jobs.map((job) =>
        job.id === jobId
          ? {
              ...job,
              status: "completed",
              progress: 1,
              stage: "Complete",
              output,
            }
          : job,
      ),
    }));
  },

  failJob: (jobId, error) => {
    set((state) => ({
      jobs: state.jobs.map((job) =>
        job.id === jobId ? { ...job, status: "failed", error } : job,
      ),
    }));
  },

  removeJob: (jobId) => {
    set((state) => ({
      jobs: state.jobs.filter((job) => job.id !== jobId),
      activeJobId: state.activeJobId === jobId ? null : state.activeJobId,
    }));
  },

  setActiveJob: (jobId) => {
    set({ activeJobId: jobId });
  },

  setJobs: (jobs) => {
    set({ jobs });
  },

  getJob: (jobId) => {
    return get().jobs.find((job) => job.id === jobId);
  },
}));
