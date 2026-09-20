---
name: create-github-pr
description: >-
  Open GitHub pull requests with a concise change summary, test plan, and
  linked ticket. Use when creating a PR, opening a pull request, running
  gh pr create or another environment PR API/tool, requesting review,
  converting a draft PR to ready, or preparing changes for review and merge.
  These rules apply even if you did not run gh.
---

# Create GitHub pull request

Use `gh` for GitHub work (issues, PRs, checks) when it is available. The same
ready/draft, description, and CI rules apply if the PR is opened with another
environment PR API or tool. Do not skip this skill because you did not run
`gh`. Do not use the TodoWrite or Task tools. Return the PR URL when done.

## Ready vs draft

- **Ready (never draft):** the change is complete and meant to be reviewed,
  approved, and merged.
- **Draft:** reserved solely for works-in-progress and cases where the contents
  are experimental.

A PR that is genuinely ready to be reviewed, approved, and merged should never
be in draft. Do not open a complete PR as draft to wait on CI. If a PR is
already draft and the work is now complete, mark it ready, then wait for CI
green before presenting it for review.

## Description

Every PR description includes a concise description of the changes and how they
were tested. Use this body:

```markdown
## Summary
<1-5 bullets: what changed and why>

## Test plan
- [ ] <how it was actually tested>

```

If the PR stemmed from a GitHub issue or other ticketing system, link the ticket
in the description. When this PR fully resolves a GitHub issue, use
`Fixes #N` / `Closes #N` so merge closes it.

Do not put in the description (or the code):

- What was not done, out of scope, skipped, or left for later
- Implementation diary, file lists, or AI/tool attribution
- Ticket numbers in **code** (source, comments, in-repo docs). Tickets belong
  in the PR description only.

If follow-up work exists, file or link an issue. Do not document omissions in
the PR.

**Title:** short, imperative, matches the change. No ticket numbers.

## CI before review

All CI status checks must pass before a PR is given for review.

That means: no reviewer pings, no `--reviewer`, no "please review" / "ready for
review" to the user, until `gh pr checks` (or the equivalent check API) is all
pass. CODEOWNERS may auto-subscribe on open; still wait for green before
treating it as ready.

If checks fail, fix and push; re-wait. Do not hand off a red PR.

## Workflow

1. Inspect `git status`, `git diff`, `git log`, and
   `git diff <base>...HEAD`. Base is the PR target branch, usually `main`.
   Confirm tracking vs origin.
2. Self-review the diff: focused to one concern, no secrets, no ticket numbers
   in code, no "what we didn't do" comments. If client GDScript changed, run
   `make lint` and `make test` from `client/` first (see
   [gdscript-client-quality](../gdscript-client-quality/SKILL.md)).
3. Push with `-u` if the branch is not on origin. Do not update git config,
   force-push `main`/`master`, skip hooks, or use `git -i`.
4. `gh pr create` (or the environment PR API) with the title and body above.
   Ready unless the work is WIP/experimental (or the user asked for a draft
   for those reasons).
5. If the PR is already draft and the work is complete and meant for
   review/merge, mark it ready for review, then wait for CI green before
   presenting it for review (same CI gate as a fresh ready open).
6. `gh pr checks --watch` (or equivalent). On failure, fix, push, watch again.
7. Return the PR URL only after checks pass (draft/WIP: URL is enough; do not
   present it for review).

## Examples

**Ready PR from issue #42 (CI still running — not yet for review):**

```markdown
## Summary
- Persist world difficulty on create and send it to guests on join.

Fixes #42

## Test plan
- [x] Created Easy/Normal/Hard worlds and confirmed the value in the save blob
- [x] Joined as a guest and saw the host difficulty
```

**Do not write:** "Not handling migration of old saves (follow-up)." File that
as an issue instead.

**Do not write in code:** `// Fixes #42` or `TODO(#42): ...`.
