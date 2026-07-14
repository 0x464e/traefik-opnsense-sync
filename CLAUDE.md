# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`traefik-opnsense-sync` is a Go daemon that keeps OPNsense Unbound DNS host aliases in sync with a
hostname source, computes the desired set of DNS aliases, and reconciles them against the aliases
attached to a single OPNsense Unbound "host override" entry — creating and deleting aliases so that
every proxied hostname resolves to the reverse proxy without manual DNS edits. The hostname source
is selected via `sync.source`:

- `traefik` (default) — polls the Traefik API for router `Host`/`HostRegexp` rules.
- `kubernetes` — reads `Ingress`, Traefik `IngressRoute`, and/or Gateway API `HTTPRoute` objects
  directly from a Kubernetes cluster (in-cluster auth only).

## Commands

```bash
go build ./...                 # compile
go run ./cmd/traefik-opnsense-sync   # run (needs a config file / env vars, see below)
go test ./...                  # run all tests
go test -run TestName ./internal/...  # run a single test
go vet ./...                   # static checks
gofmt -w .                     # format
```

Version metadata (`version`, `commit`, `date`, `builtBy` in `cmd/traefik-opnsense-sync/main.go`) is
injected at image-build time by the `builder` stage of `Dockerfile` via `-X main.*` ldflags (see
`.forgejo/workflows/build-push.yml`); a plain `go build` leaves the `"dev"`/`"unknown"` defaults.
Run the binary with `-v`/`--version`/`version` to print it.

## Running locally

The binary requires configuration before it will start. Provide either a `config.yaml`/`config.yml`
in the working directory (see `config.example.yml`), a path via `TOS_CONFIG`, or `TOS_*` env vars.
Required keys: `traefik.base_url`, `opnsense.base_url`, `opnsense.api_key`, `opnsense.api_secret`,
`opnsense.host_override`. Set `sync.dry_run: true` (or `TOS_SYNC_DRY_RUN=true`) to compute and log
the plan without mutating OPNsense — in dry-run the app performs exactly one sync and exits.

## Architecture

Data flows in one direction each sync tick: **source (Traefik or Kubernetes, selected via
`sync.source`) → reconcile → OPNsense (sink)**.

- `cmd/traefik-opnsense-sync/main.go` — entrypoint: parses the version flag, loads config, wires
  signal-based context cancellation, and starts the app.
- `internal/app` — the run loop. Does an initial sync, then (unless dry-run) ticks every
  `sync.interval`. Only logs when changes are detected.
- `internal/syncer` — the core.
  - `runner.go` defines the `Source` interface (`DesiredAliases(ctx) ([]model.HostAlias, error)`),
    which both `internal/traefik` and `internal/kubernetes` implement — this is the pluggable
    boundary between "get hostnames from somewhere" and reconciliation. `NewRunner` picks the
    concrete `Source` from `cfg.Sync.Source` and can fail (e.g. the Kubernetes source can't reach
    the in-cluster API), so both it and `app.NewApp` return errors. `Runner.Sync` orchestrates one
    cycle: find the OPNsense host-override UUID → fetch its current aliases → `source.DesiredAliases`
    → `engine.computePlan` → `executePlan`. Execution applies deletes/creates and, if anything
    changed, calls `ReconfigureUnbound` **once** at the end to apply the batch.
  - `engine.go` (`Engine.computePlan`) is pure/deterministic and fully source-agnostic: it takes the
    desired `[]model.HostAlias` from whichever `Source` ran, derives current aliases from OPNsense
    (filtered to only those whose description equals `sync.description_tag`), diffs the two keyed
    sets, and emits ordered operations (deletes before creates, then alphabetical).
