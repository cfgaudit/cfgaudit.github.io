#!/usr/bin/env bash
# Copyright the cfgaudit contributors
# SPDX-License-Identifier: Apache-2.0
#
# Builds the site into _site from the latest cfgaudit release: its docs, its
# rule list and the output of its binary on sample/ in every format. Needs git,
# go and GH_TOKEN for the GitHub API.
set -euo pipefail

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

tag=$(gh api repos/cfgaudit/cfgaudit/releases/latest --jq .tag_name)
[[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "unexpected cfgaudit tag: $tag" >&2; exit 1; }

echo "cfgaudit $tag"
git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$tag" https://github.com/cfgaudit/cfgaudit.git "$work/cfgaudit"
go build -C "$work/cfgaudit" -ldflags "-X main.cfgauditVersion=${tag#v}" -o "$work/cfgaudit-bin" ./cmd/cfgaudit

# The sample in every output format. cfgaudit exits 1 on the sample's error
# findings; anything else is a failure. --claude-version pins the rule gating,
# so the build does not depend on a local Claude Code.
mkdir -p "$work/reports"
cp -R sample "$work/reports/input"
for format in text table sarif codeclimate json; do
  rc=0
  # text and table carry cfgaudit's terminal colours, which the site renders.
  force=
  case $format in text | table) force=1 ;; esac
  (cd sample && FORCE_COLOR=$force "$work/cfgaudit-bin" --claude-version 2.1.310 --format "$format" .) > "$work/reports/$format.txt" 2> "$work/stderr.txt" || rc=$?
  [ "$rc" -eq 1 ] || { echo "cfgaudit --format $format exited $rc on the sample" >&2; cat "$work/reports/$format.txt" "$work/stderr.txt" >&2; exit 1; }
done

"$work/cfgaudit-bin" list --format json > "$work/rules.json"

# The releases with their commits, for the progress page's charts and the
# release markers on the benchmark charts.
gh api --paginate repos/cfgaudit/cfgaudit/tags --jq '.[] | {tag: .name, sha: .commit.sha}' | jq -s . > "$work/tags.json"
gh api --paginate repos/cfgaudit/cfgaudit/releases --jq '.[] | select(.draft | not) | {tag: .tag_name, date: .published_at, url: .html_url}' | jq -s . > "$work/published.json"
jq -n --slurpfile t "$work/tags.json" --slurpfile p "$work/published.json" \
  '[$p[0][] as $r | ($t[0][] | select(.tag == $r.tag)) as $tag | $r + {sha: $tag.sha}]' > "$work/releases.json"

# Per release: the number of rule docs in its tree and its README, from
# which the generator counts the agents it covers (#21). A tag's tree never
# changes, so two API calls per release.
mkdir -p "$work/history/readmes"
jq -r '.[] | [.tag, .sha] | @tsv' "$work/releases.json" | while IFS=$'\t' read -r rtag rsha; do
  [[ $rtag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
  n=$(gh api "repos/cfgaudit/cfgaudit/git/trees/$rsha?recursive=1" --jq '[.tree[].path | select(test("^docs/rules/CFG[0-9]{3}\\.md$"))] | length')
  gh api -H "Accept: application/vnd.github.raw" "repos/cfgaudit/cfgaudit/contents/README.md?ref=$rsha" > "$work/history/readmes/$rtag.md"
  jq -n --arg tag "$rtag" --argjson rules "$n" --slurpfile r "$work/releases.json" '$r[0][] | select(.tag == $tag) | {tag, date, url, rules: $rules}'
done | jq -s . > "$work/history/history.json"

# The benchmark history from the bench-data branch (cfgaudit/cfgaudit#1019).
# Without the branch the progress page has no performance section.
if ! gh api -H "Accept: application/vnd.github.raw" "repos/cfgaudit/cfgaudit/contents/bench.jsonl?ref=bench-data" > "$work/bench.jsonl" 2> "$work/bench.err"; then
  grep -q "404" "$work/bench.err" || { cat "$work/bench.err" >&2; exit 1; }
  echo "no bench-data branch yet; building without benchmark charts"
  rm -f "$work/bench.jsonl"
fi

go run . -src "$work/cfgaudit" -version "${tag#v}" -reports "$work/reports" -rules "$work/rules.json" -bench "$work/bench.jsonl" -releases "$work/releases.json" -history "$work/history" -out _site
