package cloudflare_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/cloudflare"
	"elyclover.com-infra/internal/config"
	"elyclover.com-infra/internal/testutil"
)

const (
	key       = testutil.Project + ":"
	accountID = "0123456789abcdef0123456789abcdef"
)

// failingCall wraps the recording mocks and fails one invoke token.
type failingCall struct {
	*testutil.Mocks
	token string
}

func (f failingCall) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if args.Token == f.token {
		return nil, errors.New("simulated invoke failure")
	}
	return f.Mocks.Call(args)
}

// provisionWith runs cloudflare.Provision under the given mock monitor with the
// valid fixture config modified by mutate. It returns Provision's result.
func provisionWith(t *testing.T, mon pulumi.MockResourceMonitor, mutate func(map[string]string)) (*cloudflare.Result, error) {
	t.Helper()
	cfgMap := testutil.ValidConfig()
	if mutate != nil {
		mutate(cfgMap)
	}
	testutil.SetConfig(t, cfgMap)

	var res *cloudflare.Result
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg, err := config.Load(ctx)
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		res, err = cloudflare.Provision(ctx, cfg)
		return err
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, mon))
	return res, err
}

func provision(t *testing.T, mutate func(map[string]string)) (*testutil.Mocks, *cloudflare.Result) {
	t.Helper()
	m := testutil.NewMocks()
	res, err := provisionWith(t, m, mutate)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	return m, res
}

func enableCustomDomains(cfg map[string]string) { cfg[key+"enableCustomDomains"] = "true" }

func mustResource(t *testing.T, m *testutil.Mocks, name string) pulumi.MockResourceArgs {
	t.Helper()
	r, ok := m.ByName(name)
	if !ok {
		t.Fatalf("resource %q was not registered", name)
	}
	return r
}

func str(t *testing.T, r pulumi.MockResourceArgs, field string) string {
	t.Helper()
	v, ok := r.Inputs[resource.PropertyKey(field)]
	if !ok || !v.IsString() {
		t.Fatalf("%s: input %q = %v, want a string", r.Name, field, v)
	}
	return v.StringValue()
}

func num(t *testing.T, r pulumi.MockResourceArgs, field string) float64 {
	t.Helper()
	v, ok := r.Inputs[resource.PropertyKey(field)]
	if !ok || !v.IsNumber() {
		t.Fatalf("%s: input %q = %v, want a number", r.Name, field, v)
	}
	return v.NumberValue()
}

func boolean(t *testing.T, r pulumi.MockResourceArgs, field string) bool {
	t.Helper()
	v, ok := r.Inputs[resource.PropertyKey(field)]
	if !ok || !v.IsBool() {
		t.Fatalf("%s: input %q = %v, want a bool", r.Name, field, v)
	}
	return v.BoolValue()
}

func dependsOn(r pulumi.MockResourceArgs, logicalName string) bool {
	for _, urn := range r.RegisterRPC.GetDependencies() {
		if strings.HasSuffix(urn, "::"+logicalName) {
			return true
		}
	}
	return false
}

func TestZoneIsProtectedFullZoneInTheRightAccount(t *testing.T) {
	m, res := provision(t, nil)

	zones := m.ByType(testutil.TypeZone)
	if len(zones) != 1 {
		t.Fatalf("got %d zones, want 1", len(zones))
	}
	z := zones[0]
	if z.Name != "zone" {
		t.Errorf("logical name = %q, want zone", z.Name)
	}
	if !z.RegisterRPC.GetProtect() {
		t.Error("zone must be registered with protect=true")
	}
	if got := str(t, z, "name"); got != "elyclover.com" {
		t.Errorf("zone name = %q", got)
	}
	if got := str(t, z, "type"); got != "full" {
		t.Errorf("zone type = %q, want full", got)
	}
	account := z.Inputs["account"]
	if !account.IsObject() || account.ObjectValue()["id"].StringValue() != accountID {
		t.Errorf("zone account = %v, want an object with id %s", account, accountID)
	}
	if res.Zone == nil {
		t.Error("Result.Zone is nil")
	}
}

func TestZoneSettingsHaveExactIDsAndValues(t *testing.T) {
	m, _ := provision(t, nil)

	want := map[string][2]string{
		"zone-setting-ssl":              {"ssl", "strict"},
		"zone-setting-always-use-https": {"always_use_https", "on"},
		"zone-setting-min-tls-version":  {"min_tls_version", "1.2"},
	}
	settings := m.ByType(testutil.TypeZoneSetting)
	if len(settings) != len(want) {
		t.Fatalf("got %d zone settings, want %d", len(settings), len(want))
	}
	for _, s := range settings {
		w, ok := want[s.Name]
		if !ok {
			t.Errorf("unexpected zone setting %q", s.Name)
			continue
		}
		if got := str(t, s, "settingId"); got != w[0] {
			t.Errorf("%s: settingId = %q, want %q", s.Name, got, w[0])
		}
		if got := str(t, s, "value"); got != w[1] {
			t.Errorf("%s: value = %q, want %q", s.Name, got, w[1])
		}
		if got := str(t, s, "zoneId"); got != "zone_id" {
			t.Errorf("%s: zoneId = %q, want the zone's ID", s.Name, got)
		}
	}
}

