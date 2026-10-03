import { copyFileSync, readFileSync } from "node:fs";

const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
const version = pkg.version;
copyFileSync(new URL("../dist/analytics.js", import.meta.url), new URL(`../dist/analytics-${version}.js`, import.meta.url));
console.log(`wrote dist/analytics-${version}.js`);
