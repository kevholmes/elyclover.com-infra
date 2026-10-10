package github_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"elyclover.com-infra/internal/config"
	"elyclover.com-infra/internal/github"
	"elyclover.com-infra/internal/testutil"
)

const (
	key       = testutil.Project + ":"
	accountID = "0123456789abcdef0123456789abcdef"
	cfToken   = "cloudflare-deploy-token-value"
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

// provisionWith runs github.Provision under the given mock monitor with the
// valid fixture config modified by mutate.
func provisionWith(t *testing.T, mon pulumi.MockResourceMonitor, mutate func(map[string]string)) error {
	t.Helper()
	cfgMap := testutil.ValidConfig()
	if mutate != nil {
		mutate(cfgMap)
	}
	testutil.SetConfig(t, cfgMap)

	return pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg, err := config.Load(ctx)
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		return github.Provision(ctx, cfg, pulumi.String(cfToken))
	}, pulumi.WithMocks(testutil.Project, testutil.Stack, mon))
}

func provision(t *testing.T, mutate func(map[string]string)) *testutil.Mocks {
	t.Helper()
	m := testutil.NewMocks()
	if err := provisionWith(t, m, mutate); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	return m
}

func mustResource(t *testing.T, m *testutil.Mocks, name string) pulumi.MockResourceArgs {
	t.Helper()
	r, ok := m.ByName(name)
	if !ok {
		t.Fatalf("resource %q was not registered", name)
	}
	return r
}

// input returns the named input with any secret wrapper removed, and whether it was secret.
func input(r pulumi.MockResourceArgs, field string) (v resource.PropertyValue, secret bool) {
	v = r.Inputs[resource.PropertyKey(field)]
	for v.IsSecret() {
		secret = true
		v = v.SecretValue().Element
	}
	return v, secret
}

func str(t *testing.T, r pulumi.MockResourceArgs, field string) string {
	t.Helper()
	v, _ := input(r, field)
	if !v.IsString() {
		t.Fatalf("%s: input %q = %v, want a string", r.Name, field, v)
	}
	return v.StringValue()
}

func boolean(t *testing.T, r pulumi.MockResourceArgs, field string) bool {
	t.Helper()
	v, _ := input(r, field)
	if !v.IsBool() {
		t.Fatalf("%s: input %q = %v, want a bool", r.Name, field, v)
	}
	return v.BoolValue()
}

func TestBothReposGetTheReleaserTokenAsASecret(t *testing.T) {
	m := provision(t, nil)

	for name, repo := range map[string]string{
		"site-secret-releaser-token":  "elyclover.com",
		"infra-secret-releaser-token": "elyclover.com-infra",
	} {
		r := mustResource(t, m, name)
		if r.TypeToken != testutil.TypeActionsSecret {
			t.Errorf("%s: type = %q", name, r.TypeToken)
		}
		if got := str(t, r, "repository"); got != repo {
			t.Errorf("%s: repository = %q, want %q", name, got, repo)
		}
		if got := str(t, r, "secretName"); got != "RELEASER_TOKEN" {
			t.Errorf("%s: secretName = %q", name, got)
		}
		if got := str(t, r, "value"); got != "test-releaser-token" {
			t.Errorf("%s: value = %q, want the configured releaserToken", name, got)
		}
		if _, secret := input(r, "value"); !secret {
			t.Errorf("%s: value must be a secret", name)
		}
		if _, ok := r.Inputs["plaintextValue"]; ok {
			t.Errorf("%s: deprecated plaintextValue must not be used", name)
		}
	}
}

func TestSiteGetsCloudflareCredentials(t *testing.T) {
	m := provision(t, nil)

	secret := mustResource(t, m, "site-secret-cloudflare-api-token")
	if got := str(t, secret, "repository"); got != "elyclover.com" {
		t.Errorf("repository = %q, want the site repo", got)
	}
	if got := str(t, secret, "secretName"); got != "CLOUDFLARE_API_TOKEN" {
		t.Errorf("secretName = %q", got)
	}
	if got := str(t, secret, "value"); got != cfToken {
		t.Errorf("value = %q, want the token passed to Provision", got)
	}
	if _, isSecret := input(secret, "value"); !isSecret {
		t.Error("the Cloudflare token must be stored as a secret")
	}

	v := mustResource(t, m, "site-var-cloudflare-account-id")
	if v.TypeToken != testutil.TypeActionsVariable {
		t.Errorf("type = %q", v.TypeToken)
	}
	if got := str(t, v, "repository"); got != "elyclover.com" {
		t.Errorf("variable repository = %q", got)
	}
	if got := str(t, v, "variableName"); got != "CLOUDFLARE_ACCOUNT_ID" {
		t.Errorf("variableName = %q", got)
	}
	if got := str(t, v, "value"); got != accountID {
		t.Errorf("variable value = %q, want the account ID", got)
	}

	if n := len(m.ByType(testutil.TypeActionsSecret)); n != 3 {
		t.Errorf("got %d Actions secrets, want exactly 3 (cloudflare token + 2 releaser tokens)", n)
	}
	if n := len(m.ByType(testutil.TypeActionsVariable)); n != 1 {
		t.Errorf("got %d Actions variables, want exactly 1", n)
	}
}

