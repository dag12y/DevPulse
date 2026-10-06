"use client";

import Link from "next/link";
import { DATE_RANGES, useProject } from "@/lib/project-context";
import DateRangeSelector from "@/components/DateRangeSelector";
import Badge from "@/components/ui/Badge";
import Card from "@/components/ui/Card";

interface ReportHeaderProps {
  title: string;
  showDateRange?: boolean;
}

export default function ReportHeader({ title, showDateRange = true }: ReportHeaderProps) {
  const { selectedProject, selectedTrackingId, days, presetDays, customRange, loading, error, projects } =
    useProject();

  const rangeLabel = customRange
    ? `${customRange.start} → ${customRange.end}`
    : DATE_RANGES.find((r) => r.days === presetDays)?.label ?? `${days} days`;

  if (loading) {
    return (
      <div className="space-y-2">
        <h2 className="text-xl font-bold tracking-tight text-zinc-900 sm:text-2xl dark:text-white">{title}</h2>
        <p className="text-sm text-zinc-500">Loading project context…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-3">
        <h2 className="text-xl font-bold tracking-tight text-zinc-900 sm:text-2xl dark:text-white">{title}</h2>
        <Card className="border-red-200 bg-red-50/60 p-4 text-red-800 dark:border-red-800/60 dark:bg-red-950/40 dark:text-red-200">
          <p className="font-semibold">Failed to load projects</p>
          <p className="text-sm mt-1">{error}</p>
          <p className="text-sm mt-2">
            <Link className="font-medium underline underline-offset-2" href="/login">Sign in</Link> or set{" "}
            <code>NEXT_PUBLIC_API_KEY</code> and reload.
          </p>
        </Card>
      </div>
    );
  }

  if (!selectedProject || projects.length === 0) {
    return (
      <div className="space-y-3">
        <h2 className="text-xl font-bold tracking-tight text-zinc-900 sm:text-2xl dark:text-white">{title}</h2>
        <Card className="border-dashed p-6 text-sm text-zinc-600 dark:text-zinc-400">
          <p className="font-semibold text-zinc-900 dark:text-white">No analytics project selected</p>
          <ol className="mt-2 list-decimal space-y-1 pl-5">
            <li>Create a project via the API or the Projects page.</li>
            <li>Select it in the sidebar project picker.</li>
            <li>
              Install the tracker — see <Link className="font-medium text-indigo-600 underline underline-offset-2 dark:text-indigo-400" href="/install">Install</Link>.
            </li>
          </ol>
          <p className="mt-3">
            <Link className="font-medium text-indigo-600 underline underline-offset-2 dark:text-indigo-400" href="/projects">Go to Projects</Link>
            {" · "}
            <Link className="font-medium text-indigo-600 underline underline-offset-2 dark:text-indigo-400" href="/install">Go to Install</Link>
          </p>
        </Card>
      </div>
    );
  }

  const domains = selectedProject.allowed_domains.join(", ") || "All domains";

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-bold tracking-tight text-zinc-900 sm:text-2xl dark:text-white">{title}</h2>
        {showDateRange && <DateRangeSelector />}
      </div>
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-zinc-500 dark:text-zinc-400">
        <span className="font-medium text-zinc-700 dark:text-zinc-300">{selectedProject.name}</span>
        <Badge tone="neutral">
          <span className="font-mono font-medium">{selectedTrackingId}</span>
        </Badge>
        <span>{domains}</span>
        <span aria-hidden="true" className="text-zinc-300 dark:text-zinc-700">·</span>
        <span>{selectedProject.timezone}</span>
        <span aria-hidden="true" className="text-zinc-300 dark:text-zinc-700">·</span>
        <span>{rangeLabel}</span>
      </p>
    </div>
  );
}