- `internal/traefik` — API client (`GET /api/http/routers`), `parser.go`, and `source.go`.
  - `parser.go` parses Traefik router **rule expressions** (`Host(...) && !HostRegexp(...) || ...`)
    into a boolean tree using `vulcand/predicate`, then collects `Host` (literal) and `HostRegexp`
    (regex) matches while honoring negation (`!Host(...)` values are excluded). The parser is
    adapted from Traefik's own source; treat it as load-bearing and preserve its semantics.
  - `source.go` (`Source.DesiredAliases`) applies the entrypoint/provider/router filters to routers,
    then converts matched rules into `[]model.HostAlias` via `DomainsToHostAliases` — expanding
    `HostRegexp` matches through `internal/exrex` and splitting literal FQDNs via
    `model.NewHostAliasFromFQDN`. `DomainsToHostAliases` is exported because `internal/kubernetes`'s
    IngressRoute handling reuses it (IngressRoute `match` strings are the same rule syntax).
- `internal/exrex` — expands `HostRegexp` patterns into concrete hostnames by shelling out to the
  external Python `exrex` tool (`--max-number <regex.max_generated>`). Results are cached per
  pattern in-process; the executable is resolved lazily once. If `exrex` is absent, regex expansion
  fails but literal-host sync still works.
- `internal/kubernetes` — the Kubernetes `Source`. Connects to the cluster using **only** the pod's
  in-cluster service account (`client.go`, `rest.InClusterConfig()` — no kubeconfig fallback), then
  uses a single `dynamic.Interface` client uniformly for all three resource kinds it can scan
  (`kubernetes.resources`, default `["ingress"]`):
  - `ingress.go` — core `networking.k8s.io/v1` Ingress, reads `spec.rules[].host`.
  - `ingressroute.go` — Traefik `traefik.io/v1alpha1` IngressRoute CRD, reads `spec.routes[].match`
    (only `kind: Rule` routes) and hands it to `traefik.ParseDomains`/`DomainsToHostAliases`.
  - `httproute.go` — Gateway API `gateway.networking.k8s.io` HTTPRoute, reads `spec.hostnames`.
  - `filter.go` — cluster-wide list + in-memory namespace include/ignore filtering
    (`kubernetes.include_namespaces`/`ignore_namespaces`, mutually exclusive) and per-resource
    ignore list (`kubernetes.ignore_resources`, `name.namespace@kind` entries).
  - Wildcard hostnames (`*.example.com`) from Ingress/HTTPRoute have no expansion mechanism and are
    skipped with a warning, same as an unparseable literal domain.
  - See `deploy/k8s/rbac-example.yaml` for the minimum RBAC the service account needs — only grant
    rules for the resource kinds actually enabled in `kubernetes.resources`.
- `internal/opnsense` — API client for Unbound settings: search host overrides, search/add/delete
  host aliases, and reconfigure. Note the interface methods are the reconciliation contract used by
  the runner. Fully source-agnostic; unaffected by the Traefik/Kubernetes source split.
- `internal/model` — shared domain types: `HostAlias` (keyed by `hostname.domain` via `Key()`),
  `Operation`/`OpKind` (Create/Delete), `Plan`, and `NewHostAliasFromFQDN` (the one place FQDN→
  hostname/domain splitting happens, shared by both sources).
- `internal/httpx` — thin JSON HTTP helper shared by the Traefik and OPNsense API clients (the
  Kubernetes source uses `client-go` instead). Handles TLS-verify toggling, 10s timeout, basic auth
  (Traefik user/pass; OPNsense key/secret), and non-2xx → error with a 4KB body snippet.
- `internal/config` — Viper-based loader (`config.go`). See below.

## Key invariants and gotchas

- **The `description_tag` is the ownership marker.** The engine only ever considers OPNsense aliases
  whose `Description` exactly matches `sync.description_tag` as "managed by us". Anything else is
  invisible and untouched. Changing the tag orphans previously-created aliases, so treat it as
  stable identity, not cosmetic text.
- **Reconcile is create/delete only** (no in-place update), keyed by `hostname.domain`. Everything
  under the configured host override that carries our tag but is absent from the active source
  (Traefik or Kubernetes) gets deleted.
