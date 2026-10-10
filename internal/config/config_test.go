package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"gopkg.in/yaml.v3"

	"elyclover.com-infra/internal/config"
	"elyclover.com-infra/internal/testutil"
)

const key = testutil.Project + ":"

// load runs config.Load inside a mocked Pulumi program with the given stack
// config. When the config loads, it also captures the plaintext and the
// secretness of ReleaserToken.
func load(t *testing.T, stackCfg map[string]string) (cfg *config.Config, token string, secret bool, err error) {
	t.Helper()
	testutil.SetConfig(t, stackCfg)

	tokenCh := make(chan string, 1)
	err = pulumi.RunErr(func(ctx *pulumi.Context) error {
		c, lerr := config.Load(ctx)
		if lerr != nil {
			return lerr
		}
		cfg = c
		secret = pulumi.IsSecret(c.ReleaserToken)
		c.ReleaserToken.ApplyT(func(v string) string {
			tokenCh <- v
			return v
		})
		return nil
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, testutil.NewMocks()))
	if err != nil {
		return nil, "", false, err
	}
	select {
	case token = <-tokenCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for ReleaserToken to resolve")
	}
	return cfg, token, secret, nil
}

func TestLoadValidConfig(t *testing.T) {
	cfg, token, secret, err := load(t, testutil.ValidConfig())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate on valid config: %v", err)
	}

	if cfg.CloudflareAccountID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("CloudflareAccountID = %q", cfg.CloudflareAccountID)
	}
	if cfg.Domain != "elyclover.com" || cfg.PagesProjectName != "elyclover" || cfg.PagesProductionBranch != "production" {
		t.Errorf("domain/project/branch = %q/%q/%q", cfg.Domain, cfg.PagesProjectName, cfg.PagesProductionBranch)
	}
	if got := strings.Join(cfg.PagesPreviewSubdomains, ","); got != "dev,stg" {
		t.Errorf("PagesPreviewSubdomains = %q", got)
	}
	if cfg.EnableCustomDomains {
		t.Error("EnableCustomDomains = true, want false")
	}
	if cfg.SiteRepo != "elyclover.com" || cfg.InfraRepo != "elyclover.com-infra" {
		t.Errorf("repos = %q/%q", cfg.SiteRepo, cfg.InfraRepo)
	}
	if got := strings.Join(cfg.SiteRequiredChecks, "|"); got != "deploy|GitGuardian Security Checks" {
		t.Errorf("SiteRequiredChecks = %q", got)
	}
	if got := strings.Join(cfg.InfraRequiredChecks, "|"); got != "lint|test|GitGuardian Security Checks" {
		t.Errorf("InfraRequiredChecks = %q", got)
	}
	if cfg.ReleaserTokenExpiresOn != "2027-10-10" || cfg.PagesWritePermissionGroupName != "Pages Write" {
		t.Errorf("expiry/group = %q/%q", cfg.ReleaserTokenExpiresOn, cfg.PagesWritePermissionGroupName)
	}

	if len(cfg.EmailRecords) != 5 {
		t.Fatalf("len(EmailRecords) = %d, want 5", len(cfg.EmailRecords))
	}
	mx := cfg.EmailRecords[1]
	if mx.Type != "MX" || mx.Name != "elyclover.com" || mx.Content != "mailsec.protonmail.ch" || mx.Priority == nil || *mx.Priority != 20 {
		t.Errorf("EmailRecords[1] = %+v", mx)
	}
	if cfg.EmailRecords[2].Priority != nil {
		t.Errorf("TXT record has priority %d, want none", *cfg.EmailRecords[2].Priority)
	}

	if token != "test-releaser-token" {
		t.Errorf("ReleaserToken = %q", token)
	}
	if !secret {
		t.Error("ReleaserToken is not marked secret")
	}
}

func TestLoadEnableCustomDomains(t *testing.T) {
	tests := []struct {
		name    string
		value   *string // nil = key absent
		want    bool
		wantErr string
	}{
		{name: "absent defaults to false", value: nil, want: false},
		{name: "true", value: ptr("true"), want: true},
		{name: "false", value: ptr("false"), want: false},
		{name: "garbage is an error, not false", value: ptr("maybe"), wantErr: "enableCustomDomains"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testutil.ValidConfig()
			delete(c, key+"enableCustomDomains")
			if tt.value != nil {
				c[key+"enableCustomDomains"] = *tt.value
			}
			cfg, _, _, err := load(t, c)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.EnableCustomDomains != tt.want {
				t.Errorf("EnableCustomDomains = %v, want %v", cfg.EnableCustomDomains, tt.want)
			}
		})
	}
}

