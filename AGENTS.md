# automata

CLI for Kubernetes cluster maintenance and repo hygiene (kustomize, Actions, update.sh).

**Language:** Go

**Structure:** `cmd/automata/` — entry; `pkg/` — kustomize, actions, updater; `flake.nix` — dev shell

**Commit style:** Plain-text capitalized title, no prefix. Body with labels: `Design:`, `Related:`, `Closes #`.

**Stack:** 1 commit == 1 PR via ghstack. Amend + `ghstack` to resubmit. `ghstack land` on head PR to land stack. Never `gh pr merge`. Never force-push.

**Protect `main`:** 1 review, linear history, signed commits, squash+rebase only.

*Apache-2.0. Test against real kustomization.yaml files*
