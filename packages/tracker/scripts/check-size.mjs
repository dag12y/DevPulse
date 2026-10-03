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
