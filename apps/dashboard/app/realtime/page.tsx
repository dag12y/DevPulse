"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getRealtime, type RealtimeStats } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";

const POLL_INTERVAL_MS = 15000;

export default function RealtimePage() {
  const { selectedTrackingId } = useProject();
  const [realtime, setRealtime] = useState<RealtimeStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
  }, [selectedTrackingId]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Real-time" showDateRange={false} />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Real-time" showDateRange={false} />
        <div className="flex items-center justify-center h-64">
          <p className="text-zinc-500">Loading real-time...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Real-time" showDateRange={false} />
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load real-time</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Real-time" showDateRange={false} />

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <p className="text-sm text-zinc-500">Active visitors (last 5 minutes)</p>
        <p className="text-4xl font-semibold text-zinc-900 dark:text-zinc-100 mt-1">
          {realtime?.active_visitors ?? 0}
        </p>
      </div>

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">Current Pages</h3>
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
              <th className="pb-2 font-medium">Path</th>
              <th className="pb-2 font-medium text-right">Visitors</th>
            </tr>
          </thead>
          <tbody>
            {(realtime?.pages ?? []).map((p) => (
              <tr key={p.path} className="border-b border-zinc-100 dark:border-zinc-800/50">
                <td className="py-2 text-zinc-800 dark:text-zinc-200">{p.path}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{p.visitors.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(realtime?.pages ?? []).length === 0 && (
          <p className="py-4 text-center text-sm text-zinc-500">Nobody online right now.</p>
        )}
      </div>
    </div>
  );
}
