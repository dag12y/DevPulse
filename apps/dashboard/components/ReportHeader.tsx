"use client";

import Link from "next/link";
import { DATE_RANGES, useProject } from "@/lib/project-context";
import DateRangeSelector from "@/components/DateRangeSelector";

interface ReportHeaderProps {
  title: string;
  showDateRange?: boolean;
}

export default function ReportHeader({ title, showDateRange = true }: ReportHeaderProps) {
  const { selectedProject, selectedTrackingId, days, loading, error, projects } = useProject();

  const rangeLabel = DATE_RANGES.find((r) => r.days === days)?.label ?? `${days} days`;

  if (loading) {
    return (
      <div className="space-y-2">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">{title}</h2>
        <p className="text-sm text-zinc-500">Loading project context…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-3">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">{title}</h2>
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load projects</p>
          <p className="text-sm mt-1">{error}</p>
          <p className="text-sm mt-2">
            <Link className="underline" href="/login">Sign in</Link> or set{" "}
            <code>NEXT_PUBLIC_API_KEY</code> and reload.
          </p>
        </div>
      </div>
    );
  }

  if (!selectedProject || projects.length === 0) {
    return (
      <div className="space-y-3">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">{title}</h2>
        <div className="rounded-lg border border-dashed border-zinc-300 dark:border-zinc-700 p-6 text-sm text-zinc-600 dark:text-zinc-400">
          <p className="font-medium text-zinc-900 dark:text-zinc-100">No analytics project selected</p>
          <ol className="mt-2 list-decimal space-y-1 pl-5">
            <li>Create a project via the API or the Projects page.</li>
            <li>Select it in the sidebar project picker.</li>
            <li>
              Install the tracker — see <Link className="underline" href="/install">Install</Link>.
            </li>
          </ol>
          <p className="mt-3">
            <Link className="underline" href="/projects">Go to Projects</Link>
            {" · "}
            <Link className="underline" href="/install">Go to Install</Link>
          </p>
        </div>
      </div>
    );
  }

  const domains = selectedProject.allowed_domains.join(", ") || "All domains";

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">{title}</h2>
        {showDateRange && <DateRangeSelector />}
      </div>
      <p className="text-sm text-zinc-500 dark:text-zinc-400">
        {selectedProject.name} · <span className="font-mono">{selectedTrackingId}</span> · {domains} ·{" "}
        {selectedProject.timezone} · {rangeLabel}
      </p>
    </div>
  );
}
