# AGENTS.md

## Project Overview

A CLI tool (`unicli`) for querying the status of infrastructure services represented by Kubernetes custom resources. It is used by humans to inspect, create, and connect to resources such as Kubernetes clusters, virtual Kubernetes clusters, compute instances, OpenStack identities, SSH keys, and organizations.

## Language and Style

- Written in Go. All code must be idiomatic Go.

## Project Structure

- `cmd/unicli/` - CLI entrypoint (`main.go`, `command.go`, shell completions)
- `pkg/` - Core logic, organized by CLI verb:
  - `get/` - List/get resources (clustermanager, computeinstance, kubernetescluster, openstackidentity, sshkey, user, virtualkubernetescluster)
  - `describe/` - Detailed resource views (clustermanager, computeinstance, kubernetescluster, openstackidentity, virtualkubernetescluster)
  - `create/` - Resource creation (group, organization, user)
  - `connect/` - Connect to resources (clustermanager)
  - `flags/` - Shared CLI flag definitions
  - `factory/` - Shared construction/dependency wiring
  - `errors/` - Error types
  - `util/` - Shared utilities

## Building

Use the Makefile:

```sh
make          # Build for host architecture
make RELEASE=1  # Cross-compile for all release targets (amd64-linux, arm64-linux, arm64-darwin)
make lint     # Run golangci-lint
```

Binaries are output to `bin/<arch>-<os>/unicli`.

Version and git revision are injected at link time via `pkg/constants`.

## Commits

Follow the [Conventional Commits](https://www.conventionalcommits.org/) convention:

```
<type>(<scope>): <description>
```

Types: `feat`, `fix`, `refactor`, `docs`, `chore`, `test`, `build`, `ci`.

Examples:
- `feat(compute): add get and describe for compute instances`
- `fix(describe): handle nil networking in cluster detail view`
- `refactor(factory): extract shared client construction`
- `docs: update AGENTS.md with new resources`
