<!-- markdownlint-disable first-line-heading -->

![header.png](https://raw.githubusercontent.com/shikanime/shikanime/main/assets/github-header.png)

<!-- markdownlint-enable first-line-heading -->

## Overview

Automata is a fast, ergonomic CLI to maintain Kubernetes clusters and repo
hygiene:

- Updates kustomize image tags and recommended labels across `kustomization.yaml`
- Bumps GitHub Actions in `.github/workflows` to latest major versions
- Runs `update.sh` scripts discovered under a directory tree

## Requirements

- Go `1.24.5`
- Git (used for `.gitignore` detection)
- Bash (to run `update.sh` scripts)
- Optional: `GITHUB_TOKEN` for authenticated GitHub API requests

Environment variables:

- `LOG_LEVEL`: `debug`, `info`, `warn`, `error` (default `info`)
- `GITHUB_TOKEN`: personal access token to increase GitHub API rate limits

## Installation

```bash
go build -o automata ./cmd/automata
```

Alternatively:

- `go install github.com/shikanime-studio/automata/cmd/automata@latest`
- With Nix: check `flake.nix` and use your preferred `nix build` workflow

## Usage

- Show help:

```bash
./automata --help
```

- Run everything:

```bash
./automata update --all [DIR]
```

- Only update kustomize image tags and labels:

```bash
./automata update kustomization [DIR]
```

- Only update GitHub Actions versions in workflows:

```bash
./automata update githubworkflow [DIR]
```

- Only run discovered `update.sh` scripts:

```bash
./automata update updatescript [DIR]
```

Notes:

- `[DIR]` defaults to `.` if omitted
- Files/dirs ignored by `.gitignore` are skipped (via `git check-ignore`)
- Tasks are executed concurrently where applicable

## Manifests

Hey 🌸 I'm Shikanime Deva, this is the Kubernetes automata of my clusters.

### Kustomize Annotations

Automata reads image update configuration from a kustomize annotation:

- Key: `automata.shikanime.studio/images`
- Value: JSON array of objects configuring per-image tag selection

Example `kustomization.yaml`:

```yaml
labels:
  - pairs:
      app.kubernetes.io/name: myapp
      app.kubernetes.io/version: v1.2.3
images:
  - name: myapp
    newName: ghcr.io/org/myapp
    newTag: v1.2.3
annotations:
  automata.shikanime.studio/images: |
    [
      {
        "name": "myapp",
        "tag-regex": "^(?P<version>v\\d+\\.\\d+\\.\\d+)(?P<prerelease>-[^+]+)?(\\+.*)?$",
        "exclude-tags": ["v1.2.3"],
        "update-strategy": "FullUpdate"
      }
    ]
```

Behavior:

- Extracts semver from tags (supports named groups like `version`, or `major`/`minor`/`patch`)
- Skips non-semver and prerelease tags unless configured to include them
- Honors `exclude-tags` to avoid specific tags
- Applies update strategy:
  - `FullUpdate`: any greater version
  - `MinorUpdate`: same major
  - `PatchUpdate`: same major.minor

### GitHub Workflows

Automata scans `.github/workflows/*.yml` and updates `uses: owner/repo@vX` to the latest suitable tag:

- Only semver tags are considered
- Prerelease tags are skipped unless configured
- Requires `GITHUB_TOKEN` to avoid low anonymous API rate limits

### Update Scripts

Automata finds and runs `update.sh` scripts:

- Executes each `update.sh` with `bash` from its directory
- Logs combined output and continues across scripts
