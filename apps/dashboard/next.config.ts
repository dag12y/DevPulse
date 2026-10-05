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

const nextConfig: NextConfig = {
  // Minimal self-contained server for the production Docker image.
  output: "standalone",
  env: {
    NEXT_PUBLIC_TRACKER_VERSION: trackerVersion(),
  },
};

export default nextConfig;
