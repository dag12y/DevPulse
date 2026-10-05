"use client";

import { useState } from "react";
import { MAX_RANGE_DAYS, rangeSpanDays } from "@/lib/api";
import { DATE_RANGES, isValidCustomRange, useProject } from "@/lib/project-context";

function localISO(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

const inputClass =
  "rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-2 py-1 text-sm text-zinc-900 dark:text-zinc-100";

export default function DateRangeSelector() {
  const { presetDays, customRange, setDays, setCustomRange } = useProject();
  const [editing, setEditing] = useState(false);
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");

  const candidate = { start, end };
  const span = isValidCustomRange(candidate) ? rangeSpanDays(start, end) : null;

  const openEditor = () => {
    const today = new Date();
    const defaultStart = new Date(today);
    defaultStart.setDate(today.getDate() - 29);
    setStart(customRange?.start ?? localISO(defaultStart));
    setEnd(customRange?.end ?? localISO(today));
    setEditing(true);
  };

  const apply = () => {
    if (span === null) return;
    setCustomRange(start, end);
    setEditing(false);
  };

  return (
    <div className="flex flex-col items-end gap-2">
      <label className="inline-flex items-center gap-2">
        <span className="text-xs font-medium uppercase tracking-wide text-zinc-500 dark:text-zinc-400">
          Range
        </span>
        <select
          aria-label="Select date range"
          value={editing || customRange ? "custom" : String(presetDays)}
          onChange={(e) => {
            const value = e.target.value;
            if (value === "custom") {
              openEditor();
              return;
            }
            setEditing(false);
            setDays(Number(value));
          }}
          className="rounded-md border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-1.5 text-sm text-zinc-900 dark:text-zinc-100"
        >
          {DATE_RANGES.map((range) => (
            <option key={range.days} value={range.days}>
              {range.label}
            </option>
          ))}
          <option value="custom">Custom…</option>
        </select>
      </label>

      {customRange && !editing && (
        <button
          type="button"
          onClick={openEditor}
          className="text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-600"
        >
          {customRange.start} → {customRange.end} · edit
        </button>
      )}

      {editing && (
        <div
          role="group"
          aria-label="Custom date range"
          className="flex flex-wrap items-end gap-2 rounded-md border border-zinc-200 bg-white p-2 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
        >
          <label className="block">
            <span className="block text-xs text-zinc-500">Start</span>
            <input
              type="date"
              value={start}
              onChange={(e) => setStart(e.target.value)}
              aria-label="Start date"
              className={inputClass}
            />
          </label>
          <label className="block">
            <span className="block text-xs text-zinc-500">End</span>
            <input
              type="date"
              value={end}
              onChange={(e) => setEnd(e.target.value)}
              aria-label="End date"
              className={inputClass}
            />
          </label>
          <button
            type="button"
            onClick={apply}
            disabled={span === null}
            className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900"
          >
            Apply
          </button>
          <button
            type="button"
            onClick={() => setEditing(false)}
            className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm dark:border-zinc-700"
          >
            Cancel
          </button>
          <span role="status" className="w-full text-xs text-zinc-500 sm:w-auto">
            {span !== null
              ? `${span} day${span === 1 ? "" : "s"} (max ${MAX_RANGE_DAYS})`
              : start && end
                ? `Start must be on or before end · max ${MAX_RANGE_DAYS} days`
                : "Pick a start and end date"}
          </span>
        </div>
      )}
    </div>
  );
}
