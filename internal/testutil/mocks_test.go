package testutil_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi-cloudflare/sdk/v6/go/cloudflare"
	"github.com/pulumi/pulumi-github/sdk/v6/go/github"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/testutil"
)

func run(t *testing.T, m *testutil.Mocks, body pulumi.RunFunc) {
	t.Helper()
	if err := pulumi.RunErr(body, pulumi.WithMocks(testutil.Project, testutil.Stack, m)); err != nil {
		t.Fatalf("RunErr: %v", err)
	}
}

// await returns the next value on ch or fails the test after a timeout.
func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an output to resolve")
		panic("unreachable")
	}
}

func TestRecordingAndComputedOutputs(t *testing.T) {
	m := testutil.NewMocks()
	nameServers := make(chan []string, 1)
	subdomain := make(chan string, 1)
	tokenValue := make(chan string, 1)
	zoneID := make(chan string, 1)

	run(t, m, func(ctx *pulumi.Context) error {
		zone, err := cloudflare.NewZone(ctx, "zone", &cloudflare.ZoneArgs{
			Account: cloudflare.ZoneAccountArgs{Id: pulumi.String("acct")},
			Name:    pulumi.String("example.com"),
			Type:    pulumi.String("full"),
		}, pulumi.Protect(true))
		if err != nil {
			return err
		}
		project, err := cloudflare.NewPagesProject(ctx, "pages-project", &cloudflare.PagesProjectArgs{
			AccountId:        pulumi.String("acct"),
			Name:             pulumi.String("site"),
			ProductionBranch: pulumi.String("production"),
		})
		if err != nil {
			return err
		}
		token, err := cloudflare.NewAccountToken(ctx, "ci-deploy-token", &cloudflare.AccountTokenArgs{
			AccountId: pulumi.String("acct"),
			Name:      pulumi.String("t"),
			Policies:  cloudflare.AccountTokenPolicyArray{},
		})
		if err != nil {
			return err
		}

		zone.NameServers.ApplyT(func(v []string) []string { nameServers <- v; return v })
		zone.ID().ApplyT(func(v pulumi.ID) pulumi.ID { zoneID <- string(v); return v })
		project.Subdomain.ApplyT(func(v string) string { subdomain <- v; return v })
		token.Value.ApplyT(func(v string) string { tokenValue <- v; return v })
		return nil
	})

	if got := strings.Join(await(t, nameServers), ","); got != "ada.ns.cloudflare.com,bob.ns.cloudflare.com" {
		t.Errorf("zone name servers = %q", got)
	}
	if got := await(t, zoneID); got != "zone_id" {
		t.Errorf("zone ID = %q, want zone_id", got)
	}
	if got := await(t, subdomain); got != "pages-project.pages.dev" {
		t.Errorf("pages subdomain = %q, want <name>.pages.dev from the logical name", got)
	}
	if got := await(t, tokenValue); got != "test-token-value" {
		t.Errorf("token value = %q", got)
	}

	if got := len(m.Resources); got != 3 {
		t.Fatalf("recorded %d resources, want 3", got)
	}
	zones := m.ByType(testutil.TypeZone)
	if len(zones) != 1 || zones[0].Name != "zone" {
		t.Fatalf("ByType(zone) = %+v", zones)
	}
	if !zones[0].RegisterRPC.GetProtect() {
		t.Error("RegisterRPC.GetProtect() = false for a Protect(true) zone")
	}
	if got := zones[0].Inputs["name"].StringValue(); got != "example.com" {
		t.Errorf("recorded zone name input = %q", got)
	}
	if got := len(m.ByType(testutil.TypeDNSRecord)); got != 0 {
		t.Errorf("ByType(dns) returned %d, want 0", got)
	}
	if r, ok := m.ByName("pages-project"); !ok || r.TypeToken != testutil.TypePagesProject {
		t.Errorf("ByName(pages-project) = %+v, %v", r, ok)
	}
	if _, ok := m.ByName("missing"); ok {
		t.Error("ByName(missing) found a resource")
	}
}

