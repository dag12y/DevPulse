"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getDevices, type DeviceBreakdown, type DevicesStats } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

function BreakdownTable({ title, entries, csvFilename }: { title: string; entries: DeviceBreakdown[]; csvFilename: string }) {
  return (
    <section aria-label={title}>
      <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">{title}</h3>
      <ReportTable
        rows={entries}
        rowKey={(entry) => entry.name}
        csvFilename={csvFilename}
        searchPlaceholder={`Search ${title.toLowerCase()}…`}
        emptyMessage="No data yet."
        defaultSortKey="page_views"
        columns={[
          { key: "name", label: "Name", value: (entry) => entry.name },
          { key: "page_views", label: "Page Views", numeric: true, value: (entry) => entry.page_views },
          { key: "visitors", label: "Visitors", numeric: true, value: (entry) => entry.visitors },
          {
            key: "percentage",
            label: "Share",
            numeric: true,
            value: (entry) => entry.percentage,
            render: (entry) => `${entry.percentage.toFixed(1)}%`,
          },
        ]}
      />
    </section>
  );
}

export default function DevicesPage() {
  const { selectedTrackingId, days } = useProject();
  const [devices, setDevices] = useState<DevicesStats | null>(null);
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
    getDevices(selectedTrackingId, days)
      .then(setDevices)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, days, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Devices" />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Devices" variant="table" />;
  }

  if (error) {
    return <ErrorState title="Devices" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Devices" />

      <BreakdownTable title="Device Type" entries={devices?.device_types ?? []} csvFilename="device-types" />
      <BreakdownTable title="Browser" entries={devices?.browsers ?? []} csvFilename="browsers" />
      <BreakdownTable title="Operating System" entries={devices?.operating_systems ?? []} csvFilename="operating-systems" />
      <BreakdownTable title="Screen Resolution" entries={devices?.screens ?? []} csvFilename="screens" />
      <BreakdownTable title="Viewport Size" entries={devices?.viewports ?? []} csvFilename="viewports" />
    </div>
  );
}
