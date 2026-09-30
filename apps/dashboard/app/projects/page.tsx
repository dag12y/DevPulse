"use client";

import { useEffect, useState } from "react";
import { getProjects, type Project } from "@/lib/api";

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getProjects()
      .then(setProjects)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-zinc-500">Loading projects...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load projects</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Projects</h2>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {projects.map((project) => (
          <div key={project.id} className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
            <div className="flex items-start justify-between">
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
              <p>Domains: {project.allowed_domains.join(", ") || "None"}</p>
            </div>
          </div>
        ))}
      </div>

      {projects.length === 0 && (
        <p className="text-zinc-500 text-center py-8">No projects yet. Create one via the API.</p>
      )}
    </div>
  );
}
