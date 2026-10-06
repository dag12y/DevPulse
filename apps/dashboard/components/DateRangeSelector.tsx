"use client";

import { useState } from "react";
import { MAX_RANGE_DAYS, rangeSpanDays } from "@/lib/api";
import { DATE_RANGES, isValidCustomRange, useProject } from "@/lib/project-context";

function localISO(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

const inputClass =
  "rounded-lg border border-zinc-300 bg-white px-2.5 py-1.5 text-sm text-zinc-900 shadow-xs outline-none transition placeholder:text-zinc-400 focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500";

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
          className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 shadow-xs outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
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
          className="text-xs font-medium text-zinc-500 underline-offset-2 hover:text-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 dark:text-zinc-400 dark:hover:text-zinc-200"
        >
          {customRange.start} → {customRange.end} · edit
        </button>
      )}

      {editing && (
        <div
          role="group"
          aria-label="Custom date range"
          className="flex flex-wrap items-end gap-2 rounded-2xl border border-zinc-200 bg-white p-3 shadow-sm shadow-zinc-950/5 dark:border-zinc-800 dark:bg-zinc-900 dark:shadow-none"
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
            className="rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-semibold text-white shadow-sm shadow-indigo-600/25 hover:bg-indigo-500 disabled:opacity-50 dark:bg-indigo-500 dark:hover:bg-indigo-400"
          >
            Apply
          </button>
          <button
            type="button"
            onClick={() => setEditing(false)}
            className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium dark:border-zinc-700"
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