func TestResourceTypeTokensMatchTheSDK(t *testing.T) {
	// Register one of each resource the program uses and check the SDK's real
	// type token equals our constant, so a token typo cannot go unnoticed.
	m := testutil.NewMocks()
	run(t, m, func(ctx *pulumi.Context) error {
		if _, err := cloudflare.NewZoneSetting(ctx, "zs", &cloudflare.ZoneSettingArgs{
			ZoneId: pulumi.String("z"), SettingId: pulumi.String("ssl"), Value: pulumi.String("strict"),
		}); err != nil {
			return err
		}
		if _, err := cloudflare.NewDnsRecord(ctx, "dns", &cloudflare.DnsRecordArgs{
			ZoneId: pulumi.String("z"), Name: pulumi.String("a.example.com"), Type: pulumi.String("CNAME"),
			Content: pulumi.String("b.example.com"), Ttl: pulumi.Float64(1),
		}); err != nil {
			return err
		}
		if _, err := cloudflare.NewPagesDomain(ctx, "pd", &cloudflare.PagesDomainArgs{
			AccountId: pulumi.String("a"), ProjectName: pulumi.String("p"), Name: pulumi.String("example.com"),
		}); err != nil {
			return err
		}
		if _, err := github.NewActionsSecret(ctx, "gs", &github.ActionsSecretArgs{
			Repository: pulumi.String("r"), SecretName: pulumi.String("S"), Value: pulumi.String("v"),
		}); err != nil {
			return err
		}
		if _, err := github.NewActionsVariable(ctx, "gv", &github.ActionsVariableArgs{
			Repository: pulumi.String("r"), VariableName: pulumi.String("V"), Value: pulumi.String("v"),
		}); err != nil {
			return err
		}
		if _, err := github.NewRepositoryEnvironment(ctx, "ge", &github.RepositoryEnvironmentArgs{
			Repository: pulumi.String("r"), Environment: pulumi.String("dev"),
		}); err != nil {
			return err
		}
		_, err := github.NewBranchProtection(ctx, "gb", &github.BranchProtectionArgs{
			RepositoryId: pulumi.String("R_r"), Pattern: pulumi.String("main"),
		})
		return err
	})

	want := map[string]string{
		"zs":  testutil.TypeZoneSetting,
		"dns": testutil.TypeDNSRecord,
		"pd":  testutil.TypePagesDomain,
		"gs":  testutil.TypeActionsSecret,
		"gv":  testutil.TypeActionsVariable,
		"ge":  testutil.TypeRepoEnvironment,
		"gb":  testutil.TypeBranchProtection,
	}
	for name, token := range want {
		r, ok := m.ByName(name)
		if !ok {
			t.Errorf("%s was not recorded", name)
			continue
		}
		if r.TypeToken != token {
			t.Errorf("%s: SDK registered %q, constant is %q", name, r.TypeToken, token)
		}
	}
}

func TestPermissionGroupsInvoke(t *testing.T) {
	m := testutil.NewMocks()
	type group struct{ id, name string }
	got := make(chan []group, 1)

	run(t, m, func(ctx *pulumi.Context) error {
		res, err := cloudflare.LookupAccountApiTokenPermissionGroupsList(ctx, &cloudflare.LookupAccountApiTokenPermissionGroupsListArgs{
			AccountId: ptr("acct"),
		})
		if err != nil {
			return err
		}
		var groups []group
		for _, r := range res.Results {
			groups = append(groups, group{r.Id, r.Name})
		}
		got <- groups
		return nil
	})

	groups := await(t, got)
	if len(groups) != 2 || groups[0] != (group{"pages-write-id", "Pages Write"}) || groups[1] != (group{"other-id", "DNS Write"}) {
		t.Errorf("permission groups = %+v", groups)
	}
	if len(m.Calls) != 1 || m.Calls[0].Token != testutil.CallPermissionGroups {
		t.Errorf("recorded calls = %+v, want one %s", m.Calls, testutil.CallPermissionGroups)
	}
}

