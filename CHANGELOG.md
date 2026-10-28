# Changelog

All notable changes to this repository are tracked here for agent context.

## [Unreleased]

### Added

- IBC with ibc-go v11.2.0: IBC core, ICS-20 token transfer (classic channels and IBC v2 routes) and the 07-tendermint light client, plus the redundant-relay ante decorator. The transfer module account can mint and burn (escrow of XPX, vouchers for incoming coins). Modules missing from genesis.json get their default genesis in InitChainer. No channel is open until a relayer creates one.

### Changed

- Moved from the Cosmos SDK v0.54.0-rc.1 pre-release (Feb 2026) to the stable v0.54.4, CometBFT v0.39.0-beta.2 to v0.39.4, and the store module to its new path `github.com/cosmos/cosmos-sdk/store/v2`. The app constructor no longer takes a trace writer (removed in v0.54). This is the base for adding IBC (ibc-go v11 targets SDK v0.54 / CometBFT v0.39).
- Store markers keep the authz, feegrant and evidence stores non-empty: an empty IAVL store could not be loaded at any version, which broke every REST query and node restarts.


### Docs Sync Fixes

- Fixed `git diff --quiet` in both sync workflows — it missed untracked files (new tutorial pages would silently skip sync). Replaced with `git status --porcelain <dir>` which catches new, modified, and deleted files.
- Moved `transform.py` into `cosmos/docs` repo (`scripts/docs-sync/transform.py`) so the workflow no longer copies and executes an unverified script from an external repo; changes to the script now go through PR review on the docs repo.

### Tutorial Branch Automation

- Added `scripts/create-tutorial-branch.sh` to `main` (previously only on `tutorial/start`); updated to use `git checkout -B` for existing branch and cross-platform `perl -i` instead of `sed -i ''`
- Added `.github/workflows/update-tutorial-branch.yml` — regenerates `tutorial/start` automatically on every push to `main`; skips `[tutorial-sync]` and `[docs-sync]` commits to prevent loops
- Updated `CLAUDE.md` to document the automation

### Docs Changes

- Renamed tutorial docs to `NN-name.md` format (`01-prerequisites.md` through `05-run-and-test.md`), added `00-overview.md` intro page
- `02-quickstart.md`: rewrote opening paragraph, added Mintlify Note callout linking to prerequisites
- `03-build-a-module.md`: replaced blockquote prerequisite notice with Mintlify Note callout; added links to SDK concept docs throughout (modules, transactions, encoding, keeper, app.go, etc.)
- `04-counter-walkthrough.md`: added anchor links in feature comparison table; added Gas section covering `minimum-gas-prices` in `app.toml`; linked repo in branch switch instruction; removed stale to-do comment
- `05-run-and-test.md`: added Node Configuration section covering `app.toml` and `config.toml` with key settings tables
- `01-prerequisites.md`: updated Go version output to show both Linux and macOS variants; updated Make install instructions to cover both platforms
- Verified `make install` builds successfully on Linux via Docker `golang:1.25` container

### CLAUDE.md Changes

- Added Changelog Policy section requiring changelog updates after every change

---

## [2026-03-18] — Docs sync system + file renames

Branch: `feat/docs-sync` (in `cosmos/example`), `feat/example-tutorial-sync` (in `cosmos/docs`)

### example repo changes
- Renamed all 5 tutorial docs to drop the `tutorial-NN-` prefix:
  - `tutorial-00-prerequisites.md` → `prerequisites.md`
  - `tutorial-01-quickstart.md` → `quickstart.md`
  - `tutorial-02-build-a-module.md` → `build-a-module.md`
  - `tutorial-03-counter-walkthrough.md` → `counter-walkthrough.md`
  - `tutorial-04-run-and-test.md` → `run-and-test.md`
- Updated all internal cross-links between tutorial files to use new names
- Added `scripts/docs-sync/transform.py` — bidirectional transform between `.md` and Mintlify `.mdx` format (H1 ↔ frontmatter, absolute ↔ relative links, `.md` ↔ `.mdx` extensions)
- Added `scripts/docs-sync/test_transform.py` — 25 unit + round-trip tests (all passing)
- Added `.github/workflows/docs-sync.yml` — GitHub Action that auto-opens PRs on `cosmos/docs` when `docs/**` changes on `main`

### cosmos/docs repo changes
- Fixed typo: `prerequisits.mdx` → `prerequisites.mdx`
- Added 5 new `.mdx` files in `sdk/next/tutorials/example/` (transformed from `docs/*.md`)
- Added `Build a Chain` group to `docs.json` navigation under `sdk/next` How-to Guides
- Added `.github/workflows/docs-sync-to-example.yml` — reverse sync action that opens PRs on `cosmos/example` when tutorial `.mdx` files change

### Setup required (one-time)
- Add secret `DOCS_REPO_TOKEN` to `cosmos/example` (PAT: `contents:write` + `pull-requests:write` on `cosmos/docs`)
- Add secret `EXAMPLE_REPO_TOKEN` to `cosmos/docs` (PAT: `contents:write` + `pull-requests:write` on `cosmos/example`)

---

## [2026-03-18] — Initial setup

- Added `CLAUDE.md` to document repo purpose, branch strategy, docs policy, and module architecture for future agents.
- Added `CHANGELOG.md` (this file) to track changes over time.

---

## Format

Each entry should follow:

```
## [YYYY-MM-DD] — Short description

- What changed and why
- Docs site impact (if any)
- Branch(es) affected
```
