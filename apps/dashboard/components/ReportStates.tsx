"use client";

import ReportHeader from "@/components/ReportHeader";

function SkeletonBlock({ className }: { className: string }) {
  return <div aria-hidden="true" className={`animate-pulse rounded-md bg-zinc-200 dark:bg-zinc-800 ${className}`} />;
}

export function ReportLoading({ title, variant = "table" }: { title: string; variant?: "cards" | "chart" | "table" }) {
  return (
    <div className="p-6 space-y-6">
      <ReportHeader title={title} />
      <div role="status" aria-busy="true" aria-label={`Loading ${title}`} className="space-y-4">
        <span className="sr-only">Loading {title}…</span>
        {variant === "cards" && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {[0, 1, 2, 3].map((index) => (
              <div key={index} className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4 space-y-2">
                <SkeletonBlock className="h-4 w-24" />
                <SkeletonBlock className="h-8 w-20" />
                <SkeletonBlock className="h-3 w-32" />
              </div>
            ))}
          </div>
        )}
        {variant === "chart" && (
          <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4">
            <SkeletonBlock className="h-4 w-32 mb-4" />
            <div className="flex items-end gap-1 h-48">
              {[70, 45, 90, 60, 80, 55, 100, 65, 75, 50, 85, 40].map((height, index) => (
                <SkeletonBlock key={index} className="flex-1" />
              ))}
            </div>
            <span className="sr-only">Chart data is loading</span>
          </div>
        )}
        {(variant === "table" || variant === "cards" || variant === "chart") && (
          <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 p-4 space-y-2">
            {[0, 1, 2, 3, 4].map((index) => (
              <SkeletonBlock key={index} className="h-6 w-full" />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export function ErrorState({ title, message, onRetry }: { title: string; message: string; onRetry: () => void }) {
  return (
    <div className="p-6 space-y-6">
      <ReportHeader title={title} />
      <div
        role="alert"
        className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200"
      >
        <p className="font-medium">Failed to load {title.toLowerCase()}</p>
        <p className="text-sm mt-1">{message}</p>
        <button
          type="button"
          onClick={onRetry}
          className="mt-3 rounded-md border border-red-300 dark:border-red-700 px-3 py-1.5 text-sm font-medium hover:bg-red-100 dark:hover:bg-red-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-600"
        >
          Retry
        </button>
      </div>
    </div>
  );
}
