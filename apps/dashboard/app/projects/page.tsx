"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { useProject } from "@/lib/project-context";
import { createProject } from "@/lib/api";

const RETENTION_OPTIONS = [30, 90, 180, 365];

function NewProjectForm({ onCreated }: { onCreated: (trackingId: string) => void }) {
  const [name, setName] = useState("");
  const [domains, setDomains] = useState("");
  const [timezone, setTimezone] = useState("");
  const [retentionDays, setRetentionDays] = useState(90);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setFormError(null);
    setSaving(true);
    try {
      const allowed_domains = domains
        .split(",")
        .map((d) => d.trim())
        .filter(Boolean);
      const created = await createProject({
        name: name.trim(),
        ...(allowed_domains.length > 0 ? { allowed_domains } : {}),
        ...(timezone.trim() ? { timezone: timezone.trim() } : {}),
        retention_days: retentionDays,
      });
      onCreated(created.tracking_id);
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to create project");
    } finally {
      setSaving(false);
    }
  };

  const inputClass =
    "w-full rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm text-zinc-900 dark:text-zinc-100";

  return (
    <form
      onSubmit={submit}
      className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4 space-y-4 max-w-xl"
    >
      <h3 className="font-medium text-zinc-900 dark:text-zinc-100">New project</h3>

      <label className="block">
        <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
          Name
        </span>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="My site"
          required
          maxLength={255}
          className={inputClass}
        />
      </label>

      <label className="block">
        <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
          Allowed domains <span className="normal-case font-normal">(comma-separated, optional)</span>
        </span>
        <input
          value={domains}
          onChange={(e) => setDomains(e.target.value)}
          placeholder="example.com, www.example.com"
          className={`${inputClass} font-mono`}
        />
        <span className="mt-1 block text-xs text-zinc-500">
          Events from other domains are rejected. Leave empty to accept from anywhere while testing.
        </span>
      </label>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <label className="block">
          <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
            Timezone <span className="normal-case font-normal">(optional)</span>
          </span>
          <input
            value={timezone}
            onChange={(e) => setTimezone(e.target.value)}
            placeholder="Africa/Addis_Ababa"
            className={`${inputClass} font-mono`}
          />
        </label>

        <label className="block">
          <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
            Retention
          </span>
          <select
            value={retentionDays}
            onChange={(e) => setRetentionDays(Number(e.target.value))}
            className={inputClass}
          >
            {RETENTION_OPTIONS.map((days) => (
              <option key={days} value={days}>
                {days} days
              </option>
            ))}
          </select>
        </label>
      </div>

      {formError && (
        <p role="alert" className="text-sm text-red-600 dark:text-red-400">
          {formError}
        </p>
      )}

      <button
        type="submit"
        disabled={saving || !name.trim()}
        className="rounded-md bg-zinc-900 dark:bg-zinc-100 px-4 py-2 text-sm font-medium text-white dark:text-zinc-900 disabled:opacity-50"
      >
        {saving ? "Creating..." : "Create project"}
      </button>
    </form>
  );
}

export default function ProjectsPage() {
  const { projects, selectedTrackingId, selectProject, loading, error, refresh } = useProject();
  const [showForm, setShowForm] = useState(false);

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

  const formVisible = showForm || projects.length === 0;

  return (
    <div className="p-6 space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Projects</h2>
        <div className="flex gap-2">
          {projects.length > 0 && (
            <button
              onClick={() => setShowForm((v) => !v)}
              className="rounded-md border px-3 py-1.5 text-sm"
            >
              {showForm ? "Hide form" : "New project"}
            </button>
          )}
          <Link href="/install" className="rounded-md border px-3 py-1.5 text-sm underline">
            Install tracker
          </Link>
        </div>
      </div>

      {formVisible && (
        <NewProjectForm
          onCreated={(trackingId) => {
            setShowForm(false);
            refresh();
            selectProject(trackingId);
          }}
        />
      )}

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

      {projects.length > 0 && (
        <details className="text-sm text-zinc-600 dark:text-zinc-400">
          <summary className="cursor-pointer underline">Or create via the API</summary>
          <pre className="mt-3 overflow-x-auto rounded-md bg-zinc-950 p-3 font-mono text-xs text-zinc-100">
{`curl -X POST $API/v1/analytics/projects \\
  -H "Authorization: Bearer $KEY" \\
  -H 'Content-Type: application/json' \\
  -d '{"name":"My site","allowed_domains":["example.com"]}'`}
          </pre>
        </details>
      )}
    </div>
  );
}