func TestLoadMissingSecrets(t *testing.T) {
	tests := []struct {
		name    string
		drop    string
		wantErr string
	}{
		{name: "missing releaserToken", drop: key + "releaserToken", wantErr: "releaserToken"},
		{name: "missing cloudflare apiToken", drop: "cloudflare:apiToken", wantErr: "cloudflare:apiToken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testutil.ValidConfig()
			delete(c, tt.drop)
			_, _, _, err := load(t, c)
			if err == nil {
				t.Fatal("Load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "pulumi config set --secret") {
				t.Errorf("error = %q, want it to name %q and say how to set it", err, tt.wantErr)
			}
		})
	}
}

func TestLoadEmptyCloudflareTokenIsMissing(t *testing.T) {
	c := testutil.ValidConfig()
	c["cloudflare:apiToken"] = ""
	if _, _, _, err := load(t, c); err == nil || !strings.Contains(err.Error(), "cloudflare:apiToken") {
		t.Fatalf("error = %v, want it to name cloudflare:apiToken", err)
	}
}

func TestLoadRejectsMalformedStructuredConfig(t *testing.T) {
	for _, k := range []string{"pagesPreviewSubdomains", "siteRequiredChecks", "infraRequiredChecks", "emailRecords"} {
		t.Run(k, func(t *testing.T) {
			c := testutil.ValidConfig()
			c[key+k] = "{not json"
			_, _, _, err := load(t, c)
			if err == nil || !strings.Contains(err.Error(), k) {
				t.Fatalf("error = %v, want it to name %q", err, k)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c map[string]string)
		wantErr string // substring; empty = valid
	}{
		{name: "valid", mutate: func(map[string]string) {}},
		{
			name:    "empty account id",
			mutate:  func(c map[string]string) { c[key+"cloudflareAccountId"] = "" },
			wantErr: "cloudflareAccountId must not be empty",
		},
		{
			name:    "account id too short",
			mutate:  func(c map[string]string) { c[key+"cloudflareAccountId"] = "0123456789abcdef" },
			wantErr: "cloudflareAccountId must be 32 hex characters",
		},
		{
			name:    "account id not hex",
			mutate:  func(c map[string]string) { c[key+"cloudflareAccountId"] = strings.Repeat("g", 32) },
			wantErr: "cloudflareAccountId must be 32 hex characters",
		},
		{
			name:    "empty domain",
			mutate:  func(c map[string]string) { c[key+"domain"] = "" },
			wantErr: "domain must not be empty",
		},
		{
			name:    "empty project name",
			mutate:  func(c map[string]string) { c[key+"pagesProjectName"] = "" },
			wantErr: "pagesProjectName must not be empty",
		},
		{
			name: "email record type not allowed",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"A","name":"elyclover.com","content":"1.2.3.4"}]`
			},
			wantErr: `unsupported type "A"`,
		},
		{
			name: "MX without priority",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"MX","name":"elyclover.com","content":"mail.protonmail.ch"}]`
			},
			wantErr: "priority is required for MX",
		},
		{
			name: "MX with priority zero is allowed",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"MX","name":"elyclover.com","content":"mail.protonmail.ch","priority":0}]`
			},
		},
		{
			name: "record name outside the domain",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"TXT","name":"example.org","content":"x"}]`
			},
			wantErr: `name "example.org" must be "elyclover.com" or end with ".elyclover.com"`,
		},
		{
			name: "record name only shares a string suffix with the domain",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"TXT","name":"notelyclover.com","content":"x"}]`
			},
			wantErr: "must be",
		},
		{
			name: "record name with trailing dot",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"TXT","name":"elyclover.com.","content":"x"}]`
			},
			wantErr: "must be",
		},
		{
			name:    "expiry not a date",
			mutate:  func(c map[string]string) { c[key+"releaserTokenExpiresOn"] = "next year" },
			wantErr: "releaserTokenExpiresOn must be a YYYY-MM-DD date",
		},
		{
			name:    "expiry wrong format",
			mutate:  func(c map[string]string) { c[key+"releaserTokenExpiresOn"] = "10/10/2027" },
			wantErr: "releaserTokenExpiresOn must be a YYYY-MM-DD date",
		},
		{
			name:    "expiry missing",
			mutate:  func(c map[string]string) { delete(c, key+"releaserTokenExpiresOn") },
			wantErr: "releaserTokenExpiresOn must be a YYYY-MM-DD date",
		},
		{
			name:    "empty email records",
			mutate:  func(c map[string]string) { c[key+"emailRecords"] = `[]` },
			wantErr: "emailRecords must contain at least one record of type MX",
		},
		{
			name: "email records without an MX",
			mutate: func(c map[string]string) {
				c[key+"emailRecords"] = `[{"type":"TXT","name":"elyclover.com","content":"v=spf1 -all"},` +
					`{"type":"CNAME","name":"protonmail._domainkey.elyclover.com","content":"example.domains.proton.ch"}]`
			},
			wantErr: "emailRecords must contain at least one record of type MX",
		},
		{
			name:    "empty site repo",
			mutate:  func(c map[string]string) { c[key+"siteRepo"] = "" },
			wantErr: "siteRepo must not be empty",
		},
		{
			name:    "empty infra repo",
			mutate:  func(c map[string]string) { c[key+"infraRepo"] = "" },
			wantErr: "infraRepo must not be empty",
		},
		{
			name:    "empty site required checks",
			mutate:  func(c map[string]string) { c[key+"siteRequiredChecks"] = `[]` },
			wantErr: "siteRequiredChecks must not be empty",
		},
		{
			name:    "empty infra required checks",
			mutate:  func(c map[string]string) { c[key+"infraRequiredChecks"] = `[]` },
			wantErr: "infraRequiredChecks must not be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testutil.ValidConfig()
			tt.mutate(c)
			cfg, _, _, err := load(t, c)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			err = cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate: %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	c := testutil.ValidConfig()
	c[key+"cloudflareAccountId"] = ""
	c[key+"domain"] = ""
	c[key+"releaserTokenExpiresOn"] = "nope"
	cfg, _, _, err := load(t, c)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate succeeded, want errors")
	}
	for _, want := range []string{"cloudflareAccountId", "domain must not be empty", "releaserTokenExpiresOn"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate error %q does not mention %q", err, want)
		}
	}
}

