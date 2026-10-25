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

go run . -src "$work/cfgaudit" -version "${tag#v}" -reports "$work/reports" -rules "$work/rules.json" -out _site
