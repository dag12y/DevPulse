import { gzipSync } from "node:zlib";
import { readFileSync, statSync } from "node:fs";

// Size budget: tracker must stay small so it never slows down host sites.
// Budget is 10 KB gzipped; fail the build if it grows past that.
const BUDGET_GZIP_BYTES = 10 * 1024;
const file = new URL("../dist/analytics.js", import.meta.url);
const raw = readFileSync(file);
const gzipped = gzipSync(raw).length;
const { size } = statSync(file);
console.log(`tracker raw=${size}B gzip=${gzipped}B budget=${BUDGET_GZIP_BYTES}B`);
if (gzipped > BUDGET_GZIP_BYTES) {
  console.error(`tracker size budget exceeded: gzip ${gzipped}B > ${BUDGET_GZIP_BYTES}B`);
  process.exit(1);
}

// Correctness guard: the ingest endpoint is derived from the origin the
// bundle was loaded from, so no host may be baked into the published file.
// A baked-in localhost would silently send customers' events nowhere.
const source = raw.toString("utf8");
const forbidden = [...source.matchAll(/https?:\/\/[a-z0-9.-]+/gi)]
  .map((match) => match[0].toLowerCase())
  .filter((url) => url !== "https://www.w3.org");
if (forbidden.length > 0) {
  console.error(`tracker bundle must not embed an absolute host URL: ${[...new Set(forbidden)].join(", ")}`);
  console.error("derive the ingest endpoint from the script src instead of hardcoding it");
  process.exit(1);
}
