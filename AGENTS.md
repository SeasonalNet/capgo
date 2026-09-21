# AGENTS.md

## Project identity

`capgo` is a SeasonalNet Go library implementing OASIS CAP 1.2 with CAP-CP,
IPAWS, and NWS profile validation.

- Primary repository: `forge-git:SeasonalNet/capgo` on SeasonalForge.
- Canonical module path: `git.seasonalnet.org/SeasonalNet/capgo`.
- GitHub is a mirror at `github.com/SeasonalNet/capgo`, not the primary forge.
- The project is licensed under GPLv3-only. Keep the complete `LICENSE`
  text and update the README when licensing metadata changes.

## Quality interface

Use the pinned tools in `mise.toml` and run:

```sh
mise install
mise exec -- make check
```

The check includes formatting, `golangci-lint`, `go vet`, tests, and gitleaks.
Keep profile behavior and CAP lifecycle semantics covered by regression tests.

## Scope boundaries

- Keep the core library dependency-free and standard-library based.
- Keep profile-specific rules under `profiles/`.
- Do not treat CAP `Cancel` as a replacement for producer-specific lifecycle
  semantics such as NWS VTEC.
- Do not add sender authorization, cryptographic trust policy, or network
  delivery behavior to this library.
