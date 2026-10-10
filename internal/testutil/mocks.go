// Package testutil provides recording Pulumi mocks and config helpers for
// unit-testing the infrastructure program without any network access.
//
// Assert on the recorded MockResourceArgs after pulumi.RunErr returns. Do not
// assert inside a bare ApplyT callback: RunErr does not wait for ApplyTs that
// are not attached to a resource or ctx.Export, so the callback may never run
// and the test would pass vacuously. To check an output, export it (ctx.Export
// does make RunErr wait) and capture the value.
package testutil

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Project and Stack are the names to pass to pulumi.WithMocks.
const (
	Project = "elyclover.com-infra"
	Stack   = "prod"
)

// Resource type tokens used by the program.
const (
	TypeZone             = "cloudflare:index/zone:Zone"
	TypeZoneSetting      = "cloudflare:index/zoneSetting:ZoneSetting"
	TypeDNSRecord        = "cloudflare:index/dnsRecord:DnsRecord"
	TypePagesProject     = "cloudflare:index/pagesProject:PagesProject"
	TypePagesDomain      = "cloudflare:index/pagesDomain:PagesDomain"
	TypeAccountToken     = "cloudflare:index/accountToken:AccountToken"
	TypeActionsSecret    = "github:index/actionsSecret:ActionsSecret"
	TypeActionsVariable  = "github:index/actionsVariable:ActionsVariable"
	TypeRepoEnvironment  = "github:index/repositoryEnvironment:RepositoryEnvironment"
	TypeBranchProtection = "github:index/branchProtection:BranchProtection"
	CallPermissionGroups = "cloudflare:index/getAccountApiTokenPermissionGroupsList:getAccountApiTokenPermissionGroupsList"
	CallGithubGetRepo    = "github:index/getRepository:getRepository"
	ReleaserTokenKey     = Project + ":releaserToken"
)

// Mocks records every resource registration and invoke and returns canned,
// deterministic answers. It implements pulumi.MockResourceMonitor.
type Mocks struct {
	mu        sync.Mutex
	Resources []pulumi.MockResourceArgs
	Calls     []pulumi.MockCallArgs
}

// NewMocks returns an empty recording mock monitor.
func NewMocks() *Mocks { return &Mocks{} }

// NewResource records args and returns the ID "<name>_id" with the inputs as
// state, plus computed outputs for the types that have them.
func (m *Mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	m.Resources = append(m.Resources, args)
	m.mu.Unlock()

	state := resource.PropertyMap{}
	for k, v := range args.Inputs {
		state[k] = v
	}

	switch args.TypeToken {
	case TypeZone:
		state["nameServers"] = resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("ada.ns.cloudflare.com"),
			resource.NewProperty("bob.ns.cloudflare.com"),
		})
	case TypePagesProject:
		state["subdomain"] = resource.NewProperty(args.Name + ".pages.dev")
	case TypeAccountToken:
		state["value"] = resource.NewProperty("test-token-value")
	}

	return args.Name + "_id", state, nil
}

// Call records args and answers the two invokes the program makes; any other
// invoke returns an empty result.
func (m *Mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, args)
	m.mu.Unlock()

	switch args.Token {
	case CallPermissionGroups:
		scopes := resource.NewProperty([]resource.PropertyValue{resource.NewProperty("com.cloudflare.api.account")})
		return resource.PropertyMap{
			"results": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty(resource.PropertyMap{
					"id":     resource.NewProperty("pages-write-id"),
					"name":   resource.NewProperty("Pages Write"),
					"scopes": scopes,
				}),
				resource.NewProperty(resource.PropertyMap{
					"id":     resource.NewProperty("other-id"),
					"name":   resource.NewProperty("DNS Write"),
					"scopes": scopes,
				}),
			}),
		}, nil
	case CallGithubGetRepo:
		// github.LookupRepository takes either name or fullName ("owner/name").
		name := ""
		if v, ok := args.Args["name"]; ok && v.IsString() {
			name = v.StringValue()
		} else if v, ok := args.Args["fullName"]; ok && v.IsString() {
			name = v.StringValue()
			if i := strings.LastIndex(name, "/"); i >= 0 {
				name = name[i+1:]
			}
		}
		return resource.PropertyMap{
			"nodeId": resource.NewProperty("R_" + name),
		}, nil
	}
	return resource.PropertyMap{}, nil
}

// ByType returns the recorded resources with the given type token, in
// registration order.
func (m *Mocks) ByType(token string) []pulumi.MockResourceArgs {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []pulumi.MockResourceArgs
	for _, r := range m.Resources {
		if r.TypeToken == token {
			out = append(out, r)
		}
	}
	return out
}

// ByName returns the recorded resource with the given logical name.
func (m *Mocks) ByName(name string) (pulumi.MockResourceArgs, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.Resources {
		if r.Name == name {
			return r, true
		}
	}
	return pulumi.MockResourceArgs{}, false
}

// SetConfig sets PULUMI_CONFIG to the JSON encoding of cfg (keys are fully
// qualified, e.g. "elyclover.com-infra:domain") and marks
// elyclover.com-infra:releaserToken as a secret key. Both variables are
// restored when the test ends.
func SetConfig(t testing.TB, cfg map[string]string) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	keys, err := json.Marshal([]string{ReleaserTokenKey})
	if err != nil {
		t.Fatalf("marshal secret keys: %v", err)
	}
	t.Setenv(pulumi.EnvConfig, string(raw))
	t.Setenv(pulumi.EnvConfigSecretKeys, string(keys))
}

// ValidConfig returns a complete, valid stack configuration. Structured values
// are JSON strings, exactly as the Pulumi engine passes them to the program.
// Callers get a fresh map and may modify it freely. The account ID and tokens
// are fake.
func ValidConfig() map[string]string {
	return map[string]string{
		"cloudflare:apiToken":                      "test-cloudflare-api-token",
		Project + ":releaserToken":                 "test-releaser-token",
		Project + ":cloudflareAccountId":           "0123456789abcdef0123456789abcdef",
		Project + ":domain":                        "elyclover.com",
		Project + ":pagesProjectName":              "elyclover",
		Project + ":pagesProductionBranch":         "production",
		Project + ":pagesPreviewSubdomains":        `["dev","stg"]`,
		Project + ":enableCustomDomains":           "false",
		Project + ":siteRepo":                      "elyclover.com",
		Project + ":infraRepo":                     "elyclover.com-infra",
		Project + ":siteRequiredChecks":            `["deploy","GitGuardian Security Checks"]`,
		Project + ":infraRequiredChecks":           `["lint","test","GitGuardian Security Checks"]`,
		Project + ":releaserTokenExpiresOn":        "2027-10-10",
		Project + ":pagesWritePermissionGroupName": "Pages Write",
		Project + ":emailRecords": `[
			{"type":"MX","name":"elyclover.com","content":"mail.protonmail.ch","priority":10},
			{"type":"MX","name":"elyclover.com","content":"mailsec.protonmail.ch","priority":20},
			{"type":"TXT","name":"elyclover.com","content":"v=spf1 include:_spf.protonmail.ch ~all"},
			{"type":"CNAME","name":"protonmail._domainkey.elyclover.com","content":"protonmail.domainkey.example.domains.proton.ch"},
			{"type":"TXT","name":"_dmarc.elyclover.com","content":"v=DMARC1; p=quarantine"}
		]`,
	}
}
