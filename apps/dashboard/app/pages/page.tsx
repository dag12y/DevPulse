"use client";

import { useEffect, useState } from "react";
import { getTopPages, type TopPage } from "@/lib/api";

export default function PagesPage() {
  const [pages, setPages] = useState<TopPage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getTopPages(50)
      .then(setPages)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-zinc-500">Loading pages...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
          <p className="font-medium">Failed to load pages</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <h2 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-100">Top Pages</h2>

      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-200 dark:border-zinc-800">
              <th className="pb-2 font-medium">Path</th>
              <th className="pb-2 font-medium text-right">Views</th>
              <th className="pb-2 font-medium text-right">Unique Visitors</th>
            </tr>
          </thead>
          <tbody>
            {pages.map((p) => (
              <tr key={p.path} className="border-b border-zinc-100 dark:border-zinc-800/50">
                <td className="py-2 text-zinc-800 dark:text-zinc-200">{p.path}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{p.views.toLocaleString()}</td>
                <td className="py-2 text-right text-zinc-600 dark:text-zinc-400">{p.unique_visitors.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
