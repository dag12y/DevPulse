"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getRealtime, type RealtimeStats } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

const POLL_INTERVAL_MS = 15000;

export default function RealtimePage() {
  const { selectedTrackingId } = useProject();
  const [realtime, setRealtime] = useState<RealtimeStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!selectedTrackingId) {
      setLoading(false);
      return;
    }
    let cancelled = false;
    const load = () => {
      getRealtime(selectedTrackingId)
        .then((data) => {
          if (!cancelled) {
            setRealtime(data);
            setError(null);
          }
        })
        .catch((e) => {
          if (!cancelled) {
            setError(e.message);
          }
        })
        .finally(() => {
          if (!cancelled) {
            setLoading(false);
          }
        });
    };
    setLoading(true);
    load();
    const interval = setInterval(load, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [selectedTrackingId, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Real-time" showDateRange={false} />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Real-time" variant="table" />;
  }

  if (error && (realtime?.active_visitors ?? 0) === 0 && (realtime?.pages ?? []).length === 0) {
    return <ErrorState title="Real-time" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Real-time" showDateRange={false} />

      {error && (
        <div
          role="alert"
          className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200"
        >
          Live refresh paused: {error}{" "}
          <button type="button" onClick={() => setAttempt((count) => count + 1)} className="underline focus-visible:outline-2 focus-visible:outline-blue-600">
            Retry now
          </button>
        </div>
      )}

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <p className="text-sm text-zinc-500">Active visitors (last 5 minutes)</p>
        <p className="text-4xl font-semibold text-zinc-900 dark:text-zinc-100 mt-1" aria-live="polite">
          {realtime?.active_visitors ?? 0}
        </p>
      </div>

      <ReportTable
        rows={realtime?.pages ?? []}
        rowKey={(page) => page.path}
        csvFilename="realtime-pages"
        searchPlaceholder="Search current pages…"
        emptyMessage="Nobody online right now."
        defaultSortKey="visitors"
        columns={[
          { key: "path", label: "Path", value: (page) => page.path },
          { key: "visitors", label: "Visitors", numeric: true, value: (page) => page.visitors },
        ]}
      />
    </div>
  );
}