func TestSiteHasThreeEnvironments(t *testing.T) {
	m := provision(t, nil)

	envs := m.ByType(testutil.TypeRepoEnvironment)
	if len(envs) != 3 {
		t.Fatalf("got %d environments, want 3", len(envs))
	}
	want := map[string]string{"site-env-dev": "dev", "site-env-stg": "stg", "site-env-prod": "prod"}
	for _, e := range envs {
		env, ok := want[e.Name]
		if !ok {
			t.Errorf("unexpected environment resource %q", e.Name)
			continue
		}
		if got := str(t, e, "environment"); got != env {
			t.Errorf("%s: environment = %q, want %q", e.Name, got, env)
		}
		if got := str(t, e, "repository"); got != "elyclover.com" {
			t.Errorf("%s: repository = %q, want the site repo", e.Name, got)
		}
	}
}

func TestBranchProtectionsHaveExactContextsApprovalsAndRepoIDs(t *testing.T) {
	m := provision(t, nil)

	cases := []struct {
		name, repoID string
		contexts     []string
	}{
		{"site-main-protection", "R_elyclover.com", []string{"deploy", "GitGuardian Security Checks"}},
		{"infra-main-protection", "R_elyclover.com-infra", []string{"lint", "test", "GitGuardian Security Checks"}},
	}
	if n := len(m.ByType(testutil.TypeBranchProtection)); n != len(cases) {
		t.Errorf("got %d branch protections, want %d", n, len(cases))
	}
	for _, tc := range cases {
		r := mustResource(t, m, tc.name)
		if r.TypeToken != testutil.TypeBranchProtection {
			t.Errorf("%s: type = %q", tc.name, r.TypeToken)
		}
		if got := str(t, r, "repositoryId"); got != tc.repoID {
			t.Errorf("%s: repositoryId = %q, want the looked-up node ID %q", tc.name, got, tc.repoID)
		}
		if got := str(t, r, "pattern"); got != "main" {
			t.Errorf("%s: pattern = %q", tc.name, got)
		}

		checks := r.Inputs["requiredStatusChecks"].ArrayValue()
		if len(checks) != 1 {
			t.Fatalf("%s: got %d requiredStatusChecks blocks, want 1", tc.name, len(checks))
		}
		sc := checks[0].ObjectValue()
		if !sc["strict"].BoolValue() {
			t.Errorf("%s: strict must be true", tc.name)
		}
		var contexts []string
		for _, c := range sc["contexts"].ArrayValue() {
			contexts = append(contexts, c.StringValue())
		}
		if strings.Join(contexts, "|") != strings.Join(tc.contexts, "|") {
			t.Errorf("%s: contexts = %v, want %v", tc.name, contexts, tc.contexts)
		}

		reviews := r.Inputs["requiredPullRequestReviews"].ArrayValue()
		if len(reviews) != 1 {
			t.Fatalf("%s: got %d requiredPullRequestReviews blocks, want 1", tc.name, len(reviews))
		}
		count, ok := reviews[0].ObjectValue()["requiredApprovingReviewCount"]
		if !ok || !count.IsNumber() || count.NumberValue() != 0 {
			t.Errorf("%s: requiredApprovingReviewCount = %v, want 0", tc.name, count)
		}

		if !boolean(t, r, "requireConversationResolution") {
			t.Errorf("%s: requireConversationResolution must be true", tc.name)
		}
		for _, f := range []string{"enforceAdmins", "allowsForcePushes", "allowsDeletions"} {
			if boolean(t, r, f) {
				t.Errorf("%s: %s must be false", tc.name, f)
			}
		}
	}
}

func TestBranchProtectionContextsComeFromConfig(t *testing.T) {
	m := provision(t, func(c map[string]string) {
		c[key+"siteRequiredChecks"] = `["only-check"]`
		c[key+"infraRequiredChecks"] = `["x","y"]`
	})

	got := func(name string) string {
		r := mustResource(t, m, name)
		var out []string
		for _, c := range r.Inputs["requiredStatusChecks"].ArrayValue()[0].ObjectValue()["contexts"].ArrayValue() {
			out = append(out, c.StringValue())
		}
		return strings.Join(out, ",")
	}
	if g := got("site-main-protection"); g != "only-check" {
		t.Errorf("site contexts = %q", g)
	}
	if g := got("infra-main-protection"); g != "x,y" {
		t.Errorf("infra contexts = %q", g)
	}
}

func TestRepositoriesAreLookedUpByName(t *testing.T) {
	m := provision(t, nil)

	var names []string
	for _, c := range m.Calls {
		if c.Token == testutil.CallGithubGetRepo {
			names = append(names, c.Args["name"].StringValue())
		}
	}
	if strings.Join(names, ",") != "elyclover.com,elyclover.com-infra" {
		t.Errorf("repository lookups = %v, want site then infra", names)
	}
}

func TestRepositoryLookupFailureIsReturned(t *testing.T) {
	m := testutil.NewMocks()
	err := provisionWith(t, failingCall{Mocks: m, token: testutil.CallGithubGetRepo}, nil)
	if err == nil {
		t.Fatal("Provision succeeded although the repository lookup failed")
	}
	if !strings.Contains(err.Error(), "looking up repository elyclover.com") {
		t.Errorf("error %q does not name the repository", err)
	}
	if n := len(m.ByType(testutil.TypeBranchProtection)); n != 0 {
		t.Errorf("%d branch protections were created without a repository ID", n)
	}
}
