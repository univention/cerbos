# Univention notes

This file documents the changes in this fork and the GitLab pipeline.
The upstream `README.md` remains the reference for Cerbos itself.

## Workflow

- Mirror upstream Cerbos into our GitLab
- Downstream release branches: for a given upstream release tag `vX.Y.Z`,
  create a protected branch `vX.Y.Z-univention` **from the tag** (not from a
  moving branch), and apply our patch series on top.
- Patch series: our modifications are maintained as a clean, cherry-pickable
  series written to upstream's contribution standards, so that each patch is
  individually upstreamable and our delta vs. upstream stays trivially
  answerable.
- Build/test pipeline: on `*-univention` branches, run upstream's own Go build
  and test targets plus our additional tests, build a Cerbos container image,
  and publish it to our registry.
- Upstream update: detect a new upstream release tag that has no corresponding
  `*-univention` branch, create the branch from the tag, apply the patch
  series, and push it.

## Pipeline overview

The pipeline uses `univention/dev/nubus-for-k8s/common-ci` v1.71.0 for stages,
the `.tagged-release` version stamp,
and the buildkit container build with publishing to
`artifacts.software-univention.de`.
The local `workflow:` replaces the shared `nubus-workflow` so the fork can
release from protected `vX.Y.Z-nubusN` tags
and build its protected `*-univention` release branches.

| Event | Pipeline | Jobs |
| --- | --- | --- |
| Protected `vX.Y.Z-nubusN` tag | Release | `pre-semantic-release`, `build-cerbos`, `smoke-cerbos`, `container-build`, `push-image-external`, `container-sign-external`, `container-release-attestation`, `start-tests`, `test-cerbos`, `test-cerbos-passed` |
| Merge request | Merge request | `pre-semantic-release`, `build-cerbos`, `smoke-cerbos`, `container-build`, `push-image-external`, `container-sign-external`, `container-release-attestation`, `start-tests` (manual), `test-cerbos`, `ai-review:*` (manual) |
| Push to `main` | None | None |
| Push to a protected `*-univention` branch | Branch | `pre-semantic-release`, `build-cerbos`, `smoke-cerbos`, `container-build`, `push-image-external`, `container-sign-external`, `container-release-attestation`, `start-tests` (manual), `test-cerbos` |
| Feature branch without a merge request | None on push | None |
| `schedule` pipeline | Scheduled | Build, test, and container jobs are skipped |

`schedule-release` is also allowed by the workflow for common-ci compatibility,
but this fork does not configure a release schedule.

`main` mirrors upstream and is not pushed to directly.
A mirror update does not create a pipeline.

A feature branch push does not create a branch pipeline.
This includes a `*-univention` branch that is not protected.
The branch is tested through its merge request pipeline.
A manually started web pipeline can still run on a feature branch.

## Versions and image tags

`pre-semantic-release` extends common-ci's `.tagged-release`
and writes `RELEASE_VERSION` and `PUBLIC_RELEASE` as a dotenv artifact.
`build-cerbos` stamps `RELEASE_VERSION` into the binary,
and the container jobs tag the image with it.

| Pipeline | `RELEASE_VERSION` |
| --- | --- |
| Protected release tag `vX.Y.Z-nubusN` | `X.Y.Z-nubusN` |
| Any other ref | `<base>-post-<ref-slug>` |

`<base>` is the newest `vX.Y.Z-nubusN` tag without the `v`,
or `INITIAL_VERSION` (`0.57.0`) before the first release tag exists.

On a release tag, `pre-semantic-release` fails
unless the upstream tag `vX.Y.Z` is an ancestor of the tagged commit.

`container-build` pushes to the project container registry
with the tag `RELEASE_VERSION`,
and on a non-tag ref also with the ref slug:

```text
$CI_REGISTRY_IMAGE/cerbos:<tag>
```

`push-image-external` copies the image to
`artifacts.software-univention.de/nubus-dev/images/cerbos:<RELEASE_VERSION>`,
and on a release tag also to `nubus/images/cerbos`.
`container-sign-external` signs the public image,
and `container-release-attestation` attests both copies.
These jobs need the `EXTERNAL_REGISTRY_ROBOT_TOKEN`
and `COSIGN_*` CI/CD variables.

| Pipeline | Image tags |
| --- | --- |
| Merge request | `0.57.0-post-<slug>`, `<slug>` |
| Push to a protected `*-univention` branch | `0.57.0-post-<slug>`, `<slug>`, for example `0.57.0-post-v0-57-0-univention` |
| Protected release tag | `X.Y.Z-nubusN`, for example `0.57.0-nubus1` |

## Release process

1. Create a protected tag matching `vX.Y.Z-nubusN`.
2. The tag pipeline runs `build-cerbos` and then `smoke-cerbos`.
3. `container-build` runs after both jobs succeed and pushes the image.
   The publish jobs copy it to `nubus-dev` and `nubus`,
   then sign and attest it.
   `test-cerbos` runs automatically on a release tag,
   and `push-image-external` waits for it,
   so a failing test stops the release.
4. CI does not create a Git tag or a GitLab Release.
   The tag is created manually.
5. `latest` is not published.
   Pushes to `main` run no pipeline,
   and the common-ci release detection only matches plain `X.Y.Z` versions.

## Build job

