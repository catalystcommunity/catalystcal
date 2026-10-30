// Package config reads the catalystcal server configuration from the environment.
//
// Every setting has a CATALYSTCAL_ name. Load reads them once at startup and
// validates them, so a bad security setting stops the boot instead of
// failing later.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment names.
const (
	EnvDev     = "dev"
	EnvNonprod = "nonprod"
	EnvProd    = "prod"
)

// MinJWTSecretBytes is the shortest bearer secret accepted in prod.
const MinJWTSecretBytes = 32

// Config is the complete server configuration.
type Config struct {
	Env     string
	APIPort int
	OpsPort int
	DBPath  string

	JWTSecret   string
	PublicURL   string
	CallbackURL string

	DevAuthEnabled bool

	AllowedDomains []string
	AllowedUsers   []string

	BearerTTL  time.Duration
	SessionTTL time.Duration

	LinkkeysIDPDomain        string
	LinkkeysIDPURL           string
	LinkkeysRPDomain         string
	LinkkeysPKIAPIKey        string
	LinkkeysTCPAddr          string
	LinkkeysTCPFingerprints  []string
	LinkkeysTCPAllowInsecure bool

	SubscriptionRefresh      time.Duration
	SubscriptionAllowPrivate bool

	ActAsAppKeysFile        string
	ActAsMaxPresentationAge time.Duration
}

// Load reads the configuration from the process environment.
func Load() (*Config, error) {
	return LoadFrom(os.Getenv)
}

