# Contributing

## Setup

Tool versions (Go, AWS CLI, SAM CLI, mockery, golangci-lint) are pinned via [mise](https://mise.jdx.dev/):

```sh
mise install
mise activate  # add to your shell profile so `go`, `sam`, `aws`, etc. resolve directly
```

Deploying uses two logins: an AWS one (see [AWS accounts](#aws-accounts)), and a Cloudflare one for DNS, `mise run cf-login` once. The Cloudflare login can be narrowed to zone read and DNS edit on the consent page.

`sam build` requires Docker running locally. On macOS with Docker Desktop, make sure `/Applications/Docker.app/Contents/Resources/bin` is on your `PATH` — otherwise `sam build`/`docker push` fail with a credential-store error.

## Everyday commands

```sh
mise run lint             # golangci-lint + actionlint + shellcheck
mise run test             # unit tests
mise run build            # sam build
mise run gen              # regenerate mocks, OpenAPI types, API Gateway routes, and go.mod/go.sum after changing an interface, api/openapi.yaml or brand/
mise run check-generated  # run gen and fail if it changed anything (what CI runs)
```

Single-test invocations use the underlying tools directly:

```sh
go test ./... -run TestVerifyTokenSuite -v   # a single suite
go test ./internal/auth/... -v               # a single package
sam validate --lint
```

## Running the app locally

```sh
mise run func-setup    # deploys template.yaml to floci + generates local OIDC signing material
# export the MYKEEBS_* vars func-setup prints, then:
mise run func-test     # runs the functional test suite against it
mise run func-teardown # tears it all down
```

`func-setup` deploys mykeebs-api's real `template.yaml` to [floci](https://github.com/floci-io/floci) (a local AWS emulator) via a genuine `sam deploy` — not `sam local start-api`, which never emulates API Gateway's native JWT authorizer, so it couldn't exercise any auth-required write route (see `internal/middleware.RequireAuthorizerIdentity`). It also runs [`oidc-testkit-gen`](https://github.com/roguekeyboards/oidc-testkit) to produce the signing key + discovery doc + JWKS. All `MYKEEBS_*` values land in `.env.floci`, which `func-test` sources automatically.

Most iteration just needs `mise run func-test` again — Go code and test changes are picked up automatically. Restart (`mise run func-teardown && mise run func-setup`) after editing `template.yaml` or `docker-compose.floci.yml`.

## Backing up / moving a user's data (`mykeebs-migrate`)

`cmd/mykeebs-migrate` is a standalone CLI that dumps a user's profile and every entity they own (keyboards, switches, keycap sets, builds) plus every S3 image to a local directory, and restores such a dump into another environment through the same public REST API. Use it to take a verifiable backup before a data-model change, to move a user's data between environments, or to export someone's account for a data request (see [Exporting another user's account](#exporting-another-users-account)). It uses **no AWS credentials** — image bytes move over the URLs the API signs — and never touches lookups (they're a static catalog built into the API; the dump captures `lookups/lookups.json` for diffing only).

```sh
mise run migrate-build          # -> bin/mykeebs-migrate

# 1. Get a token for the SOURCE environment (opens a browser to sign in with Discord).
bin/mykeebs-migrate login --issuer https://auth.jay.mykeebs.dev

# 2. Dump.
bin/mykeebs-migrate dump --issuer https://auth.jay.mykeebs.dev --base-url https://jay.api.mykeebs.dev --out ./dump

# 3. Get a token for the TARGET (may be a different account/environment), then restore.
bin/mykeebs-migrate login --issuer <target issuer>
bin/mykeebs-migrate restore --issuer <target issuer> --base-url <target base url> --in ./dump

# 4. Check the restore against the dump.
bin/mykeebs-migrate verify --issuer <target issuer> --base-url <target base url> --in ./dump
```

`login` runs a standard OAuth 2.0 authorization-code + PKCE flow and binds a **fixed** localhost port (`8765`) for the redirect, because IdP redirect URIs are exact-match. That redirect, `http://localhost:8765/authorize.html`, is registered on the OAuth client `login` creates through dynamic client registration, which both mykeebs-api Stytch projects (dev and Live) allow, so no client ID or project redirect URL is needed. An IdP without dynamic registration needs `--client-id` (or `MYKEEBS_OIDC_CLIENT_ID`). The token is cached under `~/.config/mykeebs-migrate/` keyed by issuer host; `dump`/`restore`/`verify` find that cached token only through `--issuer` (or `MYKEEBS_OIDC_ISSUER_BASE_URL`), or take one directly with `--token` / `MYKEEBS_AUTH_TOKEN`.

**Moving data between two accounts on the same issuer** (step 1 and step 3 are different people): the second `login` opens the browser with the first account's IdP session still live, so it lands on the consent screen already "Signed in" as the wrong account. Click **"Not you? Use a different account"** on that page to drop the session and sign in as the target account.

`dump` converts JPEG and PNG images to WebP and scales any image over 2560px down, the way mykeebs-web prepares an upload (lossy quality 82, EXIF orientation applied), so whatever an environment holds, a restore leaves only WebP behind. WebPs within 2560px are kept as they are. Any image the API would refuse on restore (another format such as GIF, an animated image, or one still over the API's size cap after conversion) fails the dump rather than leaving a dump that can't be restored.

Restore always **creates new** items (new server-generated IDs), recording an old→new `id-map.json` in the dump directory; builds are restored last with their keyboard/switch/keycap-kit references remapped through that map, and a re-run resumes from wherever a failure stopped.

The **profile** is restored first. If the target account has no profile, restore creates one with the dumped username, and stops before writing anything if another account already has that username. If the target already has a profile, restore replaces its fields, and its avatar if the dump has one, but keeps its username. `verify` compares everything but the username.

### Exporting another user's account

`dump --impersonate` dumps the account you open with the Stytch dashboard's **Impersonate user**, for a user's request for a copy of their data. It needs no login and no Stytch secret: the dashboard sends a one-time token to `http://localhost:8765/impersonate`, where `mykeebs-migrate` serves a page that swaps it for a session using Stytch's browser SDK (loaded from jsDelivr).

```sh
bin/mykeebs-migrate dump --impersonate --issuer https://auth.mykeebs.info --base-url https://api.mykeebs.info \
  --public-token <IdpConsentPublicToken from samconfig.toml> --out ./export-<user>
```

Then, in the Stytch dashboard: turn on **Allow user impersonation** (Password and User Policies), open the user, click **Impersonate user**, give a reason, and click **Launch in new tab**. The tab says when it's done, and the dump runs in the terminal. Turn impersonation off again afterwards; leave it off between exports. Stytch logs every impersonation with its reason.

Each Stytch project needs, once: `http://localhost:8765/impersonate` as the **default Login** redirect URL, `http://localhost:8765` as an authorized SDK domain, and **Authenticate user impersonation tokens** on in the Frontend SDK configuration. Both mykeebs-api projects (dev and Live) have them. The session JWT lasts 5 minutes, which is plenty for one account; a dump that runs longer fails with a 401.

A data-request export also needs what Stytch holds for the user (their Discord name, avatar and email), from the user's page in the dashboard.

## Reviewing moderation decisions (`mykeebs-moderation`)

There's no admin UI for image moderation. `cmd/mykeebs-moderation`, run through the `mise run moderation-*` tasks with an admin AWS profile, works the review queue, takes down reported images and undoes either decision. See [`docs/moderation.md`](docs/moderation.md) for how to use it.

## Deploying to AWS

Each environment is a section in `samconfig.toml` — stack name, artifact bucket, ECR repo, region and every template parameter — deployed with three tasks:

```sh
mise run setup [env]     # one-time: creates the env's ECR repo stack, then deploys
mise run deploy [env]    # deploy your current code to it
mise run teardown [env]  # delete the stack, the ECR repo stack and the repo itself
```

`env` defaults to `dev-$(whoami)`. Developers each get their own isolated stack, so nobody can break anyone else's testing. To add yours, copy the `[dev-jay.deploy.parameters]` section to `[dev-<your-name>.deploy.parameters]` and change its values: `stack_name` must be `mykeebs-api-dev-<your-name>`, and the ECR repo in `image_repositories` must be `mykeebs-api-<stack_name>`. None of the values are secret; commit the section. Want a second stack? Add a second section, e.g. `dev-<your-name>-test`.

Put every template parameter in the section's `parameter_overrides`. Passing `--parameter-overrides` on the command line replaces that list wholesale rather than merging with it, so the scripts never do.

Before changing anything, each task checks that your active AWS credentials belong to the account in the section's `s3_bucket` (`mykeebs-sam-artifacts-<account-id>`) and stops if not. `teardown` refuses `prod` outright.

**Prod** is deployed only by GitHub Actions, from releases; don't run `deploy prod` by hand. See the [prod runbook](docs/runbook-prod.md).

You just need an active, authenticated AWS session — any profile works, there's no required profile name. This project's own maintainer setup happens to use a profile named `AWS_PROFILE=mykeebs-api-dev-admin` (see [AWS accounts](#aws-accounts) below), but that's just this project's convention, not a requirement. **If you're forking this repo**, use any profile authenticated to your own AWS account with admin access.

### IdP OIDC config (`OidcIssuerBaseUrl`/`OidcAudience`/`IdpConsentPublicToken`)

mykeebs-api is IdP-agnostic (see [Identity provider requirements](README.md#identity-provider-requirements)); this project is currently run against a Stytch **Test** project, and the parenthetical guidance below reflects that.

- `OidcIssuerBaseUrl` — your IdP project's OIDC issuer base URL (for a Stytch Test project: `https://test.stytch.com/v1/public/{project_id}`).
- `OidcAudience` — the `aud` claim on every access token, REST and MCP alike: typically your IdP project ID. Some IdPs (Stytch included) put the project ID in `aud` regardless of client type, so one value covers both flows — no separate MCP audience needed.
- `IdpConsentPublicToken` — your IdP's browser-SDK public token (for Stytch: dashboard → your project → API keys → Public token). Rendered client-side into the `GET /authorize` consent page to construct the IdP SDK client - safe to embed client-side by design, but still varies per stack.

Test is for dev/personal stacks only — never point a dev stack at a live/production IdP project.

### CORS (`CorsAllowOrigins`)

A comma-separated list of browser origins (each scheme + host + port) allowed to call your stack's `HttpApi` cross-origin, e.g. `http://localhost:5173,https://jay.mykeebs.dev` to cover both a local frontend dev server and its deployed counterpart. Without this, browser preflight (`OPTIONS`) requests 404 before the authorizer is ever reached — API Gateway auto-generates CORS `OPTIONS` routes and exempts them from the JWT authorizer only when `HttpApi.Properties.CorsConfiguration` is set.

### Custom domains (`<your-name>.api.mykeebs.dev`, `<your-name>.images.mykeebs.dev`)

Optional. They give your stack stable domains instead of its default `execute-api.amazonaws.com` and `*.cloudfront.net` ones, which a web app's CSP can name. There's nothing to request per developer: two wildcard certificates in the `mykeebs-api-dev` account cover everyone. `*.api.mykeebs.dev` is in `us-east-2`, for API Gateway. `*.images.mykeebs.dev` is in `us-east-1`, because CloudFront only takes certificates from there. Their ARNs are in `[dev-jay.deploy.parameters]`.

1. **Add `CustomDomainName=<your-name>.api.mykeebs.dev`, `ImagesDomainName=<your-name>.images.mykeebs.dev` and the two certificate ARNs** to your section's `parameter_overrides`.
2. **Run `mise run setup`** (or `mise run dns` if the stack already exists). After deploying, it creates the DNS-only `<your-name>.api` and `<your-name>.images` CNAMEs in the `mykeebs.dev` Cloudflare zone, pointed at the stack's `ApiCustomDomainTarget` and `ImagesDomainTarget` outputs. It needs a Cloudflare login: `mise run cf-login` once (see [Setup](#setup)).

DNS for `mykeebs.dev` is on Cloudflare, not Route53. The wildcard certificates were validated once with CNAMEs there, and renew on their own as long as those records stay. The zone's CAA records include `issuewild "amazon.com"`, without which ACM can't issue them.

### AWS accounts

Four accounts, one SSO session (`aws configure sso` once, then `aws sso login --sso-session mykeebs-api` to refresh):

| Profile | Account | Purpose |
|---|---|---|
| `mgmt-admin` | `roguekeyboards` | Administrative only, no workloads |
| `mykeebs-api-dev-admin` | `mykeebs-api-dev` | Hosts each developer's own stack |
| `mykeebs-api-ci-admin` | `mykeebs-api-ci` | CI's per-PR stacks |
| `mykeebs-api-prod-admin` | `mykeebs-api-prod` | Production (`api.mykeebs.info`) |

There's no default AWS profile — commands will fail with `NoCredentials` unless `AWS_PROFILE` is set.

The organization root has an AWS AI services opt-out policy, `ai-opt-out`, attached by hand from `mgmt-admin` rather than through `bootstrap/stacks.yaml`, and every account inherits it. Don't detach it: without it, Rekognition may use uploaded photos to improve AWS's models, and [mykeebs.com's AI statement](https://github.com/roguekeyboards/mykeebs.com/blob/main/src/routes/about/ai/%2Bpage.svelte) promises that nothing in mykeebs is used to train a model.

### First-time account bootstrap

New AWS account, never deployed to before? With an admin profile for it:

```sh
AWS_PROFILE=<profile> mise run bootstrap <dev|ci|prod>
```

It checks the credentials belong to that account, then deploys its bootstrap stacks in order. `bootstrap/stacks.yaml` lists them per account: the artifact bucket, the [stack permissions boundary](#stack-permissions-boundary), image delivery (the image signing key pair, generated once, and the CloudFront key group and response headers policy every stack's distribution shares), the cost budget, and for `ci` and `prod` the GitHub Actions deploy role. Rerunning it is safe. **If you're forking this repo** to deploy to your own account, add your account to `stacks.yaml` as `dev` and run `mise run bootstrap dev`; the `ci` and `prod` stacks are this project's own.

Then the steps a script can't do:

- **`mykeebs-api-ci` account:** set the `mykeebs-api-bootstrap-jwks` stack's `JWKSBucketName` output as the `JWKS_BUCKET_NAME` GitHub Actions repo variable (Settings → Secrets and variables → Actions → Variables). `ci.yml`'s functional-test job fails at "Generate OIDC signing material" until it's set.
- **`mykeebs-api-prod` account:** set the `mykeebs-api-prod-oidc` stack's `GitHubActionsDeployRoleArn` output as the `PROD_DEPLOY_ROLE_ARN` repo variable. CI's role ARN is in `.github/actions/mykeebs-api-pr-stack/action.yml`.
- **Every account:** confirm the cost budget's email subscription. Its link only works while you're signed in to that account's console.

CI and the prod deploy both run `mise run bootstrap-check`, which stops them when one of the account's bootstrap stacks is behind the repo. `ci` runs it on PRs too, so a change to a bootstrap template needs `mise run bootstrap ci` from its branch before its checks can pass. Run `mise run bootstrap` for every other account the template is in before merging, so the prod deploy never stops.

#### Stack permissions boundary

`template.yaml` puts `bootstrap/stack-boundary.yaml`'s managed policy on every role it creates. It's the most any stack role can do, so a change that needs a new AWS permission updates this template in every account (`mykeebs-api-ci` first, or the PR's functional tests fail), then merges.

#### Setting up `mykeebs-api-prod`

After `mise run bootstrap prod`:

1. **Raise the Lambda concurrency quota.** A new account's limit of 10 is below the API's `ApiReservedConcurrency` of 15. Request 1000 in Service Quotas (Lambda → Concurrent executions).
2. **Certificates for `api.mykeebs.info` (in `us-east-2`) and `images.mykeebs.info` (in `us-east-1`).** Request each in ACM with DNS validation, add its validation CNAME to the `mykeebs.info` Cloudflare zone (DNS only, for example with `cf dns records create`), and wait for it to be issued. Their ARNs go in `CustomDomainCertificateArn` and `ImagesDomainCertificateArn`.
3. **Fill in the `[prod]` section of `samconfig.toml`.**
4. **First deploy, by hand:** `AWS_PROFILE=mykeebs-api-prod-admin mise run setup prod`. It also points `api.mykeebs.info` and `images.mykeebs.info` at the stack.
5. **Confirm the alarm email subscription.**

#### ECR repos

`sam deploy` pushes the function's image before it creates or updates the stack, so a repo can't live in `template.yaml`: it's in its own permanent stack from `bootstrap/ecr-repo.yaml`. `mise run setup` deploys one per environment. CI's per-PR stacks all share one, `mykeebs-api-ecr-bootstrap`, which `mise run bootstrap ci` deploys. Its defaults (`RepositoryName=mykeebs-api`, `ExpireBy=age`) are CI's: images expire 2 days after push, since nothing there is long-lived.

## Testing strategy

Two test layers, kept separate — don't mix them:

- **Unit tests** (`*_test.go` next to the code they test): `testify/suite` + `mockery`-generated mocks. No real infra, no network calls.
- **Functional tests** (`test/functional/features/`): Ginkgo + Gomega, driving real HTTP/MCP calls against a running stack (local via `func-setup`, or a real deployed stack via `MYKEEBS_API_BASE_URL`).
  - The `authorize` suite also opens the sign-in page in headless Chromium ([`playwright-go`](https://github.com/mxschmitt/playwright-go), installed by `func-test`). The stacks under test aren't connected to a real IdP, so `test/functional/support/browser` answers the IdP SDK's API calls with a stub and refuses requests to any origin other than the stack, the IdP and mykeebs.com's fonts. These specs are what check an IdP SDK bump.

Functional specs follow a consistent BDD shape: `Describe` the subject (and, for multi-action resources, a nested `Describe` per action) → `Context("given <precondition>")` → `When("<action>")` → one `It` with a `By(...)` per assertion. Build the actual request inside the innermost `BeforeEach`, not an outer one — Ginkgo's context is scoped to the node that received it.

Functional tests run no auth server. [`oidc-testkit`](https://github.com/roguekeyboards/oidc-testkit) generates a signing key + discovery doc + JWKS pre-deploy; the JSON is published where the deployed authorizer and `auth.NewVerifier` can fetch it (floci's S3 locally, the public `mykeebs-jwks` bucket in CI), and the suite signs its own tokens via `pkg/oidctest` (see `test/functional/support/api/token.go`).

## Conventions

- **Commit messages and PR titles** follow [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): subject` (e.g. `fix(ci): scope IAM permissions to account/region`). Common types: `feat`, `fix`, `chore`, `docs`, `test`, `ci`, `refactor`.
- **Changes to deployed behavior use `fix(...)` or `feat(...)`, never `chore(...)`.** Only commits that cut a release reach prod, and `chore` doesn't. That includes hand-written changes to `template.yaml`. Renovate follows the same rule: Dockerfile image and Go toolchain bumps are `fix(deps)`, while CI actions, dev tools and lock file maintenance stay `chore(deps)`.
- **Mise tasks live in `scripts/`**, one `.sh` file per task, referenced from `mise.toml` via `file = "scripts/<name>.sh"` — even one-liners. Add a new task the same way.
- Package layout, mocking patterns, and other code-level conventions are documented in `CLAUDE.md`.
