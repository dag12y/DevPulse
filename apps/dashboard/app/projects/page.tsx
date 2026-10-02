"use client";

import Link from "next/link";
import { useProject } from "@/lib/project-context";

export default function ProjectsPage() {
  const { projects, selectedTrackingId, selectProject, loading, error, refresh } = useProject();

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Projects</h2>
        <div className="flex items-center justify-center h-64">
          <p className="text-zinc-500">Loading projects...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 space-y-6">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Projects</h2>
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load projects</p>
          <p className="text-sm mt-1">{error}</p>
          <button onClick={refresh} className="mt-3 rounded-md border px-3 py-1.5 text-sm underline">
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Projects</h2>
        <Link href="/install" className="rounded-md border px-3 py-1.5 text-sm underline">
          Install tracker
        </Link>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {projects.map((project) => {
          const isSelected = project.tracking_id === selectedTrackingId;
          return (
            <div
              key={project.id}
              className={`rounded-lg border p-4 ${
                isSelected
                  ? "border-blue-500 dark:border-blue-400"
                  : "border-zinc-200 dark:border-zinc-800"
              }`}
            >
              <div className="flex items-start justify-between gap-2">
                <h3 className="font-medium text-zinc-900 dark:text-zinc-100">{project.name}</h3>
                <span
                  className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                    project.enabled
                      ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                      : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400"
                  }`}
                >
                  {project.enabled ? "Active" : "Disabled"}
                </span>
              </div>
              <p className="text-sm text-zinc-500 mt-1 font-mono">{project.tracking_id}</p>
              <div className="mt-3 space-y-1 text-sm text-zinc-600 dark:text-zinc-400">
                <p>Timezone: {project.timezone}</p>
                <p>Retention: {project.retention_days} days</p>
                <p>Domains: {project.allowed_domains.join(", ") || "All domains"}</p>
              </div>
              <div className="mt-4 flex gap-2">
                <button
                  onClick={() => selectProject(project.tracking_id)}
                  disabled={isSelected}
                  className="rounded-md border px-3 py-1.5 text-sm disabled:opacity-50"
                >
                  {isSelected ? "Selected" : "Select"}
                </button>
                <Link
                  href={`/install?project=${encodeURIComponent(project.tracking_id)}`}
                  className="rounded-md border px-3 py-1.5 text-sm underline"
                >
                  Install
                </Link>
              </div>
            </div>
          );
        })}
      </div>

      {projects.length === 0 && (
        <div className="rounded-lg border border-dashed border-zinc-300 dark:border-zinc-700 p-6 text-sm text-zinc-600 dark:text-zinc-400">
          <p className="font-medium text-zinc-900 dark:text-zinc-100">No projects yet</p>
          <p className="mt-1">Create one via the API, then select it here:</p>
          <pre className="mt-3 overflow-x-auto rounded-md bg-zinc-950 p-3 font-mono text-xs text-zinc-100">
{`curl -X POST $API/v1/analytics/projects \\
  -H "Authorization: Bearer $KEY" \\
  -H 'Content-Type: application/json' \\
  -d '{"name":"My site","allowed_domains":["example.com"]}'`}
          </pre>
        </div>
      )}
    </div>
  );
}
