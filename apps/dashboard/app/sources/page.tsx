"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getSources, getUTM, type TrafficSource, type UTMBreakdown, type UTMReport } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function SourcesPage() {
  const { selectedTrackingId, days } = useProject();
  const [sources, setSources] = useState<TrafficSource[]>([]);
  const [utm, setUTM] = useState<UTMReport | null>(null);
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
    Promise.all([getSources(selectedTrackingId, days), getUTM(selectedTrackingId, days)])
      .then(([classified, campaigns]) => {
        setSources(classified);
        setUTM(campaigns);
      })
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

      <UTMTable
        title="UTM Sources"
        entries={utm?.sources ?? []}
        csvFilename="utm-sources"
        emptyMessage="No tagged traffic yet. Add ?utm_source=… to links you share."
      />
      <UTMTable
        title="UTM Mediums"
        entries={utm?.mediums ?? []}
        csvFilename="utm-mediums"
        emptyMessage="No tagged traffic yet."
      />
      <UTMTable
        title="UTM Campaigns"
        entries={utm?.campaigns ?? []}
        csvFilename="utm-campaigns"
        emptyMessage="No tagged traffic yet. Add ?utm_campaign=… to links you share."
      />
    </div>
  );
}

function UTMTable({
  title,
  entries,
  csvFilename,
  emptyMessage,
}: {
  title: string;
  entries: UTMBreakdown[];
  csvFilename: string;
  emptyMessage: string;
}) {
  return (
    <section aria-label={title}>
      <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">{title}</h3>
      <ReportTable
        rows={entries}
        rowKey={(entry) => entry.name}
        csvFilename={csvFilename}
        searchPlaceholder={`Search ${title.toLowerCase()}…`}
        emptyMessage={emptyMessage}
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