`build-cerbos` uses `$GO_IMAGE`, which defaults to `golang:1.27` from the
dependency proxy.
It runs a direct `go build` with the same settings as the `cerbos` build in
`.goreleaser.yml`:

- `CGO_ENABLED=0` and `GOAMD64=v1`
- `-trimpath`
- `-s -w`
- `Version`, `Commit`, and `BuildDate` linker flags

`-buildvcs=false` is specific to CI and keeps the build independent of the Git
checkout.
The telemetry linker flags from `.goreleaser.yml` are not set.
The binary is written to `docker/cerbos/cerbos` and uploaded as a job artifact.

## Test job

`test-cerbos` runs the tests behind the `tests` build tag with the race
detector.
It installs `testsplit`, applies `.ignore-packages.yaml`, and excludes
`internal/storage/blob` and `internal/storage/overlay`.
Those two packages start SeaweedFS through `ory/dockertest`, so they need
Docker.
The job does not use dind.
Tests behind the `integration` tag are not run.
The tests start when the `start-tests` job runs,
which starts all four parts at once.
On a protected release tag, `start-tests` runs automatically,
and `push-image-external` waits for all four parts to pass.
In every other pipeline, `start-tests` is manual and allowed to fail,
so unstarted tests block neither the pipeline nor publishing.

The job runs as four parallel jobs, `test-cerbos 1/4` to `test-cerbos 4/4`,
like upstream's six-way split in `.github/workflows/pr-test.yaml`.
`testsplit` assigns each job a share of the packages
from `CI_NODE_INDEX` and `CI_NODE_TOTAL`.
It balances the jobs with the package durations in `ci/testsplit-times.json`,
the mean `go test` time per package over pipelines 467991, 468002 and 469668.
Packages missing from the file count as one second.
Regenerate the file when the split becomes uneven.
Each job keeps its own build cache.
`go test` runs with `-timeout=30m` per package,
because `internal/parser` and `internal/server` take more than
Go's default of 10 minutes with the race detector.

## Smoke job

`smoke-cerbos` tests the binary from `build-cerbos`,
where `test-cerbos` only tests the sources.
It uses `$GO_IMAGE` and does the following:

1. `cerbos compile internal/test/testdata/store` compiles the test policies
   and runs their `*_test.yaml` suites.
2. `cerbos server` starts with `hack/dev/conf.insecure.yaml`.
3. `cerbos healthcheck --kind=http` polls the server once per second
   and fails the job if it is not healthy after 30 attempts.

The server log is kept as the `cerbos.log` artifact, also when the job fails.
The job does not run in scheduled pipelines.

## Container image

`container-build` uses the common-ci buildkit template.
The build context is `docker/cerbos` and the Dockerfile is
`docker/cerbos/Dockerfile`.
The image is `$CI_REGISTRY_IMAGE/cerbos`.
The job runs after `build-cerbos` and `smoke-cerbos` succeed.
It does not wait for `test-cerbos`.

The local Dockerfile is a small adaptation of the upstream `Dockerfile.cerbos`.
The upstream file expects goreleaser's `${TARGETPLATFORM}/cerbos` build context
and ignores the `DOCKER_PROXY` build argument.
The local file copies `cerbos` from the context and uses the proxy for the
Alpine certificates stage when the dependency proxy is configured.

## Running the checks locally

Build the binary with the same flags as CI:

```sh
VERSION="$(git describe --tags --exact-match 2>/dev/null || git rev-parse --short=8 HEAD)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
CGO_ENABLED=0 GOAMD64=v1 go build -trimpath -buildvcs=false \
  -ldflags "-s -w \
    -X github.com/cerbos/cerbos/internal/util.Version=${VERSION} \
    -X github.com/cerbos/cerbos/internal/util.Commit=$(git rev-parse HEAD) \
    -X github.com/cerbos/cerbos/internal/util.BuildDate=${BUILD_DATE}" \
  -o docker/cerbos/cerbos ./cmd/cerbos
```

Run the same test selection:

```sh
GOWORK=off go install -C hack/tools/testsplit
"$(go env GOPATH)/bin/testsplit" split --kind=tests --index=0 --total=1 --ignore-file=.ignore-packages.yaml \
  | grep -v -E '^github.com/cerbos/cerbos/internal/storage/(blob|overlay|git|db|db/postgres)$' \
  | xargs -r go test -tags=tests -race -failfast -count=1 -p 4 -timeout=30m
```

Run the smoke checks against the built binary:

```sh
./docker/cerbos/cerbos compile internal/test/testdata/store
./docker/cerbos/cerbos server --config=hack/dev/conf.insecure.yaml &
sleep 2
./docker/cerbos/cerbos healthcheck --config=hack/dev/conf.insecure.yaml --kind=http
kill %1
```

## Files

- `.gitlab-ci.yml`: pipeline workflow, jobs, and includes.
- `docker/cerbos/Dockerfile`: runtime image for the CI build.
- `ci/testsplit-times.json`: package test durations for the `test-cerbos` split.

## Notes and limits

- The test job does not run the Docker-backed blob and overlay storage tests.
- Integration tests are not run.
- `test-cerbos` is manual except on release tags.
  It gates publishing only on release tags,
  so MR and branch images reach `nubus-dev` even when the Go tests fail.
  A failing `smoke-cerbos` does stop the image.
- The CI binary is built with `-buildvcs=false`, so it has no VCS stamping.
