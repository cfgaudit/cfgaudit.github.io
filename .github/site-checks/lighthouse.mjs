// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

// Reads Lighthouse JSON reports and fails when a page falls below the
// thresholds (#29). Usage: node lighthouse.mjs report.json...
import { readFileSync } from "node:fs";

// Measured against serve.py, which compresses like GitHub Pages; served
// uncompressed, /progress/ (441 KB of HTML, about 30 KB gzipped) scored 90 on
// mobile (#34).
const minimum = { accessibility: 100, seo: 100, "best-practices": 100, performance: 90 };

let failed = false;
for (const file of process.argv.slice(2)) {
  const report = JSON.parse(readFileSync(file, "utf8"));
  const scores = Object.entries(minimum).map(([id, min]) => {
    const score = Math.round((report.categories[id]?.score ?? 0) * 100);
    if (score < min) {
      failed = true;
      console.log(`::error::${report.finalDisplayedUrl}: ${id} ${score} < ${min}`);
      for (const ref of report.categories[id]?.auditRefs ?? []) {
        const audit = report.audits[ref.id];
        if (ref.weight > 0 && audit.score !== null && audit.score < 1) {
          console.log(`  ${audit.id}: ${audit.title}`);
        }
      }
    }
    return `${id} ${score}`;
  });
  console.log(`${report.finalDisplayedUrl}: ${scores.join(", ")}`);
}
process.exit(failed ? 1 : 0);
