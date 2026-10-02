"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getDevices, type DeviceBreakdown, type DevicesStats } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";

function BreakdownTable({ title, entries }: { title: string; entries: DeviceBreakdown[] }) {
  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
      <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-4">{title}</h3>
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
            <th className="pb-2 font-medium">Name</th>
            <th className="pb-2 font-medium text-right">Page Views</th>
            <th className="pb-2 font-medium text-right">Visitors</th>
            <th className="pb-2 font-medium text-right">Share</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e) => (
            <tr key={e.name} className="border-b border-zinc-100 dark:border-zinc-800/50">
              <td className="py-2 text-zinc-800 dark:text-zinc-200">{e.name}</td>
              <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{e.page_views.toLocaleString()}</td>
              <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{e.visitors.toLocaleString()}</td>
              <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{e.percentage.toFixed(1)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
      {entries.length === 0 && (
        <p className="py-4 text-center text-sm text-zinc-500">No data yet.</p>
      )}
    </div>
  );
}

export default function DevicesPage() {
  const { selectedTrackingId } = useProject();
  const [devices, setDevices] = useState<DevicesStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedTrackingId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    getDevices(selectedTrackingId)
      .then(setDevices)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Devices" />
      </div>
    );
  }

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Devices" />
        <div className="flex items-center justify-center h-64">
          <p className="text-zinc-500">Loading devices...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Devices" />
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load devices</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Devices" />

      <BreakdownTable title="Device Type" entries={devices?.device_types ?? []} />
      <BreakdownTable title="Browser" entries={devices?.browsers ?? []} />
      <BreakdownTable title="Operating System" entries={devices?.operating_systems ?? []} />
    </div>
  );
}