// TestValidateRejectsMissingEmailRecordsKey covers the dangerous case behind
// the MX rule: a stack config without any emailRecords key loads fine (as an
// empty list), so Validate is the only thing standing between a bad edit and a
// pulumi up that deletes every mail record.
func TestValidateRejectsMissingEmailRecordsKey(t *testing.T) {
	c := testutil.ValidConfig()
	delete(c, key+"emailRecords")

	cfg, _, _, err := load(t, c)
	if err != nil {
		t.Fatalf("Load without emailRecords: %v, want it to load", err)
	}
	if len(cfg.EmailRecords) != 0 {
		t.Fatalf("len(EmailRecords) = %d, want 0 when the key is absent", len(cfg.EmailRecords))
	}

	const want = "emailRecords must contain at least one record of type MX"
	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate succeeded with no emailRecords key, want an error")
	}
	if err.Error() != want {
		t.Fatalf("Validate error = %q, want exactly %q", err, want)
	}
}

func TestDaysUntil(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		date    string
		now     time.Time
		want    int
		wantErr bool
	}{
		{name: "future", date: "2027-10-10", now: now, want: 365},
		{name: "today", date: "2026-10-10", now: now, want: 0},
		{name: "tomorrow", date: "2026-10-11", now: now, want: 1},
		{name: "past", date: "2026-10-05", now: now, want: -5},
		{name: "truncates toward zero like the pat-expiry workflow", date: "2026-10-11", now: now.Add(12 * time.Hour), want: 0},
		{name: "non-UTC now is normalised", date: "2026-10-11", now: time.Date(2026, 10, 9, 19, 0, 0, 0, time.FixedZone("UTC-5", -5*3600)), want: 1},
		{name: "bad format", date: "10-10-2027", now: now, wantErr: true},
		{name: "empty", date: "", now: now, wantErr: true},
		{name: "impossible date", date: "2027-02-30", now: now, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.DaysUntil(tt.date, tt.now)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DaysUntil(%q) = %d, want an error", tt.date, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DaysUntil(%q): %v", tt.date, err)
			}
			if got != tt.want {
				t.Errorf("DaysUntil(%q) = %d, want %d", tt.date, got, tt.want)
			}
		})
	}
}

