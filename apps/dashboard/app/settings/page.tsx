"use client";

import { useState, type FormEvent, type ReactNode } from "react";
import { deleteProject, updateProject, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import Card from "@/components/ui/Card";

const RETENTION_OPTIONS = [30, 90, 180, 365];

const inputClass =
  "w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500";

function SectionCard({
  title,
  danger = false,
  children,
}: {
  title: string;
  danger?: boolean;
  children: ReactNode;
}) {
  return (
    <Card as="section" className={`max-w-xl space-y-4 ${danger ? "border-red-200 dark:border-red-900/60" : ""}`}>
      <h3 className={`font-semibold ${danger ? "text-red-700 dark:text-red-300" : "text-zinc-900 dark:text-white"}`}>
        {title}
      </h3>
      {children}
    </Card>
  );
}

export default function SettingsPage() {
  const { selectedProject, loading, error, refresh } = useProject();
  const { selectedMembership } = useAuth();
  const canEdit = selectedMembership?.role !== "viewer";

  if (loading) {
    return <ReportLoading title="Settings" />;
  }

  if (error) {
    return <ErrorState title="Settings" message={error} onRetry={refresh} />;
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Settings" showDateRange={false} />
      {selectedProject && (
        <>
          <GeneralSettings
            key={`general-${selectedProject.id}`}
            project={selectedProject}
            canEdit={canEdit}
            onSaved={refresh}
          />
          <TrackingDetails key={`tracking-${selectedProject.id}`} project={selectedProject} />
          <DangerZone
            key={`danger-${selectedProject.id}`}
            project={selectedProject}
            canEdit={canEdit}
            onDeleted={refresh}
          />
        </>
      )}
    </div>
  );
}

function TrackingDetails({ project }: { project: Project }) {
  const [copied, setCopied] = useState(false);

  const copyTrackingID = async () => {
    try {
      await navigator.clipboard.writeText(project.tracking_id);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <SectionCard title="Tracking">
      <div className="space-y-2 text-sm">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-zinc-500 dark:text-zinc-400">Tracking ID:</span>
          <code className="rounded-md bg-zinc-100 px-2 py-0.5 font-mono text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100">
            {project.tracking_id}
          </code>
          <button
            type="button"
            onClick={copyTrackingID}
            className="rounded-lg border border-zinc-300 px-2 py-1 text-xs font-medium hover:bg-zinc-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 dark:border-zinc-700 dark:hover:bg-zinc-800"
          >
            {copied ? "Copied!" : "Copy"}
          </button>
        </div>
        <p className="text-zinc-500 dark:text-zinc-400">
          Enabled: {project.enabled ? "yes" : "no"} · Timezone: {project.timezone} · Retention:{" "}
          {project.retention_days} days
        </p>
        <p className="text-zinc-500 dark:text-zinc-400">
          Domains: {project.allowed_domains.join(", ") || "All domains"}
        </p>
      </div>
    </SectionCard>
  );
}

function GeneralSettings({
  project,
  canEdit,
  onSaved,
}: {
  project: Project;
  canEdit: boolean;
  onSaved: () => void;
}) {
  const [name, setName] = useState(project.name);
  const [domains, setDomains] = useState(project.allowed_domains.join(", "));
  const [timezone, setTimezone] = useState(project.timezone);
  const [retentionDays, setRetentionDays] = useState(project.retention_days);
  const [enabled, setEnabled] = useState(project.enabled);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const dirty =
    name !== project.name ||
    domains !== project.allowed_domains.join(", ") ||
    timezone !== project.timezone ||
    retentionDays !== project.retention_days ||
    enabled !== project.enabled;

  const reset = () => {
    setName(project.name);
    setDomains(project.allowed_domains.join(", "));
    setTimezone(project.timezone);
    setRetentionDays(project.retention_days);
    setEnabled(project.enabled);
    setFormError(null);
    setSaved(false);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!canEdit || saving || !name.trim()) return;
    setSaving(true);
    setFormError(null);
    setSaved(false);
    try {
      await updateProject(project.id, {
        name: name.trim(),
        allowed_domains: domains
          .split(",")
          .map((d) => d.trim().toLowerCase())
          .filter(Boolean),
        timezone: timezone.trim() || project.timezone,
        retention_days: retentionDays,
        enabled,
      });
      setSaved(true);
      onSaved();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to save settings");
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard title="General">
      <form onSubmit={submit} className="space-y-4">
        <label className="block">
          <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
            Name
          </span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={255}
            disabled={!canEdit || saving}
            className={inputClass}
          />
        </label>

        <label className="block">
          <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
            Allowed domains <span className="normal-case font-normal">(comma-separated)</span>
          </span>
          <input
            value={domains}
            onChange={(e) => setDomains(e.target.value)}
            placeholder="example.com, www.example.com"
            disabled={!canEdit || saving}
            className={`${inputClass} font-mono`}
          />
        </label>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <label className="block">
            <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
              Timezone
            </span>
            <input
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              disabled={!canEdit || saving}
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
              disabled={!canEdit || saving}
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

        <label className="flex items-center gap-2 text-sm text-zinc-700 dark:text-zinc-300">
          <input
            type="checkbox"
            checked={enabled}
            disabled={!canEdit || saving}
            onChange={(e) => setEnabled(e.target.checked)}
            className="h-4 w-4 rounded border-zinc-300 accent-indigo-600 dark:border-zinc-700"
          />
          Accept events for this project
        </label>
        {!enabled && (
          <p className="text-xs text-zinc-500">
            Disabled: the tracking script is rejected until this is re-enabled.
          </p>
        )}

        {formError && (
          <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-300">
            {formError}
          </p>
        )}
        {saved && !dirty && (
          <p role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-800/60 dark:bg-emerald-950/40 dark:text-emerald-300">
            Settings saved.
          </p>
        )}
        {!canEdit && (
          <p className="text-xs text-zinc-500">
            Viewers have read-only access. Ask an owner or admin to make changes.
          </p>
        )}

        <div className="flex gap-2">
          <button
            type="submit"
            disabled={!canEdit || saving || !dirty || !name.trim()}
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 disabled:opacity-50 dark:bg-indigo-500 dark:hover:bg-indigo-400"
          >
            {saving ? "Saving..." : "Save changes"}
          </button>
          <button type="button" onClick={reset} disabled={!dirty || saving} className="rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 disabled:opacity-50 dark:border-zinc-700 dark:hover:bg-zinc-800">
            Reset
          </button>
        </div>
      </form>
    </SectionCard>
  );
}

function DangerZone({
  project,
  canEdit,
  onDeleted,
}: {
  project: Project;
  canEdit: boolean;
  onDeleted: () => void;
}) {
  const [confirmName, setConfirmName] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const matches = confirmName === project.name;

  const remove = async () => {
    if (!matches || !canEdit || deleting) return;
    setDeleting(true);
    setDeleteError(null);
    try {
      await deleteProject(project.id);
      onDeleted();
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : "Failed to delete project");
    } finally {
      setDeleting(false);
      setConfirmName("");
    }
  };

  return (
    <SectionCard title="Danger zone" danger>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Deleting this project permanently removes its visitors, sessions, and page views. The
        tracking ID stops working immediately. This cannot be undone.
      </p>
      <label className="block">
        <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
          Type <span className="normal-case font-mono">{project.name}</span> to confirm
        </span>
        <input
          value={confirmName}
          onChange={(e) => setConfirmName(e.target.value)}
          placeholder={project.name}
          disabled={!canEdit || deleting}
          className={inputClass}
        />
      </label>
      {deleteError && (
        <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-300">
          {deleteError}
        </p>
      )}
      <button
        type="button"
        onClick={remove}
        disabled={!canEdit || deleting || !matches}
        className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-red-500 disabled:opacity-50"
      >
        {deleting ? "Deleting..." : "Delete project"}
      </button>
      {!canEdit && <p className="text-xs text-zinc-500">Viewers cannot delete projects.</p>}
    </SectionCard>
  );
}
