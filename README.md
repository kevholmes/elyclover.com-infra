# elyclover.com-infra

Pulumi (Go) infrastructure for [elyclover.com](https://elyclover.com): Cloudflare Pages hosting, Cloudflare DNS and the GitHub
Actions wiring that lets the site repo deploy itself. The site lives in
[kevholmes/elyclover.com](https://github.com/kevholmes/elyclover.com).

The Azure implementation (Storage, Classic CDN, Key Vault, SOPS) is preserved at tag `azure-final`.

State is managed in Pulumi Cloud. There is a single stack, `prod`.

## What the stack manages

Cloudflare resources (`internal/cloudflare`):

| Logical name | Type | Notes |
| --- | --- | --- |
| `zone` | `cloudflare.Zone` | Full zone for the domain, `pulumi.Protect(true)` |
| `zone-setting-ssl` | `cloudflare.ZoneSetting` | `ssl` = `strict` |
| `zone-setting-always-use-https` | `cloudflare.ZoneSetting` | `always_use_https` = `on` |
| `zone-setting-min-tls-version` | `cloudflare.ZoneSetting` | `min_tls_version` = `1.2` |
| `dns-email-<index>-<type>` | `cloudflare.DnsRecord` | One per entry in `emailRecords` (MX, TXT, CNAME), not proxied |
| `pages-project` | `cloudflare.PagesProject` | Direct Upload project (no Git source) |
| `pages-domain-apex`, `pages-domain-<sub>` | `cloudflare.PagesDomain` | Only when `enableCustomDomains` is true |
| `dns-pages-apex`, `dns-pages-<sub>` | `cloudflare.DnsRecord` | Proxied CNAMEs to the Pages project, only when `enableCustomDomains` is true |
| `ci-deploy-token` | `cloudflare.AccountToken` | Pages Write on this account only; the value is handed to GitHub as a secret |

GitHub resources (`internal/github`):

| Logical name | Type | Notes |
| --- | --- | --- |
| `site-secret-cloudflare-api-token` | `github.ActionsSecret` | `CLOUDFLARE_API_TOKEN` on the site repo, from `ci-deploy-token` |
| `site-var-cloudflare-account-id` | `github.ActionsVariable` | `CLOUDFLARE_ACCOUNT_ID` on the site repo |
| `site-secret-releaser-token` | `github.ActionsSecret` | `RELEASER_TOKEN` on the site repo |
| `infra-secret-releaser-token` | `github.ActionsSecret` | `RELEASER_TOKEN` on this repo |
| `site-env-dev`, `site-env-stg`, `site-env-prod` | `github.RepositoryEnvironment` | Deployment environments on the site repo |
| `site-main-protection` | `github.BranchProtection` | Required checks `deploy` and `GitGuardian Security Checks`, 0 approvals |
| `infra-main-protection` | `github.BranchProtection` | Required checks `lint`, `test` and `GitGuardian Security Checks`, 0 approvals |

Stack outputs: `zoneId`, `nameServers`, `pagesSubdomain`, `customDomainsEnabled`, `releaserTokenDaysLeft`, `customDomains`.

Stack config (non-secret keys live in `Pulumi.prod.yaml`): `cloudflareAccountId`, `domain`, `pagesProjectName`,
`pagesProductionBranch`, `pagesPreviewSubdomains`, `enableCustomDomains`, `siteRepo`, `infraRepo`, `siteRequiredChecks`,
`infraRequiredChecks`, `releaserTokenExpiresOn`, `pagesWritePermissionGroupName`, `emailRecords`.
Secret config: `cloudflare:apiToken` (read by the Cloudflare provider) and `elyclover.com-infra:releaserToken`.
The program fails with a clear error if either secret is missing.

## Prerequisites

- Go (version in `go.mod`), the Pulumi CLI, the GitHub CLI (`gh`, logged in) and `jq`
- For `make lint`: `markdownlint` (`markdownlint-cli`). `staticcheck` runs as `go tool staticcheck`, pinned in `go.mod`, so there is nothing to install
- A Pulumi Cloud account for state
- A Cloudflare account (the account ID is the `cloudflareAccountId` config value)
- A GitHub fine-grained PAT for `RELEASER_TOKEN`

## Bootstrap

1. Cloudflare bootstrap token. In the dashboard go to Manage Account, Account API Tokens, Create Token, Custom token
   named `pulumi-bootstrap-elyclover`, with these permissions and no expiry (store it in a password manager):
   - Account: Account API Tokens Edit, Cloudflare Pages Edit, Account Settings Read
   - Zone: Zone Edit, Zone Settings Edit, DNS Edit (all zones from the account)
2. `RELEASER_TOKEN` PAT. Create a fine-grained token named `releaser-elyclover`, owner `kevholmes`, only the repositories
   `elyclover.com` and `elyclover.com-infra`, with Contents, Pull requests and Issues set to Read and write.
   Record its rotation date in `releaserTokenExpiresOn` (`YYYY-MM-DD`).
3. Confirm the `Pages Write` permission group exists for the account (the name is the `pagesWritePermissionGroupName` config value):

   ```bash
   read -rs CF_BOOTSTRAP_TOKEN; export CF_BOOTSTRAP_TOKEN
   ACCOUNT_ID="$(pulumi config get elyclover.com-infra:cloudflareAccountId)"
   curl -sS "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/tokens/permission_groups" \
     -H "Authorization: Bearer $CF_BOOTSTRAP_TOKEN" | jq '[.result[] | select(.name=="Pages Write")] | length'
   ```

4. Create the stack, set the secrets (hidden prompts) and preview. Keep custom domains off until the zone is active:

   ```bash
   pulumi stack init kevholmes/prod
   pulumi config set cloudflare:apiToken --secret                 # paste the bootstrap token
   pulumi config set elyclover.com-infra:releaserToken --secret   # paste the PAT
   export GITHUB_TOKEN="$(gh auth token)"
   pulumi preview --diff
   ```

5. If `main` already has a branch protection rule in either repo, delete it first, because creating a `BranchProtection`
   fails while a rule exists. Then apply right away so the repos are unprotected only for the length of the run:

   ```bash
   gh api -X DELETE repos/kevholmes/elyclover.com-infra/branches/main/protection
   gh api -X DELETE repos/kevholmes/elyclover.com/branches/main/protection
   pulumi up --yes
   pulumi stack output --json | jq '{zoneId, nameServers, pagesSubdomain, customDomainsEnabled, releaserTokenDaysLeft}'
   ```

6. Point the registrar's nameservers at the two `nameServers` outputs and wait for the zone to become `active`
   (see the next section).

## Custom domains (`enableCustomDomains`)

`enableCustomDomains` defaults to `false`. While it is false the stack creates the zone, settings, email records, Pages
project, deploy token and GitHub wiring, but no Pages domains and no site CNAMEs. The flag is turned on only after the
nameservers have moved and the zone is `active`:

```bash
git switch -c chore/enable-custom-domains
pulumi config set elyclover.com-infra:enableCustomDomains true
git commit -am "chore: enable pages custom domains"
# open a PR, merge it once lint and test pass, then from main:
export GITHUB_TOKEN="$(gh auth token)"
pulumi preview --diff   # expect exactly 6 creates: 3 PagesDomain + 3 DnsRecord (apex, dev, stg)
pulumi up --yes
```

The CNAME targets are built from config (`<project>.pages.dev` and `<sub>.<project>.pages.dev`). Before enabling the flag,
check that the `pagesSubdomain` output equals `<pagesProjectName>.pages.dev`; Cloudflare adds a suffix when the name is taken.

## Make targets

| Target | What it does |
| --- | --- |
| `make build` | `go build ./...` |
| `make test` | `go test ./... -race` with atomic coverage over `./internal/...` (testutil excluded), writes `coverage.out` |
| `make coverage-check` | Runs `make test`, then fails if total coverage is below 80% |
| `make lint` | Runs `lint-go` and `lint-markdown` |
| `make lint-go` | `go vet ./...` and `go tool staticcheck ./...` (staticcheck is a Go tool dependency pinned in `go.mod`) |
| `make lint-markdown` | `markdownlint '**/*.md' --ignore CHANGELOG.md` |

All tests run under Pulumi mocks with no network access.

## CI

- `ci.yml` runs on pull requests with two required jobs: `lint` and `test`.
- `pat-expiry.yml` runs weekly and fails when `releaserTokenExpiresOn` in `Pulumi.prod.yaml` is less than 30 days away.
  GitHub emails you about failed scheduled runs, so the reminder does not depend on anyone running `pulumi up`.
- `release-please.yml` opens release PRs using `RELEASER_TOKEN`.
- ggshield runs as a pre-commit hook. The first time, authenticate with `$GGSHIELD auth login`.

## Runbooks

### Rotate `RELEASER_TOKEN`

Three things alert you: the weekly `pat-expiry` workflow starts failing 30 days before `releaserTokenExpiresOn`, `pulumi up`
and `pulumi preview` warn, and GitHub emails before a PAT expires.

1. In the GitHub UI, create a new fine-grained PAT with the same settings as in Bootstrap step 2.
2. Update the stack:

   ```bash
   export GITHUB_TOKEN="$(gh auth token)"
   pulumi config set elyclover.com-infra:releaserToken --secret
   pulumi config set elyclover.com-infra:releaserTokenExpiresOn YYYY-MM-DD
   pulumi up --yes
   ```

3. Commit the `Pulumi.prod.yaml` change through a `chore:` PR, then delete the old PAT in the UI.

### Rotate the Cloudflare bootstrap token

1. In the Cloudflare dashboard, create a new token with the Bootstrap step 1 permissions.
2. Run `pulumi config set cloudflare:apiToken --secret`, then `pulumi up`.
3. Delete the old token.

The CI deploy token (`CLOUDFLARE_API_TOKEN` on the site repo) is owned by Pulumi. `pulumi up --replace <AccountToken URN>`
rotates it and updates the GitHub secret in the same run.

### Roll back prod content

Wrangler has no Pages rollback command. The preferred way needs the CLI only and no Cloudflare token: re-run the prod
deploy of the last good release.

```bash
gh run list -R kevholmes/elyclover.com --workflow cicd-release.yml --json databaseId,displayTitle,createdAt
gh run rerun <databaseId> -R kevholmes/elyclover.com
```

Pick the run for the good tag. A re-run reuses the original release event, so it checks out that old tag and redeploys it
to `production`. Other options:

- Revert the bad commit through a `fix:` PR, which produces a new release.
- Roll back through the Pages API:

  ```bash
  read -rs CF_BOOTSTRAP_TOKEN; export CF_BOOTSTRAP_TOKEN            # paste from your password manager
  ACCOUNT_ID="$(pulumi config get elyclover.com-infra:cloudflareAccountId)"
  curl -sS -H "Authorization: Bearer $CF_BOOTSTRAP_TOKEN" "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/pages/projects/elyclover/deployments?env=production" | jq -r '.result[] | "\(.id) \(.created_on) \(.deployment_trigger.metadata.commit_hash)"'
  curl -sS -X POST -H "Authorization: Bearer $CF_BOOTSTRAP_TOKEN" "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/pages/projects/elyclover/deployments/<id>/rollback" | jq .success
  ```

### Email broken after a nameserver switch

Only possible while the old Azure DNS zone still exists.

1. At the registrar (Namecheap), go to Custom DNS and set the nameservers back to the four Azure ones:
   `ns1-36.azure-dns.com`, `ns2-36.azure-dns.net`, `ns3-36.azure-dns.org`, `ns4-36.azure-dns.info`.
2. Fix the email records in the Pulumi config (`emailRecords`), run `pulumi up`, confirm the answers from the Cloudflare
   nameservers match the old zone, then repeat the nameserver switch.
