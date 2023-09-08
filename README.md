# elyclover.com-infra

As of September 2023, elyclover.com is hosted on GitHub pages. There are a number over very unique benefits
to using this service to host static pages. It's very affordable and included with a GH Pro subscription,
it includes managed TLS and custom domains, along with a very easy CI/CD process using GitHub Actions
and their beta gh pages Action.

Some downsides are that GH Pages has some size restrictions on what can actually be hosted. If you have a site
with a lot of media content being deployed directly to GH pages, this can cause timeouts and other issues as
we've already seen so far when adding large quantities of images in the 500KiB-2MiB range.

In order to ensure the CI/CD process remains reliable and doesn't timeout - and we we eventually do not go over the size limit
allowed for GH Pages in general - I have created this `elyclover.com-infra` repo to host IaC that generates
other resources for us to ship big items off to. This will complicate the CI/CD and development process for elyclover.com, unfortunately.

What I'm thinking we can do is have the PRs from elyclover.com actually upload the content to our Azure blob storage and
build/deploy the entire website to a `dev`` area with a generic domain from Azure blob storage.

Then when we have a release-please PR that would be built and shipped to a `staging` area.

The final step upon having a release generated is to then build and ship this off to a `prod` area.

This repo is currently using Azure Bicep IaC to describe and configure
the following resources:

1. A Storage Account for each environment
1. Blob service with `public-media` container in each storage account for assets
1. A single CDN profile using the Standard Microsoft CDN (legacy, not Front Door at this time)
1. CDN Endpoints using our single CDN Profile for each storage account

## Azure Storage

This uses the existing `root` Resource Group within the existing `elyclover.com` Subscription in Azure.

```bash
az account set --subscription elyclover.com
az provider register --namespace Microsoft.Cdn
az deployment group create --resource-group root --template-file azure-bicep/main.bicep
```

## CI/CD

This is manually run at the moment from my local CLI with an Azure Owner acct.
Bicep does not maintain state remotely/locally.

### Release management

The release-please Action is used when combined with conventional commit PR titles for semantic versioning.

If we end up making frequent changes here it might be worth wiring up a service principal and keys
to add into GH Secrets and automate this process... but for now it's going to be a lot of initial
dev work to get storage/CDNs up and working, followed by almost no changes for the forseeable future.
