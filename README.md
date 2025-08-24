# ghops

> We are not related to GitHub in any way other than using their platform. All rights to the name "GitHub" are owned by GitHub, Inc.

![GitHub Tag](https://img.shields.io/github/v/tag/odit-services/ghops?style=for-the-badge&logo=git) ![GitHub Release Date](https://img.shields.io/github/release-date/odit-services/ghops?style=for-the-badge&label=Latest%20release) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/odit-services/ghops?style=for-the-badge&logo=go) ![GitHub License](https://img.shields.io/github/license/odit-services/ghops?style=for-the-badge) ![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/odit-services/ghops/lint.yml?style=for-the-badge&label=Checks)

Does the "GH" in "ghops" stand for GitHub? Maybe, maybe not. Maybe it stands for "ghost hops" 👻🍺 or "Git Happyness".

All we know is that this is a Kubernetes operator to manage resources on GitHub.

## Description

### Supported resources

- `DeployKey`: A GitHub deploy key for a repository.

## Deploy the operator

### Prerequisites

- `kubectl` version v1.11.3+ (or a reasonably recent client compatible with your cluster).
- Access to a Kubernetes v1.11.3+ cluster.

### Create the GitHub credentials secret

This operator expects a Kubernetes secret that provides a GitHub access token used to call the GitHub API.

1. Create a GitHub fine-grained personal access token with the following repository permissions (scope as needed for the target repositories/organization):
   - Repository: Read metadata
   - Repository: Read and write administration

2. Create the Kubernetes secret in the namespace where the operator expects it (example: `ghops-system`):

```sh
kubectl create namespace ghops-system || true
kubectl create secret generic -n ghops-system ghops --from-literal=GITHUB_TOKEN=<your-github-token>
```

Note: If you deploy into a different namespace update the manifests or the operator configuration accordingly.

### Deploy the full operator

This repository contains manifests that install CRDs, RBAC and the controller.

```sh
# Deploy the latest version from the repository
kubectl apply -f https://raw.githubusercontent.com/odit-services/ghops/main/config/deployment/full.yaml

# Or deploy a specific tag
kubectl apply -f https://raw.githubusercontent.com/odit-services/ghops/<tag>/config/deployment/full.yaml
```

## Development

### Local prerequisites

- Go 1.25.0+
- Docker (for building images)
- `kubectl` and access to a Kubernetes cluster (kind / minikube / remote cluster)

### Build and push images

This project includes Make targets for building and publishing images.

Single-arch (build for the host architecture):

```sh
make docker-build docker-push IMG=ghcr.io/odit-services/ghops:tag
```

Multi-arch (build for linux/amd64 and linux/arm64):

```sh
make docker-build-multiarch IMG=ghcr.io/odit-services/ghops:tag
```

Important note about multi-arch builds and QEMU:
- If you are building multi-arch images on a host that is not amd64 (for example an Apple Silicon / arm64 host) and you request linux/amd64 images, Docker will use QEMU emulation. The Go runtime (and other native toolchains) can crash under incorrect or unregistered QEMU binfmt support and you'll see runtime panics like `fatal error: taggedPointerPack` during `go mod download`.

If you see such a panic in CI (or locally when building inside an emulated image), resolve it by either:

1. Registering QEMU/binfmt on the host (self-hosted runner) once with a privileged container:

```powershell
# Run on the runner host (PowerShell)
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
```

2. Building on a native amd64 runner instead of relying on emulation (for CI use `runs-on: ubuntu-latest` or another amd64 runner).

3. Building only for the host's architecture (remove the linux/amd64 platform when on arm64 hosts).

Either registering binfmt on the host or using a native builder will prevent the Go runtime emulation panic.

### Buildx and caching (CI)

The included GitHub Actions workflow uses `docker/setup-qemu-action` and `docker/setup-buildx-action` then `docker/build-push-action` with `platforms:` set to `linux/amd64,linux/arm64`. If you run this on a self-hosted runner you must ensure the runner host has QEMU/binfmt registered (see above) or run the workflow on an AMD64 runner.

### Run the operator locally (development mode)

To run the controller locally against a cluster (useful for debugging):

```sh
# Run the manager locally (points to local kubeconfig)
make run
```

This will build the binary and run the controller using the local kubeconfig context.

### Tests

- Unit tests (Go): `go test ./...`
- Integration / e2e: see `test/e2e` and the `Makefile` targets. E2E tests assume a Kubernetes test environment is available.

## Troubleshooting

- QEMU / `taggedPointerPack` runtime panic during `go mod download` while building an amd64 image on an arm64 host: register QEMU on the runner host or use a native amd64 runner (see the QEMU note above).
- Image pull / RBAC issues: ensure the operator service account has the correct RBAC permissions and that the image is published in a registry accessible to your cluster.

## Examples / Samples

Sample CRs are available under `config/samples/` — apply them to create `DeployKey` resources for testing.

```sh
kubectl apply -k config/samples/
```

## Contributing

Contributions are welcome. Typical workflow:

1. Fork the repository
2. Create a branch (e.g. `git checkout -b feat/some-feature`)
3. Implement and test your changes
4. Commit and push your branch
5. Open a pull request

Please include tests for new features or bug fixes and make sure linters pass (`make lint` / CI).