func TestEmailRecordsOnePerConfiguredRecord(t *testing.T) {
	m, _ := provision(t, nil)

	records := m.ByType(testutil.TypeDNSRecord)
	if len(records) != 5 {
		t.Fatalf("got %d DNS records, want 5 (the fixture's email records): %v", len(records), records)
	}

	want := []struct {
		name, typ, dnsName, content string
		priority                    *float64
	}{
		{"dns-email-0-mx", "MX", "elyclover.com", "mail.protonmail.ch", ptr(10)},
		{"dns-email-1-mx", "MX", "elyclover.com", "mailsec.protonmail.ch", ptr(20)},
		{"dns-email-2-txt", "TXT", "elyclover.com", `"v=spf1 include:_spf.protonmail.ch ~all"`, nil},
		{"dns-email-3-cname", "CNAME", "protonmail._domainkey.elyclover.com", "protonmail.domainkey.example.domains.proton.ch", nil},
		{"dns-email-4-txt", "TXT", "_dmarc.elyclover.com", `"v=DMARC1; p=quarantine"`, nil},
	}
	for _, w := range want {
		r := mustResource(t, m, w.name)
		if r.TypeToken != testutil.TypeDNSRecord {
			t.Errorf("%s: type = %q", w.name, r.TypeToken)
		}
		if got := str(t, r, "type"); got != w.typ {
			t.Errorf("%s: type = %q, want %q", w.name, got, w.typ)
		}
		if got := str(t, r, "name"); got != w.dnsName {
			t.Errorf("%s: name = %q, want %q", w.name, got, w.dnsName)
		}
		if got := str(t, r, "content"); got != w.content {
			t.Errorf("%s: content = %q, want %q", w.name, got, w.content)
		}
		if got := str(t, r, "zoneId"); got != "zone_id" {
			t.Errorf("%s: zoneId = %q", w.name, got)
		}
		if boolean(t, r, "proxied") {
			t.Errorf("%s: email records must not be proxied", w.name)
		}
		if got := num(t, r, "ttl"); got != 1 {
			t.Errorf("%s: ttl = %v, want 1 (automatic)", w.name, got)
		}
		_, hasPriority := r.Inputs["priority"]
		switch {
		case w.priority == nil && hasPriority:
			t.Errorf("%s: non-MX record must not set a priority", w.name)
		case w.priority != nil:
			if !hasPriority {
				t.Errorf("%s: MX record has no priority", w.name)
			} else if got := num(t, r, "priority"); got != *w.priority {
				t.Errorf("%s: priority = %v, want %v", w.name, got, *w.priority)
			}
		}
	}
}

func ptr(v float64) *float64 { return &v }

func TestLongTXTRecordIsSplitIntoQuotedChunks(t *testing.T) {
	long := strings.Repeat("a", 300)
	records := fmt.Sprintf(`[{"type":"TXT","name":"elyclover.com","content":%q}]`, long)
	m, _ := provision(t, func(c map[string]string) { c[key+"emailRecords"] = records })

	r := mustResource(t, m, "dns-email-0-txt")
	want := `"` + long[:255] + `" "` + long[255:] + `"`
	if got := str(t, r, "content"); got != want {
		t.Errorf("content = %q, want two quoted chunks of 255 and 45 characters", got)
	}
}

func TestPagesProjectIsDirectUpload(t *testing.T) {
	m, res := provision(t, nil)

	projects := m.ByType(testutil.TypePagesProject)
	if len(projects) != 1 {
		t.Fatalf("got %d pages projects, want 1", len(projects))
	}
	p := projects[0]
	if p.Name != "pages-project" {
		t.Errorf("logical name = %q, want pages-project", p.Name)
	}
	if got := str(t, p, "accountId"); got != accountID {
		t.Errorf("accountId = %q", got)
	}
	if got := str(t, p, "name"); got != "elyclover" {
		t.Errorf("name = %q", got)
	}
	if got := str(t, p, "productionBranch"); got != "production" {
		t.Errorf("productionBranch = %q", got)
	}
	if _, ok := p.Inputs["source"]; ok {
		t.Error("a git source would turn off Direct Upload; source must be unset")
	}
	if res.Project == nil {
		t.Error("Result.Project is nil")
	}
}

