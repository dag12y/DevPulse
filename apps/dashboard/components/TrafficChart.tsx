"use client";

import { useState } from "react";
import type { TrafficData } from "@/lib/api";

type Metric = "views" | "visitors";

const METRICS: { key: Metric; label: string; color: string }[] = [
  { key: "views", label: "Views", color: "bg-blue-500" },
  { key: "visitors", label: "Visitors", color: "bg-emerald-500" },
];

function compact(value: number): string {
  if (value >= 1000) {
    const short = value / 1000;
    return `${short >= 100 ? Math.round(short) : short.toFixed(1).replace(/\.0$/, "")}k`;
  }
  return String(value);
}

// niceCeil rounds the axis ceiling up to a 1/2/2.5/5/10 step so gridline
// labels stay round regardless of the data peak.
function niceCeil(value: number): number {
  if (value <= 4) return 4;
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const scaled = value / magnitude;
  const step = scaled <= 1 ? 1 : scaled <= 2 ? 2 : scaled <= 2.5 ? 2.5 : scaled <= 5 ? 5 : 10;
  return step * magnitude;
}

// sampleTicks picks up to maxTicks evenly spread indexes so x-axis labels
// stay readable from 1 to 365 days.
function sampleTicks(length: number, maxTicks: number): number[] {
  if (length === 0) return [];
  if (length === 1) return [0];
  const count = Math.min(maxTicks, length);
  const ticks: number[] = [];
  for (let i = 0; i < count; i += 1) {
    ticks.push(Math.round((i * (length - 1)) / (count - 1)));
  }
  return [...new Set(ticks)];
}

export default function TrafficChart({
  data,
  days,
  tall = false,
}: {
  data: TrafficData[];
  days: number;
  tall?: boolean;
}) {
  const [metric, setMetric] = useState<Metric>("views");
  const valueOf =
    metric === "views"
      ? (point: TrafficData) => point.page_views
      : (point: TrafficData) => point.visitors;

  if (data.length === 0) {
    return (
      <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
          Traffic <span className="font-normal text-zinc-500">({days} days)</span>
        </h3>
        <p className="mt-4 text-sm text-zinc-500">No traffic in this range yet.</p>
      </div>
    );
  }

  const peak = Math.max(...data.map(valueOf), 0);
  const ceiling = niceCeil(peak);
  const active = METRICS.find((option) => option.key === metric) ?? METRICS[0];
  const ticks = sampleTicks(data.length, 6);
  const plotHeight = tall ? "h-56" : "h-40";

  return (
    <figure className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
          Traffic <span className="font-normal text-zinc-500">({days} days)</span>
        </h3>
        <div
          role="group"
          aria-label="Chart metric"
          className="flex gap-1 rounded-md border border-zinc-300 p-0.5 dark:border-zinc-700"
        >
          {METRICS.map((option) => {
            const selected = metric === option.key;
            return (
              <button
                key={option.key}
                type="button"
                onClick={() => setMetric(option.key)}
                aria-pressed={selected}
                className={`flex items-center gap-1.5 rounded px-2 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-600 ${
                  selected
                    ? "bg-zinc-200 text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100"
                    : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                }`}
              >
                <span aria-hidden="true" className={`h-2 w-2 rounded-full ${option.color}`} />
                {option.label}
              </button>
            );
          })}
        </div>
      </div>

      <div className="mt-4 overflow-x-auto pb-1">
        <div style={{ minWidth: Math.max(480, data.length * 5) }}>
          <div className="flex gap-3">
            <div aria-hidden="true" className={`relative w-10 shrink-0 ${plotHeight}`}>
              {[1, 0.75, 0.5, 0.25].map((fraction) => (
                <span
                  key={fraction}
                  style={{ bottom: `${fraction * 100}%` }}
                  className="absolute right-1 -translate-y-1/2 text-[11px] tabular-nums text-zinc-400 dark:text-zinc-500"
                >
                  {compact(ceiling * fraction)}
                </span>
              ))}
              <span className="absolute bottom-0 right-1 translate-y-1/2 text-[11px] tabular-nums text-zinc-400 dark:text-zinc-500">
                0
              </span>
            </div>

            <div
              role="img"
              aria-label={`${metric} per day over ${days} days, peak ${peak.toLocaleString()}`}
              className={`relative flex-1 ${plotHeight}`}
            >
              <div aria-hidden="true" className="absolute inset-0">
                {[0.25, 0.5, 0.75, 1].map((fraction) => (
                  <div
                    key={fraction}
                    style={{ bottom: `${fraction * 100}%` }}
                    className="absolute inset-x-0 border-t border-zinc-100 dark:border-zinc-800/70"
                  />
                ))}
                <div className="absolute inset-x-0 bottom-0 border-t border-zinc-200 dark:border-zinc-700" />
              </div>
              <div className="relative flex h-full items-end gap-1">
                {data.map((point) => {
                  const value = valueOf(point);
                  const tooltip = `${point.date}: ${point.page_views.toLocaleString()} views, ${point.visitors.toLocaleString()} visitors`;
                  return (
                    <div
                      key={point.date}
                      className="group relative flex min-w-px flex-1 flex-col justify-end self-stretch"
                    >
                      <div
                        tabIndex={0}
                        aria-label={tooltip}
                        title={tooltip}
                        style={{ height: value > 0 ? `${Math.max((value / ceiling) * 100, 2)}%` : "0%" }}
                        className={`w-full rounded-t focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-600 ${active.color}`}
                      />
                      <span
                        role="tooltip"
                        className="pointer-events-none absolute -top-1 left-1/2 z-10 hidden -translate-x-1/2 -translate-y-full whitespace-nowrap rounded-md bg-zinc-900 px-2 py-1 text-xs text-white group-hover:block group-focus-within:block"
                      >
                        {tooltip}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>

          <div aria-hidden="true" className="mt-2 flex justify-between gap-2 pl-[3.25rem] text-[11px] text-zinc-500">
            {ticks.map((index) => (
              <span key={data[index].date} className="truncate">
                {data[index].date}
              </span>
            ))}
          </div>
        </div>
      </div>

      <figcaption className="mt-2 flex items-center gap-4 text-xs text-zinc-500">
        {METRICS.map((option) => (
          <span key={option.key} className="inline-flex items-center gap-1.5">
            <span aria-hidden="true" className={`h-2 w-2 rounded-full ${option.color}`} />
            {option.label}
          </span>
        ))}
      </figcaption>
    </figure>
  );
}
