"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getTopPages, type TopPage } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function PagesPage() {
  const { selectedTrackingId, days } = useProject();
  const [pages, setPages] = useState<TopPage[]>([]);
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
    getTopPages(50, selectedTrackingId, days)
      .then(setPages)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, days, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Top Pages" />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Top Pages" variant="table" />;
  }

  if (error) {
    return <ErrorState title="Top Pages" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Top Pages" />

      <ReportTable
        rows={pages}
        rowKey={(page) => page.path}
        csvFilename="top-pages"
        searchPlaceholder="Search paths…"
        emptyMessage="No page views yet. Install the tracker, visit the site, then check back."
        defaultSortKey="views"
        columns={[
          { key: "path", label: "Path", value: (page) => page.path },
          { key: "views", label: "Views", numeric: true, value: (page) => page.views },
          { key: "unique_visitors", label: "Unique Visitors", numeric: true, value: (page) => page.unique_visitors },
        ]}
      />
    </div>
  );
}
