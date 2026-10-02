"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getSummary, getTraffic, getTopPages, type AnalyticsSummary, type TrafficData, type TopPage } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";

export default function OverviewPage() {
  const { selectedTrackingId, days } = useProject();
  const [summary, setSummary] = useState<AnalyticsSummary | null>(null);
  const [traffic, setTraffic] = useState<TrafficData[]>([]);
  const [topPages, setTopPages] = useState<TopPage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedTrackingId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    Promise.all([
      getSummary(selectedTrackingId),
      getTraffic(days, selectedTrackingId),
      getTopPages(10, selectedTrackingId),
    ])
      .then(([s, t, p]) => {
        setSummary(s);
        setTraffic(t);
        setTopPages(p);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, days]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Overview" />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Overview" />
        <div className="flex items-center justify-center h-64">
          <p className="text-zinc-500">Loading analytics...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Overview" />
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load analytics</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  const maxViews = Math.max(...traffic.map((d) => d.page_views), 1);

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Overview" />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard label="Page Views" value={summary?.total_page_views ?? 0} />
        <MetricCard label="Unique Visitors" value={summary?.unique_visitors ?? 0} />
        <MetricCard label="Sessions" value={summary?.sessions ?? 0} />
        <MetricCard label="Bounce Rate" value={`${((summary?.bounce_rate ?? 0) * 100).toFixed(1)}%`} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
          <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Traffic ({days} days)</h3>
          <div className="flex items-end gap-1 h-40">
            {traffic.map((d) => (
              <div key={d.date} className="flex-1 flex flex-col items-center gap-1">
                <div
                  className="w-full bg-blue-500 rounded-t"
                  style={{ height: `${(d.page_views / maxViews) * 100}%` }}
                  title={`${d.date}: ${d.page_views} views`}
                />
              </div>
            ))}
          </div>
          <div className="flex justify-between mt-2 text-xs text-zinc-500">
            <span>{traffic[0]?.date}</span>
            <span>{traffic[traffic.length - 1]?.date}</span>
          </div>
        </div>

        <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
          <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Top Pages</h3>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
                <th className="pb-2 font-medium">Path</th>
                <th className="pb-2 font-medium text-right">Views</th>
                <th className="pb-2 font-medium text-right">Visitors</th>
              </tr>
            </thead>
            <tbody>
              {topPages.map((p) => (
                <tr key={p.path} className="border-b border-zinc-100 dark:border-zinc-800/50">
                  <td className="py-2 text-zinc-800 dark:text-zinc-200 truncate max-w-48">{p.path}</td>
                  <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{p.views}</td>
                  <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{p.unique_visitors}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
      <p className="text-sm text-zinc-500 dark:text-zinc-400">{label}</p>
      <p className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100 mt-1">
        {typeof value === "number" ? value.toLocaleString() : value}
      </p>
    </div>
  );
}
