# cfgaudit.github.io

Source of <https://cfgaudit.github.io>, the website and rule reference for [cfgaudit](https://github.com/cfgaudit/cfgaudit).

The rule pages are generated from `docs/rules/CFGxxx.md` in the cfgaudit repository, at the latest release tag, so the site always describes the version people can install. The rule overview and the OWASP page sort the rules from that release's `cfgaudit list --format json` into the OWASP Top 10 for LLM Applications 2026 and the OWASP MCP Top 10. The example on the home page and the example reports are the output of that release's binary on the small repository in [`sample/`](sample), in every format.

The benchmarks page charts `bench.jsonl` from the `bench-data` branch of the cfgaudit repository, which the Benchmark history workflow there extends on every push to `main`. The charts are inline SVG drawn at build time, one unit per chart, with the releases marked. Without that branch the site builds without the page.

## Building

Needs Go, git and the GitHub CLI, logged in or with `GH_TOKEN` set.

```sh
./scripts/build.sh                     # writes _site/
python3 -m http.server -d _site 8000   # http://localhost:8000
go test ./...
```

The build fails on a rule doc whose first line isn't `# CFGxxx — Title` (or `# CFGxxx: Title`), on a relative link that leaves the cfgaudit repository, on any internal link without a page, and when the rule list and the rule docs disagree.

## Deployment

`.github/workflows/deploy.yml` publishes to GitHub Pages on every push to `main`, once a day, on `workflow_dispatch` and on a `cfgaudit-release` repository dispatch.

To change rule text, edit the doc in [cfgaudit](https://github.com/cfgaudit/cfgaudit/tree/main/docs/rules). It appears here with the next release.

## License

Apache-2.0, like cfgaudit.
