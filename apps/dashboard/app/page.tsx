"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getSummary, getTraffic, getTopPages, type AnalyticsSummary, type TrafficData, type TopPage } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function OverviewPage() {
  const { selectedTrackingId, days } = useProject();
  const [summary, setSummary] = useState<AnalyticsSummary | null>(null);
  const [traffic, setTraffic] = useState<TrafficData[]>([]);
  const [topPages, setTopPages] = useState<TopPage[]>([]);
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
    Promise.all([
      getSummary(selectedTrackingId, days),
      getTraffic(days, selectedTrackingId),
      getTopPages(10, selectedTrackingId, days),
    ])
      .then(([s, t, p]) => {
        setSummary(s);
        setTraffic(t);
        setTopPages(p);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, days, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Overview" />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Overview" variant="cards" />;
  }

  if (error) {
    return <ErrorState title="Overview" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  const maxViews = Math.max(...traffic.map((d) => d.page_views), 1);

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Overview" />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          label="Page Views"
          value={summary?.total_page_views ?? 0}
          change={summary?.page_views_change ?? null}
          invert={false}
        />
        <MetricCard
          label="Unique Visitors"
          value={summary?.unique_visitors ?? 0}
          change={summary?.visitors_change ?? null}
          invert={false}
        />
        <MetricCard
          label="Sessions"
          value={summary?.sessions ?? 0}
          change={summary?.sessions_change ?? null}
          invert={false}
        />
        <MetricCard
          label="Bounce Rate"
          value={`${((summary?.bounce_rate ?? 0) * 100).toFixed(1)}%`}
          change={null}
          sub={
            summary != null
              ? `${formatPoints(summary.bounce_rate_change)} pts vs prior ${summary.days}d`
              : undefined
          }
          invert={false}
        />
      </div>

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-3">New vs Returning Visitors</h3>
        {(() => {
          const fresh = summary?.new_visitors ?? 0;
          const returning = summary?.returning_visitors ?? 0;
          const total = fresh + returning;
          const freshShare = total > 0 ? (fresh / total) * 100 : 0;
          return (
            <div>
              <div
                className="flex h-3 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
                role="img"
                aria-label={`${fresh.toLocaleString()} new and ${returning.toLocaleString()} returning visitors`}
              >
                <div className="bg-green-500" style={{ width: `${freshShare}%` }} />
                <div className="bg-blue-500" style={{ width: `${100 - freshShare}%` }} />
              </div>
              <dl className="mt-3 grid grid-cols-2 gap-4 text-sm">
                <div>
                  <dt className="text-zinc-500 dark:text-zinc-400">New</dt>
                  <dd className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
                    {fresh.toLocaleString()}{" "}
                    <span className="text-xs font-normal text-zinc-500">({freshShare.toFixed(1)}%)</span>
                  </dd>
                </div>
                <div>
                  <dt className="text-zinc-500 dark:text-zinc-400">Returning</dt>
                  <dd className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
                    {returning.toLocaleString()}{" "}
                    <span className="text-xs font-normal text-zinc-500">({(100 - freshShare).toFixed(1)}%)</span>
                  </dd>
                </div>
              </dl>
            </div>
          );
        })()}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
          <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Traffic ({days} days)</h3>
          <div className="flex items-end gap-1 h-40" role="img" aria-label={`Page views per day over ${days} days`}>
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
          rows={topPages}
          rowKey={(page) => page.path}
          csvFilename="overview-top-pages"
          searchPlaceholder="Search paths…"
          emptyMessage="No page views yet. Install the tracker, visit the site, then check back."
          defaultSortKey="views"
          columns={[
            { key: "path", label: "Path", value: (page) => page.path },
            { key: "views", label: "Views", numeric: true, value: (page) => page.views },
            { key: "unique_visitors", label: "Visitors", numeric: true, value: (page) => page.unique_visitors },
          ]}
        />
      </div>
    </div>
  );
}

function MetricCard({
  label,
  value,
  change,
  sub,
  invert,
}: {
  label: string;
  value: number | string;
  change: number | null;
  sub?: string;
  invert: boolean;
}) {
  const positive = (change ?? 0) >= 0;
  const good = invert ? !positive : positive;
  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
      <p className="text-sm text-zinc-500 dark:text-zinc-400">{label}</p>
      <p className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100 mt-1">
        {typeof value === "number" ? value.toLocaleString() : value}
      </p>
      {change != null ? (
        <p className={`text-xs mt-1 ${good ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400"}`}>
          {change >= 0 ? "+" : ""}{change.toFixed(1)}% vs prior period
        </p>
      ) : (
        <p className="text-xs mt-1 text-zinc-400">— no prior baseline</p>
      )}
      {sub && <p className="text-xs mt-0.5 text-zinc-500">{sub}</p>}
    </div>
  );
}

function formatPoints(change: number): string {
  const sign = change >= 0 ? "+" : "";
  return `${sign}${(change).toFixed(1)}`;
}
