"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { useProject } from "@/lib/project-context";
import { createProject } from "@/lib/api";
import Badge from "@/components/ui/Badge";
import Card from "@/components/ui/Card";
import PageHeader from "@/components/ui/PageHeader";

const RETENTION_OPTIONS = [30, 90, 180, 365];

const inputClass =
  "w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500";

function NewProjectForm({ onCreated, onCancel }: { onCreated: (trackingId: string) => void; onCancel: () => void }) {
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

  return (
    <Card as="form" onSubmit={submit} className="max-w-xl space-y-4">
      <h3 className="font-semibold text-zinc-900 dark:text-white">New project</h3>

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
            aria-label="Retention in days"
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
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-300">
          {formError}
        </p>
      )}

      <div className="flex gap-2">
        <button
          type="submit"
          disabled={saving || !name.trim()}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 disabled:opacity-50 dark:bg-indigo-500 dark:hover:bg-indigo-400"
        >
          {saving ? "Creating..." : "Create project"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
        >
          Cancel
        </button>
      </div>
    </Card>
  );
}

export default function ProjectsPage() {
  const { projects, selectedTrackingId, selectProject, refresh } = useProject();
  const [showForm, setShowForm] = useState(false);

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Projects"
        description="Create tracking projects, switch context, and grab install snippets."
        actions={
          !showForm ? (
            <button
              onClick={() => setShowForm(true)}
              className="rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 dark:bg-indigo-500 dark:hover:bg-indigo-400"
            >
              New project
            </button>
          ) : undefined
        }
      />

      <div className="rounded-2xl border border-indigo-200 bg-indigo-50/60 p-4 text-sm text-indigo-900 dark:border-indigo-800/60 dark:bg-indigo-950/40 dark:text-indigo-200">
        <p className="font-semibold">About tracking IDs</p>
        <p className="mt-1">
          The tracking ID (<span className="font-mono">dp_…</span>) is public and goes in your site&apos;s
          script tag. Keep the secret API key on the server — it reads this dashboard data.
        </p>
        <div className="mt-2">
          <Link className="font-medium underline underline-offset-2" href="/install">
            Install tracker
          </Link>
        </div>
      </div>

      {showForm && (
        <NewProjectForm
          onCancel={() => setShowForm(false)}
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
            <Card
              key={project.id}
              className={isSelected ? "border-indigo-500 ring-1 ring-indigo-500 dark:border-indigo-400" : ""}
            >
              <div className="flex items-start justify-between gap-2">
                <h3 className="font-semibold text-zinc-900 dark:text-white">{project.name}</h3>
                <Badge tone={project.enabled ? "success" : "neutral"}>
                  {project.enabled ? "Active" : "Disabled"}
                </Badge>
              </div>
              <p className="text-sm text-zinc-500 mt-1 font-mono">{project.tracking_id}</p>
              <div className="mt-3 space-y-1 text-sm text-zinc-600 dark:text-zinc-400">
                <p>Timezone: {project.timezone}</p>
                <p>Retention: {project.retention_days} days</p>
                <p>Domains: {project.allowed_domains.join(", ") || "All domains"}</p>
              </div>
              <div className="mt-4 flex flex-wrap gap-2">
                <button
                  onClick={() => selectProject(project.tracking_id)}
                  disabled={isSelected}
                  className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium disabled:opacity-50 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
                >
                  {isSelected ? "Selected" : "Select"}
                </button>
                <Link
                  href={`/install?project=${encodeURIComponent(project.tracking_id)}`}
                  className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium text-indigo-600 hover:bg-indigo-50 dark:border-zinc-700 dark:text-indigo-400 dark:hover:bg-indigo-950/40"
                >
                  Install
                </Link>
                <Link
                  href={`/settings?project=${encodeURIComponent(project.tracking_id)}`}
                  onClick={() => selectProject(project.tracking_id)}
                  className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
                >
                  Settings
                </Link>
              </div>
            </Card>
          );
        })}
      </div>

      {projects.length > 0 && (
        <details className="text-sm text-zinc-600 dark:text-zinc-400">
          <summary className="cursor-pointer font-medium text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400">Or create via the API</summary>
          <pre className="mt-3 overflow-x-auto rounded-xl bg-zinc-950 p-3 font-mono text-xs text-zinc-100">
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
