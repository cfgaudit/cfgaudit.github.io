---
title: "Secrets in Claude Code's .claude/settings.local.json: catch them before they ship"
description: "Claude Code's \"don't ask again\" writes typed secrets into .claude/settings.local.json. GitLab's fix is server-side and mostly Ultimate. cfgaudit catches it at the source: a pre-commit hook, a CI component, and across every agent."
date: 2026-11-11
slug: claude-code-dont-ask-again-secret-leak
summary: "GitLab and Apiiro flagged secrets committed in Claude Code's .claude/settings.local.json. The cheapest fix is to never commit them, and the leak is not unique to Claude Code."
---

Today GitLab published a [good post](https://about.gitlab.com/blog/prevent-leaks-from-claude-code-dont-ask-again-file/) about a real problem. When you choose "Yes, and don't ask again" for a shell command, Claude Code saves that exact command, including any token you typed, into `.claude/settings.local.json` under `permissions.allow`. The file stays out of git only through a personal global exclude on your own laptop, so it gets committed as soon as the repository is shared. Apiiro counted 104,185 copies of this file in public repositories, 1,122 of them with at least one secret, and more that were only in history.

GitLab's advice is solid: add the file to `.gitignore`, use a push rule, turn on Secret Push Protection and Secret Detection, and rotate the secret, because deleting the entry does nothing while the value is still in git history. Credit to them for the clear writeup.

Two things their post does not cover, and both matter.

## The cheapest fix is to never commit the secret

Almost every control GitLab lists is a server-side setting, and most of them need Premium or Ultimate. They also start after the secret has already reached the server. A pre-commit hook works on your own machine, before the commit even exists, for free, and on any tier or any forge.

## It is not unique to Claude Code

Every agent that remembers a "don't ask again" decision has a version of this file. The same leak pattern sits in Cursor, Copilot, Gemini, Codex and others.

## What cfgaudit does

[cfgaudit](https://github.com/cfgaudit/cfgaudit) is an open-source auditor for AI-agent configuration. For this exact problem:

- It flags a hardcoded secret inside a permission entry ([CFG007](/rules/CFG007/)), which is the pattern GitLab describes. The finding tells you to rotate the key, because removing the entry is not enough once it is in history.
- It flags the file when it is not covered by `.gitignore` ([CFG013](/rules/CFG013/)).
- It checks Claude Code, Cursor, Copilot, Gemini, Codex, Cline, Continue and Aider, not only `.claude/`.

You run it where it helps most: as a pre-commit hook that stops the commit, as a GitLab CI/CD component that catches it on push regardless of tier, or as a one-off scan of a repository.

GitLab's post is the right alarm, and their server-side controls are a good backstop. cfgaudit is the layer that stops the secret at the source, before it is ever pushed, and it works for any agent and any forge.

Disclosure: I maintain cfgaudit.
