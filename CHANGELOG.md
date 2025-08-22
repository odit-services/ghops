# Changelog

All notable changes to this project will be documented in this file.
Versions are based on [Semantic Versioning](http://semver.org/), and the changelog is generated with [Chglog](https://github.com/git-chglog/git-chglog).

## Version History

* [v0.3.1](#v0.3.1)
* [v0.3.0](#v0.3.0)
* [v0.2.1](#v0.2.1)
* [v0.2.0](#v0.2.0)
* [v0.1.0](#v0.1.0)

## Changes

<a name="v0.3.1"></a>
### [v0.3.1](https://github.com/odit-services/s3ops/compare/v0.3.0...v0.3.1)

> 2025-08-22

#### 🏡 Chore

* update changelog
* **deploy:** update deployment manifests

#### 🚀 Enhancements

* **release:** add GitHub Actions workflow for automated release generation

#### 🩹 Fixes

* **deploykey:** handle error when deleting secret after GitHub key creation failure


<a name="v0.3.0"></a>
### [v0.3.0](https://github.com/odit-services/s3ops/compare/v0.2.1...v0.3.0)

> 2025-08-22

#### 🏡 Chore

* update changelog
* **deploy:** update deployment manifests

#### 🩹 Fixes

* **deploykey:** delete secret on error during deploy key creation
* **deploykey:** swap public and private key return values in key pair generation
* **kustomization:** change file permissions from executable to read-only
* **sshservice:** swap return values for RSA key pair generation


<a name="v0.2.1"></a>
### [v0.2.1](https://github.com/odit-services/s3ops/compare/v0.2.0...v0.2.1)

> 2025-08-22

#### 🏡 Chore

* update changelog
* **deploy:** update deployment manifests
* **kustomization:** change file permissions from 755 to 644

#### 🚀 Enhancements

* **deploykey:** implement retry logic and max retries for DeployKey reconciliation
* **deploykey:** handle failed state and requeue logic in DeployKey reconciler


<a name="v0.2.0"></a>
### [v0.2.0](https://github.com/odit-services/s3ops/compare/v0.1.0...v0.2.0)

> 2025-08-22

#### 🏡 Chore

* update changelog
* **deploy:** update deployment manifests
* **kustomization:** change file permissions from 755 to 644

#### 🚀 Enhancements

* **deploykey:** add sample deploy key fields for metadata
* **dockerfile:** add source label for image metadata
* **makefile:** add multiarch docker build target


<a name="v0.1.0"></a>
### v0.1.0

> 2025-08-22

#### 🏡 Chore

* Remove before deploy
* generated stuff
* **ci:** Disable build for now b/c gha has some docker qemu problems
* **deploy:** update deployment manifests
* **deploy:** update deployment manifests
* **deploy:** update deployment manifests
* **deploy:** update deployment manifests
* **dev:** Removed unused resources
* **docker:** Bump image
* **go.mod:** update Go version to 1.25.0
* **workflows:** remove obsolete .test.yml workflow file
* **workflows:** replace test.yml with .test.yml for workflow organization

#### 📖 Documentation

* Update README with secret creation instructions and kubectl command
* Added basic docs

#### 🚀 Enhancements

* Implemented deploy key generation
* Baseline for deploy keys
* **deploykey:** Suport multiple ssh key types
* **dev:** Implemented github actions

#### 🩹 Fixes

* **build:** add QEMU setup step to the build workflow
* **build:** correct docker build context to current directory
* **build:** ensure Docker image tags and labels are correctly set
* **deploy:** correct path in build-yaml target for deployment manifests
* **lint:** update golangci-lint version to v2.4.0
* **makefile:** update GITCHGLOG variable assignment and add comment for multiarch build
* **manager:** Add environment variable reference for GitHub token secret

#### 💅 Refactors

* Split repo into owner and repo
* **dev:** Let gha build stuff for us

#### 🎨 Styles

* Lint run

