// Copyright the cfgaudit contributors
// SPDX-License-Identifier: Apache-2.0

// Runs axe-core on each page in light and dark mode and fails on any
// violation (#29). The site has a dark colour scheme, so contrast is checked
// in both. Usage: node axe.mjs <base URL> <pages file>
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import puppeteer from "puppeteer-core";

const axeSource = readFileSync(createRequire(import.meta.url).resolve("axe-core/axe.min.js"), "utf8");
const [base, pagesFile] = process.argv.slice(2);
const pages = readFileSync(pagesFile, "utf8").split("\n").filter(Boolean);

const browser = await puppeteer.launch({
  executablePath: process.env.CHROME_PATH,
  args: ["--headless=new"],
});
let failed = false;
for (const path of pages) {
  for (const scheme of ["light", "dark"]) {
    const page = await browser.newPage();
    // The site's Content-Security-Policy allows no script; axe is injected
    // for the check only.
    await page.setBypassCSP(true);
    await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: scheme }]);
    await page.goto(base + path, { waitUntil: "load" });
    await page.addScriptTag({ content: axeSource });
    const result = await page.evaluate(() =>
      window.axe.run(document, { runOnly: ["wcag2a", "wcag2aa", "wcag21aa", "best-practice"] }),
    );
    if (result.violations.length === 0) {
      console.log(`${path} (${scheme}): no violations`);
    }
    for (const v of result.violations) {
      failed = true;
      console.log(`::error::${path} (${scheme}): ${v.id}: ${v.help} (${v.nodes.length})`);
      for (const n of v.nodes.slice(0, 5)) {
        console.log(`  ${n.target.join(" ")}`);
      }
    }
    await page.close();
  }
}
await browser.close();
process.exit(failed ? 1 : 0);