func TestGetRepositoryInvoke(t *testing.T) {
	tests := []struct {
		name string
		args github.LookupRepositoryArgs
		want string
	}{
		{name: "by name", args: github.LookupRepositoryArgs{Name: ptr("elyclover.com")}, want: "R_elyclover.com"},
		{name: "by full name", args: github.LookupRepositoryArgs{FullName: ptr("kevholmes/elyclover.com-infra")}, want: "R_elyclover.com-infra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testutil.NewMocks()
			got := make(chan string, 1)
			run(t, m, func(ctx *pulumi.Context) error {
				repo, err := github.LookupRepository(ctx, &tt.args)
				if err != nil {
					return err
				}
				got <- repo.NodeId
				return nil
			})
			if v := await(t, got); v != tt.want {
				t.Errorf("nodeId = %q, want %q", v, tt.want)
			}
			if len(m.Calls) != 1 || m.Calls[0].Token != testutil.CallGithubGetRepo {
				t.Errorf("recorded calls = %+v, want one %s", m.Calls, testutil.CallGithubGetRepo)
			}
		})
	}
}

func TestUnknownInvokeReturnsEmpty(t *testing.T) {
	m := testutil.NewMocks()
	res, err := m.Call(pulumi.MockCallArgs{Token: "some:other/invoke:invoke"})
	if err != nil || len(res) != 0 {
		t.Errorf("Call(unknown) = %v, %v; want empty and no error", res, err)
	}
	if len(m.Calls) != 1 {
		t.Errorf("unknown call was not recorded: %+v", m.Calls)
	}
}

func TestSetConfig(t *testing.T) {
	t.Setenv(pulumi.EnvConfig, "")
	t.Setenv(pulumi.EnvConfigSecretKeys, "")

	t.Run("sets and encodes", func(t *testing.T) {
		testutil.SetConfig(t, map[string]string{"elyclover.com-infra:domain": "example.com", "elyclover.com-infra:emailRecords": `[{"a":1}]`})

		var cfg map[string]string
		if err := json.Unmarshal([]byte(os.Getenv(pulumi.EnvConfig)), &cfg); err != nil {
			t.Fatalf("PULUMI_CONFIG is not a JSON object of strings: %v", err)
		}
		if cfg["elyclover.com-infra:domain"] != "example.com" || cfg["elyclover.com-infra:emailRecords"] != `[{"a":1}]` {
			t.Errorf("PULUMI_CONFIG = %v", cfg)
		}
		var secrets []string
		if err := json.Unmarshal([]byte(os.Getenv(pulumi.EnvConfigSecretKeys)), &secrets); err != nil {
			t.Fatalf("PULUMI_CONFIG_SECRET_KEYS is not a JSON array: %v", err)
		}
		if len(secrets) != 1 || secrets[0] != "elyclover.com-infra:releaserToken" {
			t.Errorf("secret keys = %v, want [elyclover.com-infra:releaserToken]", secrets)
		}
	})

	if v := os.Getenv(pulumi.EnvConfig); v != "" {
		t.Errorf("PULUMI_CONFIG not restored after the subtest: %q", v)
	}
}

func TestValidConfig(t *testing.T) {
	a, b := testutil.ValidConfig(), testutil.ValidConfig()
	a["elyclover.com-infra:domain"] = "changed.example"
	if b["elyclover.com-infra:domain"] != "elyclover.com" {
		t.Error("ValidConfig returns a shared map; callers' edits leak between tests")
	}

	for _, k := range []string{
		"cloudflare:apiToken", "releaserToken", "cloudflareAccountId", "domain", "pagesProjectName",
		"pagesProductionBranch", "pagesPreviewSubdomains", "enableCustomDomains", "siteRepo", "infraRepo",
		"siteRequiredChecks", "infraRequiredChecks", "releaserTokenExpiresOn", "pagesWritePermissionGroupName",
		"emailRecords",
	} {
		full := k
		if !strings.Contains(k, ":") {
			full = testutil.Project + ":" + k
		}
		if strings.TrimSpace(b[full]) == "" {
			t.Errorf("ValidConfig is missing %s", full)
		}
	}
	for _, k := range []string{"pagesPreviewSubdomains", "siteRequiredChecks", "infraRequiredChecks", "emailRecords"} {
		if !json.Valid([]byte(b[testutil.Project+":"+k])) {
			t.Errorf("%s is not valid JSON", k)
		}
	}
	if got := b[testutil.Project+":cloudflareAccountId"]; got != "0123456789abcdef0123456789abcdef" {
		t.Errorf("fixture account ID = %q, want the fake test ID", got)
	}
}

func ptr[T any](v T) *T { return &v }
