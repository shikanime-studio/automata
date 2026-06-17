# Automata

A fast, ergonomic CLI to maintain Kubernetes clusters and repo hygiene: updates
kustomize image tags, bumps GitHub Actions versions, and runs `update.sh`
scripts.

**Language:** Go

## Structure

- `cmd/automata/` — CLI entry point
- `pkg/` — Internal packages (kustomize, actions, updater)
- `flake.nix` — Nix development shell

## Capabilities

- Update kustomize image tags across manifests
- Bump GitHub Actions versions in workflow files
- Run `update.sh` scripts for automated repo maintenance

## Commit Style

- Plain-text capitalized title, no conventional-commit prefix
- Body with labels: `Design:`, `Related:`, `Closes #`
- Keep Markdown lines wrapped at 80 columns and run `nix fmt` before shipping

## Stack

- 1 commit == 1 PR via ghstack (1 commit is 1 logical atomic change)
- The commit title **is** the PR title; the commit body **is** the PR body
- Split work into stacked PRs to keep each PR small and reviewable
- To pull down an existing stack: `ghstack checkout <PR_NUMBER>`
- To update a PR: edit files, then `jj squash` (or `git commit --amend`) into the
  **target commit** of the stack — the one that PR represents; the commit message
  updates the PR title and body automatically when resubmitted
- Resubmit with `ghstack` after squashing
- `ghstack land` on the head PR to land the entire stack
- Never `gh pr merge` (creates poisoned commits)
- Never force-push ghstack branches


## Protect `main`

- Require 1 approving review
- Require linear history (no merge commits)
- Require signed commits
- Squash+rebase merge only

*Licensed under Apache-2.0. Test against real `kustomization.yaml` files before
submitting. Always use worktrees when making changes.*
