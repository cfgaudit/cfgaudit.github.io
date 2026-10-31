# cfgaudit.github.io

Source of <https://cfgaudit.github.io>, the website and rule reference for [cfgaudit](https://github.com/cfgaudit/cfgaudit).

The rule pages are generated from `docs/rules/CFGxxx.md` in the cfgaudit repository, at the latest release tag, so the site always describes the version people can install. The rule overview and the OWASP page sort the rules from that release's `cfgaudit list --format json` into the OWASP Top 10 for LLM Applications 2026 and the OWASP MCP Top 10. The example on the home page and the example reports are the output of that release's binary on the small repository in [`sample/`](sample), in every format.

The progress page charts how cfgaudit grows from release to release. The build fetches the rule count and the README of every release tag through the GitHub API (two calls per tag), counts the agents each README covers, takes the rules per agent from the latest release's `cfgaudit list --format json` (the `agents` field, cfgaudit#1036), and adds the benchmark history from `bench.jsonl` on the `bench-data` branch. The charts are inline SVG drawn at build time, one measure per chart, with no script. `/benchmarks/`, the page's old address, redirects to its performance section.

## Building

Needs Go, git and the GitHub CLI, logged in or with `GH_TOKEN` set.

```sh
./scripts/build.sh                     # writes _site/
python3 -m http.server -d _site 8000   # http://localhost:8000
go test ./...
```

The build fails on a rule doc whose first line isn't `# CFGxxx — Title` (or `# CFGxxx: Title`), on a relative link that leaves the cfgaudit repository, on any internal link without a page, and when the rule list and the rule docs disagree.

## Checks

`go test ./...` covers the generator, plus the SEO basics of every page (`seo_test.go`), the Content-Security-Policy and safe rendering of the rule docs (`safety_test.go`) and accessibility markup (`a11y_test.go`). The built site in `_site/` is checked in CI (`site-checks.yml`) with axe in light and dark mode, html-validate and stylelint. `lighthouse.yml` runs Lighthouse on `main`, weekly and by hand. The pinned tools live in `.github/site-checks/`. To run the checks locally:

```sh
./scripts/build.sh
(cd .github/site-checks && npm ci --ignore-scripts)
.github/site-checks/node_modules/.bin/html-validate --config .github/site-checks/htmlvalidate.json "_site/**/*.html"
.github/site-checks/node_modules/.bin/stylelint --config .github/site-checks/stylelint.json static/style.css
python3 -I .github/site-checks/serve.py _site 8000 &   # gzip, like GitHub Pages
CHROME_PATH=$(command -v google-chrome) node .github/site-checks/axe.mjs http://localhost:8000 .github/site-checks/pages.txt
```

## Deployment

`.github/workflows/deploy.yml` publishes to GitHub Pages on every push to `main`, once a day, on `workflow_dispatch` and on a `cfgaudit-release` repository dispatch.

To change rule text, edit the doc in [cfgaudit](https://github.com/cfgaudit/cfgaudit/tree/main/docs/rules). It appears here with the next release.

## License

Apache-2.0, like cfgaudit.
