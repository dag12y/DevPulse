import type { NextConfig } from "next";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// Read the tracker version from the workspace so the Install page can link
// the immutable /analytics-<version>.js bundle without a hardcoded copy that
// silently 404s (or, worse, keeps serving a stale version) after a release.
function trackerVersion(): string {
  try {
    const pkg = JSON.parse(
      readFileSync(join(process.cwd(), "..", "..", "packages", "tracker", "package.json"), "utf8"),
    ) as { version?: string };
    return pkg.version ?? "";
  } catch {
    return "";
  }
}

// Baseline headers for the dashboard. The nonce-based CSP itself is emitted
// per-request by middleware.ts (it needs a fresh nonce on every response);
// these are the static complements the API already sets for itself.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-DNS-Prefetch-Control", value: "off" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), interest-cohort=()" },
  // Honored only over HTTPS, so plain-HTTP local development is unaffected.
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
];

const nextConfig: NextConfig = {
  // Minimal self-contained server for the production Docker image.
  output: "standalone",
  env: {
    NEXT_PUBLIC_TRACKER_VERSION: trackerVersion(),
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
};

export default nextConfig;
