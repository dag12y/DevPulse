"use client";
/* eslint-disable react-hooks/set-state-in-effect -- intentional refetch when project/range changes */

import { useEffect, useState } from "react";
import { getCountries, type CountryStats } from "@/lib/api";
import { useProject } from "@/lib/project-context";
import ReportHeader from "@/components/ReportHeader";
import { ErrorState, ReportLoading } from "@/components/ReportStates";
import ReportTable from "@/components/ReportTable";

export default function CountriesPage() {
  const { selectedTrackingId, range } = useProject();
  const [countries, setCountries] = useState<CountryStats[]>([]);
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
    getCountries(selectedTrackingId, range)
      .then(setCountries)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [selectedTrackingId, range, attempt]);

  if (!selectedTrackingId && !loading) {
    return (
      <div className="p-6 space-y-6">
        <ReportHeader title="Countries" />
      </div>
    );
  }

  if (loading) {
    return <ReportLoading title="Countries" variant="table" />;
  }

  if (error) {
    return <ErrorState title="Countries" message={error} onRetry={() => setAttempt((count) => count + 1)} />;
  }

  return (
    <div className="p-6 space-y-6">
      <ReportHeader title="Countries" />

      <ReportTable
        rows={countries}
        rowKey={(country) => country.country}
        csvFilename="countries"
        searchPlaceholder="Search countries…"
        emptyMessage="No country data yet. Geography requires GeoIP enrichment on the API."
        defaultSortKey="page_views"
        columns={[
          { key: "country", label: "Country", value: (country) => country.country },
          { key: "page_views", label: "Page Views", numeric: true, value: (country) => country.page_views },
          { key: "visitors", label: "Visitors", numeric: true, value: (country) => country.visitors },
          {
            key: "percentage",
            label: "Share",
            numeric: true,
            value: (country) => country.percentage,
            render: (country) => `${country.percentage.toFixed(1)}%`,
          },
        ]}
      />
    </div>
  );
}
