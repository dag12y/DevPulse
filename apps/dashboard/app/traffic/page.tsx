"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getTraffic, type TrafficData } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";

export default function TrafficPage() {
  const { selectedTrackingId, days, setDays } = useProject();
  const [traffic, setTraffic] = useState<TrafficData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedTrackingId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    getTraffic(days, selectedTrackingId)
      .then(setTraffic)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [days, selectedTrackingId]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic" />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic" />
        <div className="flex items-center justify-center h-64">
          <p className="text-zinc-500">Loading traffic...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic" />
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load traffic</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  const maxViews = Math.max(...traffic.map((d) => d.page_views), 1);

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Traffic" />
      <div className="flex items-center justify-end">
        <label className="inline-flex items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
          Range
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            className="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-sm"
          >
            <option value={1}>Last 24 hours</option>
            <option value={7}>Last 7 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>Last 90 days</option>
          </select>
        </label>
      </div>

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Page Views</h3>
        <div className="flex items-end gap-px h-48">
          {traffic.map((d) => (
            <div key={d.date} className="flex-1 flex flex-col items-center gap-1">
              <div
                className="w-full bg-blue-500 rounded-t"
                style={{ height: `${(d.page_views / maxViews) * 100}%` }}
                title={`${d.date}: ${d.page_views} views, ${d.visitors} visitors`}
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
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Daily Breakdown</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
              <th className="pb-2 font-medium">Date</th>
              <th className="pb-2 font-medium text-right">Page Views</th>
              <th className="pb-2 font-medium text-right">Visitors</th>
            </tr>
          </thead>
          <tbody>
            {traffic.map((d) => (
              <tr key={d.date} className="border-b border-zinc-100 dark:border-zinc-800/50">
                <td className="py-2 text-zinc-800 dark:text-zinc-200">{d.date}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{d.page_views.toLocaleString()}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{d.visitors.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
