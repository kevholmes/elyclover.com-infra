package infra

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/testutil"
)

const key = testutil.Project + ":"

// stackType is the token of the stack resource Pulumi itself registers.
const stackType = "pulumi:pulumi:Stack"

// clockAt returns a fixed clock.
func clockAt(y int, m time.Month, d int) func() time.Time {
	return func() time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }
}

// A clock 8 days before the fixture's 2027-10-10 expiry.
var nearExpiry = clockAt(2027, time.October, 1)

// A clock a year before expiry.
var farFromExpiry = clockAt(2026, time.October, 10)

func enable(cfg map[string]string) { cfg[key+"enableCustomDomains"] = "true" }

func setup(t *testing.T, mutate func(map[string]string)) {
	t.Helper()
	cfg := testutil.ValidConfig()
	if mutate != nil {
		mutate(cfg)
	}
	testutil.SetConfig(t, cfg)
}

// runProgram runs programWithClock under mocks.
func runProgram(t *testing.T, mutate func(map[string]string), now func() time.Time) (*testutil.Mocks, error) {
	t.Helper()
	setup(t, mutate)
	m := testutil.NewMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		return programWithClock(ctx, now)
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, m))
	return m, err
}

// runBuild runs build under mocks and resolves every output it returns. The
// values are captured through ctx.Export, which makes RunErr wait for them.
func runBuild(t *testing.T, mutate func(map[string]string), now func() time.Time) (*testutil.Mocks, map[string]any, error) {
	t.Helper()
	setup(t, mutate)
	m := testutil.NewMocks()

	var mu sync.Mutex
	got := map[string]any{}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		exports, err := build(ctx, now)
		if err != nil {
			return err
		}
		for name, in := range exports {
			ctx.Export(name, pulumi.ToOutput(in).ApplyT(func(v any) any {
				mu.Lock()
				defer mu.Unlock()
				got[name] = v
				return v
			}))
		}
		return nil
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, m))
	return m, got, err
}

// userResources returns the recorded resources except the stack itself.
func userResources(m *testutil.Mocks) []pulumi.MockResourceArgs {
	var out []pulumi.MockResourceArgs
	for _, r := range m.Resources {
		if r.TypeToken != stackType {
			out = append(out, r)
		}
	}
	return out
}

func TestProgramRunsInBothGatingModes(t *testing.T) {
	off, err := runProgram(t, nil, farFromExpiry)
	if err != nil {
		t.Fatalf("program with enableCustomDomains=false: %v", err)
	}
	on, err := runProgram(t, enable, farFromExpiry)
	if err != nil {
		t.Fatalf("program with enableCustomDomains=true: %v", err)
	}

	// The fixture has two preview subdomains: apex + dev + stg = 3 domains, each
	// with one PagesDomain and one CNAME.
	subdomains := 2
	wantExtra := 2 * (subdomains + 1)
	if got := len(userResources(on)) - len(userResources(off)); got != wantExtra {
		t.Errorf("enableCustomDomains=true registers %d more resources, want %d", got, wantExtra)
	}
	if n := len(off.ByType(testutil.TypePagesDomain)); n != 0 {
		t.Errorf("%d pages domains with the flag off", n)
	}
	if n := len(on.ByType(testutil.TypePagesDomain)); n != subdomains+1 {
		t.Errorf("%d pages domains with the flag on, want %d", n, subdomains+1)
	}
}

func TestProgramPassesTheDeployTokenToTheSiteSecret(t *testing.T) {
	m, err := runProgram(t, nil, farFromExpiry)
	if err != nil {
		t.Fatalf("program: %v", err)
	}
	secret, ok := m.ByName("site-secret-cloudflare-api-token")
	if !ok {
		t.Fatal("site-secret-cloudflare-api-token was not registered")
	}
	v := secret.Inputs["value"]
	for v.IsSecret() {
		v = v.SecretValue().Element
	}
	// "test-token-value" is the mocked AccountToken.value output.
	if !v.IsString() || v.StringValue() != "test-token-value" {
		t.Errorf("CLOUDFLARE_API_TOKEN value = %v, want the Cloudflare deploy token's value", v)
	}
}

func TestExportsHaveTheExpectedNamesAndValues(t *testing.T) {
	_, off, err := runBuild(t, nil, nearExpiry)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := map[string]any{
		"zoneId":                "zone_id",
		"nameServers":           []string{"ada.ns.cloudflare.com", "bob.ns.cloudflare.com"},
		"pagesSubdomain":        "pages-project.pages.dev", // the mock derives it from the logical name
		"customDomainsEnabled":  false,
		"releaserTokenDaysLeft": 8,
		"customDomains":         []string{},
	}
	assertExports(t, off, want)

	_, on, err := runBuild(t, enable, nearExpiry)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want["customDomainsEnabled"] = true
	want["customDomains"] = []string{"elyclover.com", "dev.elyclover.com", "stg.elyclover.com"}
	assertExports(t, on, want)
}

func assertExports(t *testing.T, got, want map[string]any) {
	t.Helper()
	var names []string
	for k := range got {
		names = append(names, k)
	}
	sort.Strings(names)
	var wantNames []string
	for k := range want {
		wantNames = append(wantNames, k)
	}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("exports = %v, want exactly %v", names, wantNames)
	}
	for k, w := range want {
		if g, ok := got[k]; ok && !reflect.DeepEqual(g, w) {
			t.Errorf("export %s = %#v, want %#v", k, g, w)
		}
	}
}