// LoadFrom reads the configuration through getenv. Tests pass a map.
func LoadFrom(getenv func(string) string) (*Config, error) {
	r := reader{getenv: getenv}
	c := &Config{
		Env:            strings.ToLower(r.str("CATALYSTCAL_ENV", EnvProd)),
		APIPort:        r.int("CATALYSTCAL_API_PORT", 7080),
		OpsPort:        r.int("CATALYSTCAL_OPS_PORT", 7081),
		DBPath:         r.str("CATALYSTCAL_DB_PATH", "./catalystcal.db"),
		JWTSecret:      getenv("CATALYSTCAL_JWT_SECRET"),
		PublicURL:      strings.TrimRight(r.str("CATALYSTCAL_PUBLIC_URL", ""), "/"),
		CallbackURL:    r.str("CATALYSTCAL_CALLBACK_URL", ""),
		DevAuthEnabled: r.bool("CATALYSTCAL_DEV_AUTH_ENABLED"),
		AllowedDomains: lowerAll(r.list("CATALYSTCAL_ALLOWED_DOMAINS")),
		AllowedUsers:   lowerDomains(r.list("CATALYSTCAL_ALLOWED_USERS")),
		BearerTTL:      time.Duration(r.int("CATALYSTCAL_BEARER_TTL_SECONDS", 43200)) * time.Second,
		SessionTTL:     time.Duration(r.int("CATALYSTCAL_SESSION_TTL_HOURS", 720)) * time.Hour,

		LinkkeysIDPDomain:        strings.ToLower(r.str("CATALYSTCAL_LINKKEYS_IDP_DOMAIN", "")),
		LinkkeysIDPURL:           strings.TrimRight(r.str("CATALYSTCAL_LINKKEYS_IDP_URL", ""), "/"),
		LinkkeysRPDomain:         strings.ToLower(r.str("CATALYSTCAL_LINKKEYS_RP_DOMAIN", "")),
		LinkkeysPKIAPIKey:        getenv("CATALYSTCAL_LINKKEYS_PKI_API_KEY"),
		LinkkeysTCPAddr:          r.str("CATALYSTCAL_LINKKEYS_TCP_ADDR", ""),
		LinkkeysTCPFingerprints:  r.list("CATALYSTCAL_LINKKEYS_TCP_FINGERPRINTS"),
		LinkkeysTCPAllowInsecure: r.bool("CATALYSTCAL_LINKKEYS_TCP_ALLOW_INSECURE"),

		SubscriptionRefresh:      time.Duration(r.int("CATALYSTCAL_SUBSCRIPTION_REFRESH_MINUTES", 30)) * time.Minute,
		SubscriptionAllowPrivate: r.bool("CATALYSTCAL_SUBSCRIPTION_ALLOW_PRIVATE"),

		ActAsAppKeysFile:        r.str("CATALYSTCAL_ACT_AS_APP_KEYS_FILE", ""),
		ActAsMaxPresentationAge: time.Duration(r.int("CATALYSTCAL_ACT_AS_MAX_PRESENTATION_AGE_SECONDS", 300)) * time.Second,
	}
	if c.CallbackURL == "" && c.PublicURL != "" {
		c.CallbackURL = c.PublicURL + "/auth/callback"
	}
	if r.err != nil {
		return nil, r.err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks the values that can stop a boot.
func (c *Config) Validate() error {
	switch c.Env {
	case EnvDev, EnvNonprod, EnvProd:
	default:
		return fmt.Errorf("CATALYSTCAL_ENV must be dev, nonprod, or prod (got %q)", c.Env)
	}
	if c.Env == EnvProd && len(c.JWTSecret) < MinJWTSecretBytes {
		return fmt.Errorf("CATALYSTCAL_JWT_SECRET must have at least %d bytes in prod", MinJWTSecretBytes)
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("CATALYSTCAL_JWT_SECRET is required")
	}
	if c.BearerTTL <= 0 {
		return fmt.Errorf("CATALYSTCAL_BEARER_TTL_SECONDS must be positive")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("CATALYSTCAL_SESSION_TTL_HOURS must be positive")
	}
	if c.SubscriptionRefresh <= 0 {
		return fmt.Errorf("CATALYSTCAL_SUBSCRIPTION_REFRESH_MINUTES must be positive")
	}
	if c.ActAsMaxPresentationAge <= 0 {
		return fmt.Errorf("CATALYSTCAL_ACT_AS_MAX_PRESENTATION_AGE_SECONDS must be positive")
	}
	return nil
}

// DevAuthAllowed is true only when dev auth is on and the environment is
// dev or nonprod. Prod never honors CATALYSTCAL_DEV_AUTH_ENABLED.
func (c *Config) DevAuthAllowed() bool {
	return c.DevAuthEnabled && (c.Env == EnvDev || c.Env == EnvNonprod)
}

// SecureCookies is false only in dev, where the SPA can run over plain http.
func (c *Config) SecureCookies() bool { return c.Env != EnvDev }

// RPDomain is the audience expected on LinkKeys assertions. It falls back to
// the IDP domain for a single-domain deployment.
func (c *Config) RPDomain() string {
	if c.LinkkeysRPDomain != "" {
		return c.LinkkeysRPDomain
	}
	return c.LinkkeysIDPDomain
}

// SignInAllowed applies CATALYSTCAL_ALLOWED_DOMAINS and CATALYSTCAL_ALLOWED_USERS. An
// empty domain list lets every domain sign in. The user list adds single
// people from other domains.
func (c *Config) SignInAllowed(domain, subject string) bool {
	if len(c.AllowedDomains) == 0 {
		return true
	}
	domain = strings.ToLower(domain)
	for _, d := range c.AllowedDomains {
		if d == domain {
			return true
		}
	}
	who := subject + "@" + domain
	for _, u := range c.AllowedUsers {
		if u == who {
			return true
		}
	}
	return false
}

// DomainMayStart reports whether a sign-in from domain can start. It is true
// when SignInAllowed can be true for some person of that domain. The
// callback applies the full check.
func (c *Config) DomainMayStart(domain string) bool {
	if len(c.AllowedDomains) == 0 {
		return true
	}
	domain = strings.ToLower(domain)
	for _, d := range c.AllowedDomains {
		if d == domain {
			return true
		}
	}
	for _, u := range c.AllowedUsers {
		if strings.HasSuffix(u, "@"+domain) {
			return true
		}
	}
	return false
}

type reader struct {
	getenv func(string) string
	err    error
}

func (r *reader) str(name, def string) string {
	if v := strings.TrimSpace(r.getenv(name)); v != "" {
		return v
	}
	return def
}

func (r *reader) int(name string, def int) int {
	v := strings.TrimSpace(r.getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("%s must be an integer (got %q)", name, v)
	}
	return n
}

func (r *reader) bool(name string) bool {
	v := strings.ToLower(strings.TrimSpace(r.getenv(name)))
	switch v {
	case "", "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	if r.err == nil {
		r.err = fmt.Errorf("%s must be true or false (got %q)", name, v)
	}
	return false
}

func (r *reader) list(name string) []string {
	var out []string
	for _, p := range strings.Split(r.getenv(name), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lowerAll(in []string) []string {
	for i := range in {
		in[i] = strings.ToLower(in[i])
	}
	return in
}

// lowerDomains lower-cases the domain part of subject@domain entries. The
// subject is an account id and keeps its case.
func lowerDomains(in []string) []string {
	for i, v := range in {
		if at := strings.LastIndex(v, "@"); at >= 0 {
			in[i] = v[:at] + "@" + strings.ToLower(v[at+1:])
		}
	}
	return in
}
