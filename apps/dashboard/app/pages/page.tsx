"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getLandingPages, getTopPages, type LandingPage, type TopPage } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function PagesPage() {
  const { selectedTrackingId, range } = useProject();
  const [pages, setPages] = useState<TopPage[]>([]);
  const [landing, setLanding] = useState<LandingPage[]>([]);
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
    Promise.all([
      getTopPages(50, selectedTrackingId, range),
      getLandingPages(50, selectedTrackingId, range),
    ])
      .then(([top, land]) => {
        setPages(top);
        setLanding(land);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, range, attempt]);

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

      <section aria-label="Most viewed pages">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">Most Viewed</h3>
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
      </section>

      <section aria-label="Session entry pages">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">Landing Pages</h3>
        <ReportTable
          rows={landing}
          rowKey={(page) => page.path}
          csvFilename="landing-pages"
          searchPlaceholder="Search landing pages…"
          emptyMessage="No sessions yet. Landing pages appear once visitors arrive."
          defaultSortKey="sessions"
          columns={[
            { key: "path", label: "Path", value: (page) => page.path },
            { key: "sessions", label: "Sessions", numeric: true, value: (page) => page.sessions },
            { key: "visitors", label: "Visitors", numeric: true, value: (page) => page.visitors },
            {
              key: "share",
              label: "Share",
              numeric: true,
              value: (page) => page.share,
              render: (page) => `${page.share.toFixed(1)}%`,
            },
          ]}
        />
      </section>
    </div>
  );
}
