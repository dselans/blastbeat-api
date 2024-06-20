go-svc-template
===============

⚡ Batteries-included Golang microservice template ⚡️

_Last updated: 06/18/2024_

**It includes:**

1. `Makefile` that is used for run, test, build, deploy actions
1. `Dockerfile` for building a Docker image (`alpine` with multi-stage build)
1. `docker-compose.yml` for local dev
1. Github workflows for [PR](.github/workflows/go-svc-template-pr.yml) and 
[release](.github/workflows/go-svc-template-release.yml) automation
1. Sane code layout [1]
1. Structured logging
1. Good health-checking practices (uses async health-checking)
1. Sample [kubernetes deploy configs](deploy.stage.old.yml)
1. Configurable profiling support (pprof)
1. Pre-instrumented with [New Relic APM](https://newrelic.com)
1. Supports AWS EKS and ECR

**It uses:**

1. `Go 1.22`
1. `julienschmidt/httprouter` for the HTTP router
1. `uber/zap` for structured, light-weight logging
1. `alecthomas/kong` for CLI args + ENV parsing
1. `newrelic/go-agent` for APM (with logging)
1. `streamdal/rabbit` for reliable RabbitMQ
1. `onsi/ginkgo` and `onsi/gomega` for BDD-style testing

<sub>[1] `main.go` for entrypoint, `deps/deps.go` for dependency setup + simple
dependency injection in tests, `backends` and `services` abstraction for business
logic.</sub>

## Makefile
All actions are performed via `make` - run `make help` to see list of available make args (targets).

For example:

* To run the service, run `make run`
* To build + push a docker img, run `make docker/build`
* To deploy to staging, run `make deploy/stg`
* To deploy to production, run `make deploy/prd`

## Secrets

Secrets are managed via [Doppler](https://doppler.com/) -- a SaaS secrets management
solution.

<TODO>: Add details

## Logging

This service uses a custom logger that wraps `uber/zap` in order to provide a
structured logging interface. While NR is able to collect logs written via `uber/zap`,
it does not include any "initial fields" set on the logger.

This makes it very difficult to create temporary loggers with base values that
are re-used throughout a method. For example: In method `A` that is 100 lines
long, we may want to create a logger with a base field "method" set to "A".

That would allow us to use the same logger throughout the method and not have
to always include "method=A" attributes in each log message - the field will be
included automatically.

The custom log wrapper provides this functionality.

## PR and Release

PR and release automation is done via GitHub Actions.

When a PR is opened, a [PR workflow](.github/workflows/go-svc-template-pr.yml)
is triggered.

When a PR is merged, a [Release workflow](.github/workflows/go-svc-template-release.yml)
is triggered. This workflow will build a docker image and push it to AWS ECR.

## Deployment

Deployment is _manual_. This is done for one primary reason:

**A deployment is a critical operation that should be handled with care.**

_Or in other words, we do not throw deployments over the wall. Just because we
can automate them, does not mean we should or will._

Deployments are performed via `make deploy/stg` and `make deploy/prd`.

> [!IMPORTANT]  
> The image the deployment will use is the _CURRENT_ short git sha in the repo!

---

## Template Usage

1. Click "Use this template" in Github to create a new repo
1. Clone newly created repo
1. Find & replace:
   1. `go-svc-template` -> lower case, dash separated service name
   2. `GO_SVC_TEMPLATE` -> upper case, underscore separated service name (for ENV vars)
   3. `your_org` -> your Github org name
    ```bash
    find . -maxdepth 3 -type f -exec sed -i "" 's/go-svc-template/service-name/g' {} \;
    find . -maxdepth 3 -type f -exec sed -i "" 's/GO_SVC_TEMPLATE/SERVICE_NAME/g' {} \;
    find . -maxdepth 3 -type f -exec sed -i "" 's/your_org/your-org-name/g' {} \;
    mv .github.rename .github
   ```

## Vendor

This template vendors packages by default to ensure reproducible builds + allow
local dev without an internet connection. Vendor can introduce its own headaches
though - if you want to remove it, remove `-mod=vendor` in the [`Makefile`](Makefile).
