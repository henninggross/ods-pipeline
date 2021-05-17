# ODS Pipeline

**IMPORTANT: This is EXPERIMENTAL ONLY. This may or may not become part of ODS one day.**

## Introduction

ODS provides CI/CD pipeline support based on OpenShift Pipelines. This repository contains everything that relates to it, such as Tekton tasks, container images, Go packages, services, documentation, ...

The "user perspective" of what the ODS pipeline is and how it works is described in https://github.com/michaelsauter/ods-pipeline/wiki. It is important to understand this before looking at this repository, which is the actual "plumbing".

## How is this repository organized?

The repo follows the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).

The most important pieces are:

* **build/package**: `Dockerfile`s for the various container images in use. These images back Tekton tasks or the webhook interceptor.
* **cmd**: Main executables. These are installed (in different combinations) into the contaier images.
* **deploy**: OpenShift resource definitions, such as `BuildConfig`/`ImageStream` or `ClusterTask` resources. The tasks typically make use of the images built via `build/package` and their `script` calls one or more executables built from the `cmd` folder.
* **docs**: Design and user documents
* **internal/interceptor**: Implementation of Tekton trigger interceptor - it creates and modifies the actual Tekton pipelines on the fly based on the config found in the repository triggering the webhook request.
* **pkg**: Packages shared by the various main executables and the interceptor. These packages are the public interface and may be used outside this repo (e.g. by custom tasks). Example of packages are `bitbucket` (a Bitbucket Server API v1.0 client), `sonar` (a SonarQube client exposing API endpoints, scanner CLI and report CLI in one unified interface), `nexus` (a Nexus client for uploading, downloading and searching for assets) and `config` (the ODS configuration specification).
* **test**: Test scripts and test data

## Details / Documentation

* [Goals and Non-Goals](/docs/goals-and-nongoals.adoc)
* [Architecture Decision Records](/docs/adr)

## Building the images locally

The following shell [script](./scripts/kind-with-registry.sh) will create a local docker registry alongside a Kubernetes cluster by Kind.

```cli
cd scripts
./kind-with-registry.sh
```

Next, build both the `ods-build-go` and `ods-sonar` docker images and push them to the local image registry.

At the root of the repo, run:

```cli
docker build -f build/package/Dockerfile.go-toolset -t localhost:5000/ods/ods-build-go:latest .

docker build -f build/package/Dockerfile.sonar -t localhost:5000/ods/ods-sonar:latest .

docker push localhost:5000/ods/ods-build-go:latest
docker push localhost:5000/ods/ods-sonar:latest
```

## Run the tests

```cli
cd scripts
./run-tekton-task.sh
```