func TestQuoteTXT(t *testing.T) {
	long := strings.Repeat("a", 255) + strings.Repeat("b", 255) + "ccc"
	exact := strings.Repeat("x", 255)
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "v=spf1 include:_spf.protonmail.ch ~all", want: `"v=spf1 include:_spf.protonmail.ch ~all"`},
		{name: "empty", in: "", want: `""`},
		{name: "already quoted is unchanged", in: `"v=DMARC1; p=quarantine"`, want: `"v=DMARC1; p=quarantine"`},
		{name: "already quoted and split is unchanged", in: `"abc" "def"`, want: `"abc" "def"`},
		{name: "exactly 255 stays one string", in: exact, want: `"` + exact + `"`},
		{
			name: "over 255 splits into 255-byte strings",
			in:   long,
			want: `"` + strings.Repeat("a", 255) + `" "` + strings.Repeat("b", 255) + `" "ccc"`,
		},
		{name: "embedded quote and backslash are escaped", in: `a"b\c`, want: `"a\"b\\c"`},
		{name: "single quote character is not 'already quoted'", in: `"`, want: `"\""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.QuoteTXT(tt.in); got != tt.want {
				t.Errorf("QuoteTXT(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestQuoteTXTNeverSplitsMultibyteCharacters(t *testing.T) {
	// 254 ASCII bytes then a 3-byte rune straddling the 255-byte boundary.
	in := strings.Repeat("a", 254) + "€" + "tail"
	got := config.QuoteTXT(in)
	parts := strings.Split(got, `" "`)
	if len(parts) != 2 {
		t.Fatalf("got %d strings, want 2: %q", len(parts), got)
	}
	first := strings.TrimPrefix(parts[0], `"`)
	if len(first) != 254 {
		t.Errorf("first chunk is %d bytes, want 254 (the euro sign moved whole to the next chunk)", len(first))
	}
	if !strings.HasPrefix(parts[1], "€tail") {
		t.Errorf("second chunk = %q, want it to start with the whole euro sign", parts[1])
	}
}

// TestProdStackConfig guards the committed Pulumi.prod.yaml against typos: it
// converts the YAML the way the Pulumi engine does (scalars as strings,
// structures as JSON) and requires the result to load and validate.
func TestProdStackConfig(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "Pulumi.prod.yaml"))
	if err != nil {
		t.Fatalf("read Pulumi.prod.yaml: %v", err)
	}
	var doc struct {
		Config map[string]any `yaml:"config"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse Pulumi.prod.yaml: %v", err)
	}
	if len(doc.Config) == 0 {
		t.Fatal("Pulumi.prod.yaml has no config")
	}

	stackCfg := map[string]string{
		// Secrets are added by `pulumi config set --secret` and are not in the file.
		"cloudflare:apiToken": "test-cloudflare-api-token",
		key + "releaserToken": "test-releaser-token",
	}
	for k, v := range doc.Config {
		switch val := v.(type) {
		case string:
			stackCfg[k] = val
		case bool, int, float64:
			b, _ := json.Marshal(val)
			stackCfg[k] = string(b)
		default:
			b, err := json.Marshal(val)
			if err != nil {
				t.Fatalf("marshal %s: %v", k, err)
			}
			stackCfg[k] = string(b)
		}
	}

	cfg, _, _, err := load(t, stackCfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Pulumi.prod.yaml does not validate: %v", err)
	}
	if got := strings.Join(cfg.PagesPreviewSubdomains, ","); got != "dev,stg" {
		t.Errorf("PagesPreviewSubdomains = %q, want dev,stg", got)
	}
	if got := len(cfg.EmailRecords); got != 9 {
		t.Errorf("len(EmailRecords) = %d, want 9 (2 MX, 3 TXT, 3 DKIM CNAME, 1 DMARC)", got)
	}
}

func ptr[T any](v T) *T { return &v }
