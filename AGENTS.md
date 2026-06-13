# automata

A fast, ergonomic CLI to maintain Kubernetes clusters and repo hygiene: updates kustomize image tags, bumps GitHub Actions versions, and runs `update.sh` scripts.

**Language:** Go

## Structure

- `cmd/automata/` — CLI entry point
- `pkg/` — Internal packages (kustomize, actions, updater)
- `flake.nix` — Nix development shell

## Commit Style

- Plain-text capitalized title, no conventional-commit prefix
- Body with labels: `Design:`, `Related:`, `Closes #`
- Keep Markdown lines wrapped at 80 columns and run `nix fmt` before shipping

## Stack

- 1 commit == 1 PR via ghstack
- Amend + `ghstack` to resubmit
- `ghstack land` on head PR to land the entire stack
- Never `gh pr merge` (creates poisoned commits)
- Never force-push ghstack branches
- ghstack only works on HEAD commit chains, not detached HEADs

## Protect `main`

- Require 1 approving review
- Require linear history (no merge commits)
- Require signed commits
- Squash+rebase merge only

*Licensed under Apache-2.0. Test against real `kustomization.yaml` files before submitting*