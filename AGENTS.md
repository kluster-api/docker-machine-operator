# AGENTS.md

This file provides guidance to coding agents (e.g. Claude Code, claude.ai/code) when working with code in this repository.

## Repository purpose

Go module `go.klusters.dev/docker-machine-operator` — a Kubernetes operator that wraps [Docker Machine](https://github.com/cluster-api/docker-machine) under two CRDs:

- `Driver` (`docker-machine.klusters.dev/v1alpha1`) — declares which `docker-machine` driver binary to use (e.g. `amazonec2`, `google`). Can be `Builtin` or downloaded from a `DownloadURL`.
- `Machine` (`docker-machine.klusters.dev/v1alpha1`) — describes a VM to be created via the referenced driver, complete with cloud-provider-specific spec fields.

The controller calls out to the actual `docker-machine` CLI to provision/destroy VMs on AWS, Azure, or GCP. The produced binary is `docker-machine-operator`.

The git remote is `cluster-api/docker-machine-operator` (note: `cluster-api`, **not** `kluster-api` like the sibling repos); the module name is the source of truth.

## Architecture

- `cmd/docker-machine-operator/` — entry point.
- `pkg/cmds/` — Cobra command tree:
  - `root.go`, `run.go` — top-level + long-running command.
  - `server/` — `RecommendedOptions` and start glue for the controller manager.
- `api/v1alpha1/` — Kubebuilder API types (`driver_types.go`, `machine_types.go`, plus `helper.go` / `machine_helper.go` accessors and `zz_generated.deepcopy.go`).
- `pkg/controller/` — reconcilers and cloud helpers:
  - `driver_controller.go` — reconciles `Driver` (resolves/downloads the docker-machine driver binary).
  - `machine_controller.go` — reconciles `Machine` (calls docker-machine CLI to create/destroy VMs).
  - `machine.go` — shared docker-machine invocation plumbing.
  - `cluster.go` — bookkeeping for the parent cluster.
  - `aws.go`, `aws_region.go`, `azure.go` — per-cloud branches called from the machine reconciler.
  - `util.go`, `suite_test.go` — helpers and envtest suite.
- `pkg/script/ami.sh` — embedded provisioning script.
- `config/samples/` — example CR manifests for `kubectl apply`.
- `examples/` — `aws-driver.yaml`, `aws-machine.yaml`, `aws-secret.yaml`, `gcp-driver.yaml`, `gcp-machine.yaml` — end-to-end example manifests.
- `crds/` — generated CRD YAMLs (`docker-machine.klusters.dev_drivers.yaml`, `docker-machine.klusters.dev_machines.yaml`).
- `PROJECT` — Kubebuilder project metadata. Domain is `klusters.dev`. Do not hand-edit unless also running `kubebuilder edit`.
- `Dockerfile.in` (PROD) + `Dockerfile.dbg` (debian) — two image variants (no UBI for this one).
- `hack/`, `Makefile` — AppsCode build harness (runs everything inside `ghcr.io/appscode/golang-dev`).
- `vendor/` — checked-in deps.

API group/version: `API_GROUPS := apps:v1alpha1` in the Makefile (the actual CRDs live under group `docker-machine.klusters.dev`; the Makefile variable's `apps:v1alpha1` is the codegen alias).

## Common commands

All Make targets run inside `ghcr.io/appscode/golang-dev` — Docker must be running.

- `make build` / `make all-build` — build host or all-platform binaries.
- `make gen` — regenerate clientset + manifests. Run after any change to `api/v1alpha1/*_types.go`.
- `make manifests` — regenerate CRDs only.
- `make clientset` — regenerate client code.
- `make fmt` — gofmt + goimports.
- `make lint` — golangci-lint.
- `make unit-tests` — Go unit tests.
- `make e2e-tests` / `make test` — runs both unit and e2e (Ginkgo envtest suite under `pkg/controller/suite_test.go`).
- `make verify` — `verify-gen verify-modules`; `go mod tidy && go mod vendor` must leave the tree clean.
- `make container` — build PROD and DBG images.
- `make push` — push both; `make docker-manifest` writes multi-arch manifests; `make release` is the full publish flow.
- `make push-to-kind` / `make deploy-to-kind` — load into Kind and Helm-install.
- `make install` / `make uninstall` / `make purge` — Helm install lifecycle.
- `make add-license` / `make check-license` — manage license headers.

Run a single Go test (requires a local Go toolchain):

```
go test ./pkg/controller/... -run TestName -v
```

## Conventions

- Module path is `go.klusters.dev/docker-machine-operator` (vanity URL); imports must use that, not the GitHub URL (`cluster-api/docker-machine-operator`). The repo's GitHub org `cluster-api` is **different** from the sister org `kluster-api`; do not confuse the two.
- License header in `api/v1alpha1/` is **Apache-2.0** (Kubebuilder scaffold default). When adding new files, match the surrounding package's existing header.
- Sign off commits (`git commit -s`); contributions follow the DCO (`DCO` file).
- Vendor directory is checked in — `go mod tidy && go mod vendor` must leave the tree clean (enforced by `verify-modules`).
- Per-cloud code lives strictly in `pkg/controller/{aws,aws_region,azure}.go`. New providers should follow the same pattern; do not sprinkle cloud-specific branches across `machine_controller.go`.
- Do not hand-edit `zz_generated.*.go` or anything under `crds/` — change `api/v1alpha1/*_types.go` and re-run `make gen`.
- Two Dockerfiles, one binary — no UBI variant. Keep `Dockerfile.in` and `Dockerfile.dbg` in sync when changing build steps.
- This is a **Kubebuilder project** (`PROJECT` file present). Use `kubebuilder` to scaffold new APIs/controllers; don't hand-create files that `PROJECT` should track.
