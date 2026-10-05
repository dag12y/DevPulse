"use client";

import { useState, type FormEvent, type ReactNode } from "react";
import { deleteProject, updateProject, type Project } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";

const RETENTION_OPTIONS = [30, 90, 180, 365];

const inputClass =
  "w-full rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm text-zinc-900 dark:text-zinc-100 disabled:opacity-60";

const primaryButton =
  "rounded-md bg-zinc-900 dark:bg-zinc-100 px-4 py-2 text-sm font-medium text-white dark:text-zinc-900 disabled:opacity-50";

const secondaryButton =
  "rounded-md border border-zinc-300 dark:border-zinc-700 px-3 py-1.5 text-sm text-zinc-700 dark:text-zinc-300 disabled:opacity-50";

function parseDomains(value: string): string[] {
  return value
    .split(",")
    .map((domain) => domain.trim().toLowerCase())
    .filter(Boolean);
}

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
    <section
      className={`rounded-lg border p-4 space-y-4 max-w-xl ${
        danger ? "border-red-200 dark:border-red-900" : "border-zinc-200 dark:border-zinc-800"
      }`}
    >
      <h3
        className={`font-medium ${
          danger ? "text-red-700 dark:text-red-300" : "text-zinc-900 dark:text-zinc-100"
        }`}
      >
        {title}
      </h3>
      {children}
    </section>
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
          <code className="rounded bg-zinc-100 px-2 py-0.5 font-mono text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100">
            {project.tracking_id}
          </code>
          <button type="button" onClick={copyTrackingID} className={secondaryButton}>
            {copied ? "Copied!" : "Copy"}
          </button>
        </div>
        <p className="text-zinc-500 dark:text-zinc-400">
          Created {new Date(project.created_at).toLocaleString()} · Updated{" "}
          {new Date(project.updated_at).toLocaleString()}
        </p>
        <p className="text-zinc-500 dark:text-zinc-400">
          Status: {project.enabled ? "Active — accepting events" : "Disabled — events are rejected"}
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

  const nextDomains = parseDomains(domains);
  const dirty =
    name.trim() !== project.name ||
    nextDomains.join(",") !== project.allowed_domains.join(",") ||
    timezone.trim() !== project.timezone ||
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
    setFormError(null);
    setSaved(false);
    setSaving(true);
    try {
      await updateProject(project.id, {
        name: name.trim(),
        allowed_domains: nextDomains,
        timezone: timezone.trim(),
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

  const fieldDisabled = !canEdit || saving;
  const labelClass = "mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500";

  return (
    <SectionCard title="General settings">
      <form onSubmit={submit} className="space-y-4">
        <label className="block">
          <span className={labelClass}>Name</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={255}
            disabled={fieldDisabled}
            className={inputClass}
          />
        </label>

        <label className="block">
          <span className={labelClass}>
            Allowed domains{" "}
            <span className="normal-case font-normal">(comma-separated, empty = any domain)</span>
          </span>
          <input
            value={domains}
            onChange={(e) => setDomains(e.target.value)}
            placeholder="example.com, www.example.com"
            disabled={fieldDisabled}
            className={`${inputClass} font-mono`}
          />
          <span className="mt-1 block text-xs text-zinc-500">
            Events from other domains are rejected.
          </span>
        </label>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <label className="block">
            <span className={labelClass}>Timezone</span>
            <input
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              placeholder="Africa/Addis_Ababa"
              disabled={fieldDisabled}
              className={`${inputClass} font-mono`}
            />
            <span className="mt-1 block text-xs text-zinc-500">
              IANA name used to group days in reports.
            </span>
          </label>

          <label className="block">
            <span className={labelClass}>Retention</span>
            <select
              value={retentionDays}
              onChange={(e) => setRetentionDays(Number(e.target.value))}
              disabled={fieldDisabled}
              className={inputClass}
            >
              {RETENTION_OPTIONS.map((days) => (
                <option key={days} value={days}>
                  {days} days
                </option>
              ))}
            </select>
            <span className="mt-1 block text-xs text-zinc-500">
              Older analytics data is deleted automatically.
            </span>
          </label>
        </div>

        <label className="flex items-center gap-2 text-sm text-zinc-700 dark:text-zinc-300">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            disabled={fieldDisabled}
            className="h-4 w-4 rounded border-zinc-300 dark:border-zinc-700"
          />
          Accept events for this project
        </label>
        {!enabled && (
          <p className="text-xs text-zinc-500">
            Disabled: the tracking script is rejected until this is re-enabled.
          </p>
        )}

        {formError && (
          <p role="alert" className="text-sm text-red-600 dark:text-red-400">
            {formError}
          </p>
        )}
        {saved && !dirty && (
          <p role="status" className="text-sm text-green-600 dark:text-green-400">
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
            className={primaryButton}
          >
            {saving ? "Saving..." : "Save changes"}
          </button>
          <button type="button" onClick={reset} disabled={!dirty || saving} className={secondaryButton}>
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
        <p role="alert" className="text-sm text-red-600 dark:text-red-400">
          {deleteError}
        </p>
      )}
      <button
        type="button"
        onClick={remove}
        disabled={!canEdit || deleting || !matches}
        className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
      >
        {deleting ? "Deleting..." : "Delete project"}
      </button>
      {!canEdit && <p className="text-xs text-zinc-500">Viewers cannot delete projects.</p>}
    </SectionCard>
  );
}