- **Only one source runs per cycle.** `sync.source` selects Traefik or Kubernetes exclusively; there
  is no merging of both sources' desired aliases.
- **`ReconfigureUnbound` is called at most once per cycle**, only when at least one create/delete
  succeeded — it is the expensive apply step. Keep it out of per-alias loops.
- **A single manual host override must pre-exist** in OPNsense Unbound; `opnsense.host_override` is
  its FQDN and the sync targets aliases *under* it. If it isn't found, the cycle errors out.

## Configuration system (`internal/config`)

- Precedence: built-in defaults < YAML file < `TOS_`-prefixed env vars.
- Env mapping: config dots → underscores (`traefik.base_url` → `TOS_TRAEFIK_BASE_URL`). List values
  accept CSV in env (`TOS_TRAEFIK_INCLUDE_ENTRYPOINTS="web,websecure"`).
- **Docker/secrets**: any `TOS_*_FILE` env var has its file contents read and injected as the
  corresponding `TOS_*` var (unless already set) — this is how secrets are passed in containers.
- Durations use Go strings (`"30s"`, `"1m"`). Decode hooks handle duration and CSV→slice.
- `validate()` enforces required fields and guardrails: `include_providers` and `ignore_providers`
  are mutually exclusive; `ignore_routers` entries must include a `@provider` suffix; `max_generated`
  and `interval` must be > 0. It warns (does not fail) when no Traefik filters are set (only when
  `sync.source` is `traefik`).
- `sync.source` (`"traefik"` default, or `"kubernetes"`) gates which section is required:
  `traefik.base_url` is only required for the traefik source; `kubernetes.resources` (must be a
  non-empty subset of `ingress`/`ingressroute`/`httproute`) and `kubernetes.ignore_resources`
  (`name.namespace@kind` entries) are only validated for the kubernetes source.
- When adding a config field: add it to the relevant `*Cfg` struct with a `mapstructure` tag, set a
  default in `setDefaults` if appropriate, add validation in `validate`, thread it through the
  consuming component (usually `syncer.NewRunner`/`newEngine`, or `traefik.NewSource`/
  `kubernetes.NewSource` for source-specific fields), and document it in `config.example.yml`.

## Docker build

Single image, `Dockerfile`: a `builder` stage does an in-container `go build` (ldflags-injected
version metadata, see above), then a `runtime` stage (distroless python base, bundling the `exrex`
shim so `HostRegexp` expansion works) copies in the compiled binary. There is no separate
"noregex" variant and no dependency on a prebuilt binary from an external build tool — the image
is fully self-contained.

## Releases

Hosted on a self-hosted Forgejo instance (`git.knifeinthesocket.com`) with Forgejo Actions
(`.forgejo/workflows/`). There is no release-please equivalent (it's GitHub-API-specific with no
Forgejo port), so versioning is manual and PR-driven:

- Every PR uses `.forgejo/PULL_REQUEST_TEMPLATE.md`, which has a `## Release` section with a
  `Version: vX.Y.Z` line (bumped by the author according to the change's semver impact) and a
  `## Changelog` section (bullet points for the release notes).
- Merging to `master` runs `.forgejo/workflows/build-push.yml`, which resolves the PR that
  introduced the merge commit, extracts `Version`/`Changelog` from its body, builds and pushes the
  Docker image (`docker.knifeinthesocket.com/influential-binary/opnsense-hosts-sync`, tagged both
  `:vX.Y.Z` and `:latest`), creates the matching git tag, and creates the Forgejo release with the
  extracted changelog as its notes. It comments the result back on the PR.
- The workflow fails the build if `Version` is missing from the PR body, or if the tag already
  exists — both are meant to catch a forgotten/duplicate version bump before it ships.

`.forgejo/workflows/test.yml` runs `gofmt`/`go vet`/`go build`/`go test` on every pull request.
