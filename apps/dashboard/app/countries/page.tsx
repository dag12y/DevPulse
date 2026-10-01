"use client";

import { useEffect, useState } from "react";
import { getCountries, type CountryStats } from "@/lib/api";

export default function CountriesPage() {
  const [countries, setCountries] = useState<CountryStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getCountries()
      .then(setCountries)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-zinc-500">Loading countries...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load countries</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Countries</h2>

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
              <th className="pb-2 font-medium">Country</th>
              <th className="pb-2 font-medium text-right">Page Views</th>
              <th className="pb-2 font-medium text-right">Visitors</th>
              <th className="pb-2 font-medium text-right">Share</th>
            </tr>
          </thead>
          <tbody>
            {countries.map((c) => (
              <tr key={c.country} className="border-b border-zinc-100 dark:border-zinc-800/50">
                <td className="py-2 text-zinc-800 dark:text-zinc-200">{c.country}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{c.page_views.toLocaleString()}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{c.visitors.toLocaleString()}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{c.percentage.toFixed(1)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
