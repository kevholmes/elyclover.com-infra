// Package config loads and validates the typed stack configuration.
//
// Keys live in the project namespace (elyclover.com-infra:<key>). Two secrets
// are required as well: elyclover.com-infra:releaserToken (read with
// TrySecret) and cloudflare:apiToken (read by the Cloudflare provider itself;
// only its presence is checked here).
package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pconfig "github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const (
	// KeyReleaserToken is the secret config key holding the RELEASER_TOKEN value.
	KeyReleaserToken = "releaserToken"
	// KeyCloudflareAPIToken is the provider-namespaced secret config key for the
	// Cloudflare bootstrap token.
	KeyCloudflareAPIToken = "cloudflare:apiToken"

	// DateLayout is the only accepted format for releaserTokenExpiresOn.
	DateLayout = "2006-01-02"

	// maxTXTChunk is the longest single character-string a TXT record may hold.
	maxTXTChunk = 255
)

var accountIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// EmailRecord is one DNS record carried over from the old zone (email and
// verification records). Names are fully qualified without a trailing dot.
// TXT contents are unquoted; use QuoteTXT before sending them to Cloudflare.
type EmailRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	// Priority is required for MX records and ignored for the other types.
	Priority *int `json:"priority,omitempty"`
}

// Config is the typed view of the stack configuration.
type Config struct {
	CloudflareAccountID           string
	Domain                        string
	PagesProjectName              string
	PagesProductionBranch         string
	PagesPreviewSubdomains        []string
	EnableCustomDomains           bool
	SiteRepo                      string
	InfraRepo                     string
	SiteRequiredChecks            []string
	InfraRequiredChecks           []string
	ReleaserTokenExpiresOn        string
	PagesWritePermissionGroupName string
	EmailRecords                  []EmailRecord

	// ReleaserToken is the secret value of RELEASER_TOKEN, kept secret end to end.
	ReleaserToken pulumi.StringOutput
}

// Load reads the stack configuration. It does not validate values: call
// Validate afterwards. It fails with a clear error when a required secret
// (releaserToken, cloudflare:apiToken) is missing, or when a structured or
// boolean value cannot be parsed.
func Load(ctx *pulumi.Context) (*Config, error) {
	cfg := pconfig.New(ctx, "")

	c := &Config{
		CloudflareAccountID:           cfg.Get("cloudflareAccountId"),
		Domain:                        cfg.Get("domain"),
		PagesProjectName:              cfg.Get("pagesProjectName"),
		PagesProductionBranch:         cfg.Get("pagesProductionBranch"),
		SiteRepo:                      cfg.Get("siteRepo"),
		InfraRepo:                     cfg.Get("infraRepo"),
		ReleaserTokenExpiresOn:        cfg.Get("releaserTokenExpiresOn"),
		PagesWritePermissionGroupName: cfg.Get("pagesWritePermissionGroupName"),
	}

	structured := []struct {
		key string
		dst any
	}{
		{"pagesPreviewSubdomains", &c.PagesPreviewSubdomains},
		{"siteRequiredChecks", &c.SiteRequiredChecks},
		{"infraRequiredChecks", &c.InfraRequiredChecks},
		{"emailRecords", &c.EmailRecords},
	}
	for _, s := range structured {
		if err := cfg.GetObject(s.key, s.dst); err != nil {
			return nil, fmt.Errorf("config %q is not valid structured config: %w", s.key, err)
		}
	}

	// enableCustomDomains gates the site going live; absent means off, but a
	// malformed value is an error rather than a silent "false".
	if raw, err := cfg.Try("enableCustomDomains"); err == nil {
		enabled, perr := strconv.ParseBool(strings.TrimSpace(raw))
		if perr != nil {
			return nil, fmt.Errorf("config %q must be true or false, got %q", "enableCustomDomains", raw)
		}
		c.EnableCustomDomains = enabled
	}

	// The provider reads cloudflare:apiToken itself; fail early, with a clear
	// message, instead of a late provider authentication error.
	if tok, err := pconfig.Try(ctx, KeyCloudflareAPIToken); err != nil || tok == "" {
		return nil, fmt.Errorf("missing required secret %q: run `pulumi config set --secret %s <token>`",
			KeyCloudflareAPIToken, KeyCloudflareAPIToken)
	}

	// TrySecret returns an error when the key is missing (RequireSecret would panic).
	tok, err := cfg.TrySecret(KeyReleaserToken)
	if err != nil {
		return nil, fmt.Errorf("missing required secret %q: run `pulumi config set --secret %s <token>`: %w",
			KeyReleaserToken, KeyReleaserToken, err)
	}
	c.ReleaserToken = tok

	return c, nil
}

// Validate reports every problem found in the configuration, joined into one error.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	switch {
	case c.CloudflareAccountID == "":
		add("cloudflareAccountId must not be empty")
	case !accountIDPattern.MatchString(c.CloudflareAccountID):
		add("cloudflareAccountId must be 32 hex characters")
	}
	if c.Domain == "" {
		add("domain must not be empty")
	}
	if c.PagesProjectName == "" {
		add("pagesProjectName must not be empty")
	}

	for i, r := range c.EmailRecords {
		switch r.Type {
		case "MX":
			if r.Priority == nil {
				add("emailRecords[%d] (MX %s): priority is required for MX records", i, r.Name)
			}
		case "TXT", "CNAME":
		default:
			add("emailRecords[%d]: unsupported type %q (want MX, TXT or CNAME)", i, r.Type)
		}
		if c.Domain != "" && r.Name != c.Domain && !strings.HasSuffix(r.Name, "."+c.Domain) {
			add("emailRecords[%d]: name %q must be %q or end with %q", i, r.Name, c.Domain, "."+c.Domain)
		}
	}

	if _, err := time.Parse(DateLayout, c.ReleaserTokenExpiresOn); err != nil {
		add("releaserTokenExpiresOn must be a YYYY-MM-DD date, got %q", c.ReleaserTokenExpiresOn)
	}

	return errors.Join(errs...)
}

// DaysUntil returns the number of whole days from now until the given
// YYYY-MM-DD date (midnight UTC), negative when the date has passed. It
// truncates toward zero, exactly like the shell arithmetic in the pat-expiry
// workflow, so the two checks agree.
func DaysUntil(date string, now time.Time) (int, error) {
	target, err := time.Parse(DateLayout, date)
	if err != nil {
		return 0, fmt.Errorf("invalid date %q, want YYYY-MM-DD: %w", date, err)
	}
	return int(target.Sub(now.UTC()) / (24 * time.Hour)), nil
}

// QuoteTXT formats a TXT record value for Cloudflare: wrapped in double
// quotes, with values longer than 255 bytes split into several quoted strings
// separated by spaces. A value that is already quoted is returned unchanged.
// Backslashes and double quotes inside an unquoted value are escaped.
func QuoteTXT(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s
	}

	var chunks []string
	for len(s) > maxTXTChunk {
		cut := maxTXTChunk
		for cut > 0 && !utf8.RuneStart(s[cut]) { // never split a multi-byte character
			cut--
		}
		chunks = append(chunks, s[:cut])
		s = s[cut:]
	}
	chunks = append(chunks, s)

	escaper := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	for i, ch := range chunks {
		chunks[i] = `"` + escaper.Replace(ch) + `"`
	}
	return strings.Join(chunks, " ")
}
