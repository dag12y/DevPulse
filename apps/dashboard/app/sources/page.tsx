"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getSources, type TrafficSource } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function SourcesPage() {
  const { selectedTrackingId, days } = useProject();
  const [sources, setSources] = useState<TrafficSource[]>([]);
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
    getSources(selectedTrackingId, days)
      .then(setSources)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, days, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Traffic Sources" />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Traffic Sources" variant="table" />;
  }

  if (error) {
    return <ErrorState title="Traffic Sources" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Traffic Sources" />

      <ReportTable
        rows={sources}
        rowKey={(source) => source.source}
        csvFilename="traffic-sources"
        searchPlaceholder="Search sources…"
        emptyMessage="No sources yet for this project and range."
        defaultSortKey="page_views"
        columns={[
          { key: "source", label: "Source", value: (source) => source.source },
          { key: "category", label: "Category", value: (source) => source.category },
          { key: "page_views", label: "Page Views", numeric: true, value: (source) => source.page_views },
          { key: "visitors", label: "Visitors", numeric: true, value: (source) => source.visitors },
          {
            key: "percentage",
            label: "Share",
            numeric: true,
            value: (source) => source.percentage,
            render: (source) => `${source.percentage.toFixed(1)}%`,
          },
        ]}
      />
    </div>
  );
}
