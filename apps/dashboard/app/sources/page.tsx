"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getSources, type TrafficSource } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";

export default function SourcesPage() {
  const { selectedTrackingId } = useProject();
  const [sources, setSources] = useState<TrafficSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedTrackingId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    getSources(selectedTrackingId)
      .then(setSources)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic Sources" />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic Sources" />
        <div className="flex items-center justify-center h-64">
          <p className="text-zinc-500">Loading sources...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic Sources" />
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load sources</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Traffic Sources" />

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
              <th className="pb-2 font-medium">Source</th>
              <th className="pb-2 font-medium">Category</th>
              <th className="pb-2 font-medium text-right">Page Views</th>
              <th className="pb-2 font-medium text-right">Visitors</th>
              <th className="pb-2 font-medium text-right">Share</th>
            </tr>
          </thead>
          <tbody>
            {sources.map((s) => (
              <tr key={s.source} className="border-b border-zinc-100 dark:border-zinc-800/50">
                <td className="py-2 text-zinc-800 dark:text-zinc-200">{s.source}</td>
                <td className="py-2 text-zinc-600 dark:text-zinc-400">{s.category}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{s.page_views.toLocaleString()}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{s.visitors.toLocaleString()}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{s.percentage.toFixed(1)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
        {sources.length === 0 && (
          <p className="py-6 text-center text-sm text-zinc-500">No sources yet for this project and range.</p>
        )}
      </div>
    </div>
  );
}
