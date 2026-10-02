"use client";

import { useProject } from "@/lib/project-context";

export default function ProjectSelector() {
  const { projects, selectedTrackingId, selectProject, loading } = useProject();

  if (loading) {
    return (
      <div className="rounded-md border border-zinc-200 dark:border-zinc-800 px-3 py-2 text-sm text-zinc-500">
        Loading projects…
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-zinc-300 dark:border-zinc-700 px-3 py-2 text-sm text-zinc-500">
        No projects yet
      </div>
    );
  }

  return (
    <label className="block">
      <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500 dark:text-zinc-400">
        Project
      </span>
      <select
        aria-label="Select analytics project"
        value={selectedTrackingId ?? ""}
        onChange={(e) => selectProject(e.target.value)}
        className="w-full rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm text-zinc-900 dark:text-zinc-100"
      >
        {projects.map((project) => (
          <option key={project.tracking_id} value={project.tracking_id}>
            {project.name} · {project.tracking_id}
          </option>
        ))}
      </select>
    </label>
  );
}