func TestCustomDomainsDisabledCreatesNothing(t *testing.T) {
	m, res := provision(t, nil)

	if n := len(m.ByType(testutil.TypePagesDomain)); n != 0 {
		t.Errorf("got %d pages domains with enableCustomDomains=false, want 0", n)
	}
	for _, r := range m.Resources {
		if strings.HasPrefix(r.Name, "dns-pages-") {
			t.Errorf("unexpected pages CNAME %q with enableCustomDomains=false", r.Name)
		}
	}
	if len(res.CustomDomains) != 0 {
		t.Errorf("CustomDomains = %v, want empty", res.CustomDomains)
	}
}

func TestCustomDomainsEnabledCreatesDomainsAndCNAMEs(t *testing.T) {
	m, res := provision(t, enableCustomDomains)

	wantDomains := map[string]string{ // logical name -> custom domain
		"pages-domain-apex": "elyclover.com",
		"pages-domain-dev":  "dev.elyclover.com",
		"pages-domain-stg":  "stg.elyclover.com",
	}
	domains := m.ByType(testutil.TypePagesDomain)
	if len(domains) != len(wantDomains) {
		t.Fatalf("got %d pages domains, want %d", len(domains), len(wantDomains))
	}
	for _, d := range domains {
		want, ok := wantDomains[d.Name]
		if !ok {
			t.Errorf("unexpected pages domain %q", d.Name)
			continue
		}
		if got := str(t, d, "name"); got != want {
			t.Errorf("%s: name = %q, want %q", d.Name, got, want)
		}
		if got := str(t, d, "projectName"); got != "elyclover" {
			t.Errorf("%s: projectName = %q", d.Name, got)
		}
		if got := str(t, d, "accountId"); got != accountID {
			t.Errorf("%s: accountId = %q", d.Name, got)
		}
		if !dependsOn(d, "pages-project") {
			t.Errorf("%s must depend on the pages project", d.Name)
		}
	}

	wantCNAMEs := map[string][2]string{ // logical name -> {record name, target}
		"dns-pages-apex": {"elyclover.com", "elyclover.pages.dev"},
		"dns-pages-dev":  {"dev.elyclover.com", "dev.elyclover.pages.dev"},
		"dns-pages-stg":  {"stg.elyclover.com", "stg.elyclover.pages.dev"},
	}
	for name, w := range wantCNAMEs {
		r := mustResource(t, m, name)
		if r.TypeToken != testutil.TypeDNSRecord {
			t.Errorf("%s: type = %q", name, r.TypeToken)
		}
		if got := str(t, r, "type"); got != "CNAME" {
			t.Errorf("%s: type = %q, want CNAME", name, got)
		}
		if got := str(t, r, "name"); got != w[0] {
			t.Errorf("%s: name = %q, want %q", name, got, w[0])
		}
		if got := str(t, r, "content"); got != w[1] {
			t.Errorf("%s: content = %q, want %q", name, got, w[1])
		}
		if !boolean(t, r, "proxied") {
			t.Errorf("%s: pages CNAMEs must be proxied", name)
		}
		if got := num(t, r, "ttl"); got != 1 {
			t.Errorf("%s: ttl = %v, want 1", name, got)
		}
		if got := str(t, r, "zoneId"); got != "zone_id" {
			t.Errorf("%s: zoneId = %q", name, got)
		}
		domainName := strings.Replace(name, "dns-pages-", "pages-domain-", 1)
		if !dependsOn(r, domainName) {
			t.Errorf("%s must depend on %s", name, domainName)
		}
	}

	if got := strings.Join(res.CustomDomains, ","); got != "elyclover.com,dev.elyclover.com,stg.elyclover.com" {
		t.Errorf("CustomDomains = %q", got)
	}
}

func TestCustomDomainsFollowPreviewSubdomains(t *testing.T) {
	m, res := provision(t, func(c map[string]string) {
		enableCustomDomains(c)
		c[key+"pagesPreviewSubdomains"] = `["qa"]`
	})

	if n := len(m.ByType(testutil.TypePagesDomain)); n != 2 {
		t.Errorf("got %d pages domains, want 2 (apex + qa)", n)
	}
	r := mustResource(t, m, "dns-pages-qa")
	if got := str(t, r, "content"); got != "qa.elyclover.pages.dev" {
		t.Errorf("qa target = %q", got)
	}
	if got := strings.Join(res.CustomDomains, ","); got != "elyclover.com,qa.elyclover.com" {
		t.Errorf("CustomDomains = %q", got)
	}
}

