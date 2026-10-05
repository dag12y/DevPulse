"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getTraffic, type TrafficData } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function TrafficPage() {
  const { selectedTrackingId, range } = useProject();
  const [traffic, setTraffic] = useState<TrafficData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!selectedTrackingId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    getTraffic(range, selectedTrackingId)
      .then(setTraffic)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [range, selectedTrackingId, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic" />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Traffic" variant="chart" />;
  }

  if (error) {
    return <ErrorState title="Traffic" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  const maxViews = Math.max(...traffic.map((d) => d.page_views), 1);

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Traffic" />

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Page Views</h3>
        <div className="flex items-end gap-px h-48" role="img" aria-label={`Page views per day, ${traffic.length} days`}>
          {traffic.map((d) => {
            const tooltip = `${d.date}: ${d.page_views.toLocaleString()} views, ${d.visitors.toLocaleString()} visitors`;
            return (
              <div key={d.date} className="group relative flex-1 flex flex-col justify-end items-center gap-1 self-stretch">
                <div
                  tabIndex={0}
                  aria-label={tooltip}
                  title={tooltip}
                  className="w-full bg-blue-500 rounded-t focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-600"
                  style={{ height: `${(d.page_views / maxViews) * 100}%` }}
                />
                <span
                  role="tooltip"
                  className="pointer-events-none absolute -top-1 left-1/2 z-10 hidden -translate-x-1/2 -translate-y-full whitespace-nowrap rounded-md bg-zinc-900 px-2 py-1 text-xs text-white group-hover:block group-focus-within:block"
                >
                  {tooltip}
                </span>
              </div>
            );
          })}
        </div>
        <div className="flex justify-between mt-2 text-xs text-zinc-500">
          <span>{traffic[0]?.date}</span>
          <span>{traffic[traffic.length - 1]?.date}</span>
        </div>
      </div>

      <ReportTable
        rows={traffic}
        rowKey={(row) => row.date}
        csvFilename="traffic-daily"
        searchPlaceholder="Search dates…"
        emptyMessage="No traffic in this range yet."
        defaultSortKey="date"
        columns={[
          { key: "date", label: "Date", value: (row) => row.date },
          { key: "page_views", label: "Page Views", numeric: true, value: (row) => row.page_views },
          { key: "visitors", label: "Visitors", numeric: true, value: (row) => row.visitors },
        ]}
      />
    </div>
  );
}
