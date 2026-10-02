"use client";

import { DATE_RANGES, useProject } from "@/lib/project-context";

export default function DateRangeSelector() {
  const { days, setDays } = useProject();

  return (
    <label className="inline-flex items-center gap-2">
      <span className="text-xs font-medium uppercase tracking-wide text-zinc-500 dark:text-zinc-400">
        Range
      </span>
      <select
        aria-label="Select date range"
        value={days}
        onChange={(e) => setDays(Number(e.target.value))}
        className="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-sm text-zinc-900 dark:text-zinc-100"
      >
        {DATE_RANGES.map((range) => (
          <option key={range.days} value={range.days}>
            {range.label}
          </option>
        ))}
      </select>
    </label>
  );
}