func TestDeployTokenUsesPagesWriteScopedToThisAccount(t *testing.T) {
	m, res := provision(t, nil)

	tokens := m.ByType(testutil.TypeAccountToken)
	if len(tokens) != 1 {
		t.Fatalf("got %d account tokens, want 1", len(tokens))
	}
	tok := tokens[0]
	if tok.Name != "ci-deploy-token" {
		t.Errorf("logical name = %q, want ci-deploy-token", tok.Name)
	}
	if got := str(t, tok, "name"); got != "github-actions-elyclover-pages-deploy" {
		t.Errorf("token name = %q", got)
	}
	if got := str(t, tok, "accountId"); got != accountID {
		t.Errorf("accountId = %q", got)
	}

	policies := tok.Inputs["policies"]
	if !policies.IsArray() || len(policies.ArrayValue()) != 1 {
		t.Fatalf("policies = %v, want exactly one policy", policies)
	}
	policy := policies.ArrayValue()[0].ObjectValue()
	if got := policy["effect"].StringValue(); got != "allow" {
		t.Errorf("policy effect = %q, want allow", got)
	}
	groups := policy["permissionGroups"].ArrayValue()
	if len(groups) != 1 || groups[0].ObjectValue()["id"].StringValue() != "pages-write-id" {
		t.Errorf("permissionGroups = %v, want exactly [{id: pages-write-id}]", policy["permissionGroups"])
	}

	var resources map[string]string
	if err := json.Unmarshal([]byte(policy["resources"].StringValue()), &resources); err != nil {
		t.Fatalf("policy resources is not a JSON object string: %v", err)
	}
	want := map[string]string{"com.cloudflare.api.account." + accountID: "*"}
	if len(resources) != 1 || resources["com.cloudflare.api.account."+accountID] != "*" {
		t.Errorf("policy resources = %v, want %v", resources, want)
	}
	if res.Token == nil {
		t.Error("Result.Token is nil")
	}
}

func TestPermissionGroupLookupFiltersClientSide(t *testing.T) {
	m, _ := provision(t, nil)

	var calls int
	for _, c := range m.Calls {
		if c.Token != testutil.CallPermissionGroups {
			continue
		}
		calls++
		if v := c.Args["accountId"]; !v.IsString() || v.StringValue() != accountID {
			t.Errorf("lookup accountId = %v, want %s", v, accountID)
		}
		if _, ok := c.Args["name"]; ok {
			t.Error("the invoke's own name filter must not be used; filter client-side")
		}
	}
	if calls != 1 {
		t.Errorf("permission groups looked up %d times, want 1", calls)
	}
}

func TestMissingPermissionGroupIsAnErrorNamingTheGroup(t *testing.T) {
	m := testutil.NewMocks()
	_, err := provisionWith(t, m, func(c map[string]string) {
		c[key+"pagesWritePermissionGroupName"] = "No Such Group"
	})
	if err == nil {
		t.Fatal("Provision succeeded with an unknown permission group")
	}
	if !strings.Contains(err.Error(), "No Such Group") {
		t.Errorf("error %q does not name the missing group", err)
	}
	if n := len(m.ByType(testutil.TypeAccountToken)); n != 0 {
		t.Errorf("%d tokens were created despite the missing group", n)
	}
}

func TestPermissionGroupLookupFailureIsReturned(t *testing.T) {
	m := testutil.NewMocks()
	_, err := provisionWith(t, failingCall{Mocks: m, token: testutil.CallPermissionGroups}, nil)
	if err == nil || !strings.Contains(err.Error(), "permission groups") {
		t.Fatalf("error = %v, want one about listing permission groups", err)
	}
	if n := len(m.ByType(testutil.TypeAccountToken)); n != 0 {
		t.Errorf("%d tokens were created despite the failed lookup", n)
	}
}

func TestEmailRecordsRejectsMXWithoutPriority(t *testing.T) {
	// Validate normally catches this; NewEmailRecords must not silently emit an
	// MX record with no priority if it is handed an unvalidated config.
	cfg := &config.Config{
		CloudflareAccountID: accountID,
		Domain:              "elyclover.com",
		EmailRecords:        []config.EmailRecord{{Type: "MX", Name: "elyclover.com", Content: "mail.example.com"}},
	}
	m := testutil.NewMocks()
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		zone, err := cloudflare.NewZone(ctx, cfg)
		if err != nil {
			return err
		}
		return cloudflare.NewEmailRecords(ctx, cfg, zone)
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, m))
	if err == nil || !strings.Contains(err.Error(), "no priority") {
		t.Fatalf("error = %v, want one about the missing MX priority", err)
	}
	if n := len(m.ByType(testutil.TypeDNSRecord)); n != 0 {
		t.Errorf("%d DNS records were created for an invalid MX record", n)
	}
}
