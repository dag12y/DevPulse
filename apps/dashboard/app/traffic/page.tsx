"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getTraffic, type TrafficData } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";
import TrafficChart from "@/components/TrafficChart";

export default function TrafficPage() {
  const { selectedTrackingId, range, days } = useProject();
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

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Traffic" />

      <TrafficChart data={traffic} days={days} tall />

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
