"use client";

import { useProject } from "@/lib/project-context";

const selectClass =
  "w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100";

export default function ProjectSelector() {
  const { projects, selectedTrackingId, selectProject, loading } = useProject();

  if (loading) {
    return (
      <div className="rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-500 shadow-xs dark:border-zinc-800 dark:bg-zinc-900">
        Loading projects…
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-zinc-300 px-3 py-2 text-sm text-zinc-500 dark:border-zinc-700">
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
        className={selectClass}
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