func TestReleaserTokenDaysLeftTracksTheClock(t *testing.T) {
	cases := []struct {
		name string
		now  func() time.Time
		want int
	}{
		{"just under a year out (noon rounds the half day down)", farFromExpiry, 364},
		{"8 days out", nearExpiry, 8},
		{"already expired", clockAt(2027, time.October, 20), -10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got, err := runBuild(t, nil, tc.now)
			if err != nil {
				t.Fatalf("an expiring or expired token must only warn, not fail: %v", err)
			}
			if got["releaserTokenDaysLeft"] != tc.want {
				t.Errorf("releaserTokenDaysLeft = %v, want %d", got["releaserTokenDaysLeft"], tc.want)
			}
		})
	}
}

func TestExpiryWarning(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{365, ""},
		{30, ""},
		{29, "RELEASER_TOKEN expires in 29 days; see runbook"},
		{0, "RELEASER_TOKEN expires in 0 days; see runbook"},
		{-3, "RELEASER_TOKEN expires in -3 days; see runbook"},
	}
	for _, tc := range cases {
		if got := expiryWarning(tc.days); got != tc.want {
			t.Errorf("expiryWarning(%d) = %q, want %q", tc.days, got, tc.want)
		}
	}
}

func TestInvalidConfigFailsBeforeAnyResourceIsRegistered(t *testing.T) {
	cases := map[string]func(map[string]string){
		"bad account ID":   func(c map[string]string) { c[key+"cloudflareAccountId"] = "not-hex" },
		"bad expiry date":  func(c map[string]string) { c[key+"releaserTokenExpiresOn"] = "10/10/2027" },
		"missing releaser": func(c map[string]string) { delete(c, key+"releaserToken") },
		"missing cf token": func(c map[string]string) { delete(c, "cloudflare:apiToken") },
		"unsupported record": func(c map[string]string) {
			c[key+"emailRecords"] = `[{"type":"SRV","name":"elyclover.com","content":"x"}]`
		},
		"record outside zone": func(c map[string]string) {
			c[key+"emailRecords"] = `[{"type":"TXT","name":"example.org","content":"x"}]`
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m, err := runProgram(t, mutate, farFromExpiry)
			if err == nil {
				t.Fatal("program succeeded with an invalid configuration")
			}
			if got := userResources(m); len(got) != 0 {
				t.Errorf("%d resources were registered before the config error: %v", len(got), got)
			}
			if len(m.Calls) != 0 {
				t.Errorf("%d invokes were made before the config error", len(m.Calls))
			}
		})
	}
}

func TestInvalidConfigErrorsAreDescriptive(t *testing.T) {
	_, err := runProgram(t, func(c map[string]string) { c[key+"cloudflareAccountId"] = "short" }, farFromExpiry)
	if err == nil || !strings.Contains(err.Error(), "cloudflareAccountId") {
		t.Fatalf("error = %v, want one naming cloudflareAccountId", err)
	}

	_, err = runProgram(t, func(c map[string]string) { delete(c, key+"releaserToken") }, farFromExpiry)
	if err == nil || !strings.Contains(err.Error(), "releaserToken") {
		t.Fatalf("error = %v, want one naming releaserToken", err)
	}
}

func TestProgramEntrypointUsesTheRealClock(t *testing.T) {
	setup(t, nil)
	m := testutil.NewMocks()
	err := pulumi.RunErr(Program, pulumi.WithMocks(testutil.Project, testutil.Stack, m))
	if err != nil {
		t.Fatalf("Program: %v", err)
	}
	if len(m.ByType(testutil.TypeZone)) != 1 {
		t.Error("Program did not create the zone")
	}
}

func TestMissingPermissionGroupFailsTheProgramBeforeGitHub(t *testing.T) {
	m, err := runProgram(t, func(c map[string]string) {
		c[key+"pagesWritePermissionGroupName"] = "No Such Group"
	}, farFromExpiry)
	if err == nil || !strings.Contains(err.Error(), "No Such Group") {
		t.Fatalf("error = %v, want one naming the missing permission group", err)
	}
	for _, typ := range []string{testutil.TypeActionsSecret, testutil.TypeActionsVariable, testutil.TypeRepoEnvironment, testutil.TypeBranchProtection} {
		if n := len(m.ByType(typ)); n != 0 {
			t.Errorf("%d %s resources were created although the Cloudflare step failed", n, typ)
		}
	}
}

// failingGitHubLookup fails the repository lookup the branch protections need.
type failingGitHubLookup struct{ *testutil.Mocks }

func (f failingGitHubLookup) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if args.Token == testutil.CallGithubGetRepo {
		return nil, errors.New("simulated invoke failure")
	}
	return f.Mocks.Call(args)
}

func TestGitHubFailureFailsTheProgram(t *testing.T) {
	setup(t, nil)
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		return programWithClock(ctx, farFromExpiry)
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, failingGitHubLookup{testutil.NewMocks()}))
	if err == nil || !strings.Contains(err.Error(), "looking up repository") {
		t.Fatalf("error = %v, want one about the repository lookup", err)
	}
}
