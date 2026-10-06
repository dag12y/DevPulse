"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useProject } from "@/lib/project-context";
import Card from "@/components/ui/Card";
import PageHeader from "@/components/ui/PageHeader";

const INGEST_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:5000";
const TRACKER_URL =
  process.env.NEXT_PUBLIC_TRACKER_URL || `${INGEST_URL}/analytics.js`;

export default function InstallPage() {
  const { projects, selectedProject, selectedTrackingId, selectProject } = useProject();
  const [copied, setCopied] = useState(false);

  const trackingId = selectedTrackingId ?? projects[0]?.tracking_id ?? "dp_YOUR_TRACKING_ID";

  const versionedTrackerUrl = useMemo(() => {
    const version = process.env.NEXT_PUBLIC_TRACKER_VERSION;
    if (version) {
      return TRACKER_URL.replace("/analytics.js", `/analytics-${version}.js`);
    }
    return TRACKER_URL;
  }, []);

  const snippet = useMemo(
    () =>
      [
        "<script",
        `  src="${TRACKER_URL}"`,
        `  data-project="${trackingId}"`,
        `  data-endpoint="${INGEST_URL}/v1/analytics/events"`,
        "  defer>",
        "</script>",
      ].join("\n"),
    [trackingId],
  );

  const nextSnippet = useMemo(
    () =>
      [
        "<Script",
        `  src="${TRACKER_URL}"`,
        `  data-project="${trackingId}"`,
        `  data-endpoint="${INGEST_URL}/v1/analytics/events"`,
        '  strategy="afterInteractive"',
        "/>",
      ].join("\n"),
    [trackingId],
  );

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(snippet);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Install tracker"
        description={`Add one script tag. Events go to ${INGEST_URL}/v1/analytics/events. The tracking ID is public — reads stay behind your API key.`}
      />

      {projects.length > 1 && (
        <label className="block max-w-md">
          <span className="mb-1 block text-xs font-medium uppercase tracking-wide text-zinc-500">
            Project for this snippet
          </span>
          <select
            aria-label="Select project for install snippet"
            value={trackingId}
            onChange={(e) => selectProject(e.target.value)}
            className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm shadow-xs outline-none transition focus:border-indigo-500 focus:ring-4 focus:ring-indigo-500/15 dark:border-zinc-700 dark:bg-zinc-900"
          >
            {projects.map((project) => (
              <option key={project.tracking_id} value={project.tracking_id}>
                {project.name} · {project.tracking_id}
              </option>
            ))}
          </select>
        </label>
      )}

      <Card>
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-zinc-900 dark:text-white">Script tag</h3>
          <button
            onClick={copy}
            className="rounded-lg border border-zinc-300 px-3 py-1.5 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800"
            aria-live="polite"
          >
            {copied ? "Copied!" : "Copy"}
          </button>
        </div>
        <pre className="mt-3 overflow-x-auto rounded-xl bg-zinc-950 p-4 font-mono text-xs text-zinc-100">
          {snippet}
        </pre>
        {selectedProject && (
          <p className="mt-2 text-xs text-zinc-500">
            Allowed domains: {selectedProject.allowed_domains.join(", ") || "all"} · Timezone:{" "}
            {selectedProject.timezone} · Retention: {selectedProject.retention_days} days
          </p>
        )}
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card className="text-sm">
          <h3 className="font-semibold text-zinc-900 dark:text-white">Plain HTML</h3>
          <p className="mt-1 text-zinc-600 dark:text-zinc-400">
            Paste the tag before <code>&lt;/body&gt;</code>. Page views and SPA navigation are
            tracked automatically.
          </p>
        </Card>
        <Card className="text-sm">
          <h3 className="font-semibold text-zinc-900 dark:text-white">Next.js / React SPA</h3>
          <p className="mt-1 text-zinc-600 dark:text-zinc-400">
            Add the tag in <code>app/layout.tsx</code> with <code>next/script</code>, or include it
            once in your root component. Route changes via <code>pushState</code>/
            <code>replaceState</code>/<code>popstate</code> each send one page view.
          </p>
          <pre className="mt-3 overflow-x-auto rounded-xl bg-zinc-950 p-3 font-mono text-xs text-zinc-100">
            {nextSnippet}
          </pre>
        </Card>
      </div>

      <Card className="text-sm">
        <h3 className="font-semibold text-zinc-900 dark:text-white">Production bundle</h3>
        <p className="mt-1 text-zinc-600 dark:text-zinc-400">
          Served by the API with a 10&nbsp;KB gzip budget. Prefer the immutable versioned file:{" "}
          <code className="font-mono text-xs">{versionedTrackerUrl}</code> (
          <code>Cache-Control: immutable</code>). Events report{" "}
          <code>sdk_version</code> and retry failed sends twice with backoff, then fail silently.
        </p>
      </Card>

      <Card className="text-sm">
        <h3 className="font-semibold text-zinc-900 dark:text-white">Verify</h3>
        <ol className="mt-2 list-decimal space-y-1 pl-5 text-zinc-600 dark:text-zinc-400">
          <li>Deploy the tag to a domain in this project&apos;s allowed domains.</li>
          <li>Visit the site, then open Real-time — you should appear within seconds.</li>
          <li>If the API is down, the site keeps working; events fail silently with bounded retries.</li>
        </ol>
        <p className="mt-3">
          <Link className="font-medium text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400" href="/realtime">Open Real-time</Link>
          {" · "}
          <Link className="font-medium text-indigo-600 underline-offset-2 hover:underline dark:text-indigo-400" href="/">Open Overview</Link>
        </p>
      </Card>
    </div>
  );
}
