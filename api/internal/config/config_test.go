package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := LoadFrom(env(map[string]string{"CATALYSTCAL_JWT_SECRET": strings.Repeat("x", 32), "CATALYSTCAL_PUBLIC_URL": "https://catalystcal.example/"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != EnvProd || c.APIPort != 7080 || c.OpsPort != 7081 || c.DBPath != "./catalystcal.db" {
		t.Fatalf("defaults: %+v", c)
	}
	if c.CallbackURL != "https://catalystcal.example/auth/callback" {
		t.Fatalf("callback %q", c.CallbackURL)
	}
	if c.BearerTTL != 12*time.Hour || c.SessionTTL != 720*time.Hour || c.SubscriptionRefresh != 30*time.Minute || c.ActAsMaxPresentationAge != 5*time.Minute {
		t.Fatalf("durations: %+v", c)
	}
	if c.DevAuthAllowed() || !c.SecureCookies() || c.SubscriptionAllowPrivate {
		t.Fatal("prod defaults are not safe")
	}
}

func TestProdSecret(t *testing.T) {
	if _, err := LoadFrom(env(map[string]string{})); err == nil {
		t.Fatal("empty secret accepted in prod")
	}
	if _, err := LoadFrom(env(map[string]string{"CATALYSTCAL_JWT_SECRET": "short"})); err == nil {
		t.Fatal("short secret accepted in prod")
	}
	if _, err := LoadFrom(env(map[string]string{"CATALYSTCAL_ENV": "dev", "CATALYSTCAL_JWT_SECRET": "short"})); err != nil {
		t.Fatalf("dev short secret: %v", err)
	}
	if _, err := LoadFrom(env(map[string]string{"CATALYSTCAL_ENV": "dev"})); err == nil {
		t.Fatal("empty secret accepted in dev")
	}
}

func TestDevAuthGate(t *testing.T) {
	for envName, want := range map[string]bool{"dev": true, "nonprod": true, "prod": false} {
		c, err := LoadFrom(env(map[string]string{"CATALYSTCAL_ENV": envName, "CATALYSTCAL_JWT_SECRET": strings.Repeat("x", 32), "CATALYSTCAL_DEV_AUTH_ENABLED": "true"}))
		if err != nil {
			t.Fatal(err)
		}
		if c.DevAuthAllowed() != want {
			t.Errorf("%s: dev auth %v", envName, c.DevAuthAllowed())
		}
	}
}

func TestBadValues(t *testing.T) {
	for k, v := range map[string]string{"CATALYSTCAL_API_PORT": "x", "CATALYSTCAL_ENV": "staging", "CATALYSTCAL_DEV_AUTH_ENABLED": "maybe", "CATALYSTCAL_BEARER_TTL_SECONDS": "0"} {
		m := map[string]string{"CATALYSTCAL_JWT_SECRET": strings.Repeat("x", 32), k: v}
		if _, err := LoadFrom(env(m)); err == nil {
			t.Errorf("%s=%s accepted", k, v)
		}
	}
}

func TestSignInAllowed(t *testing.T) {
	c := &Config{}
	if !c.SignInAllowed("any.example", "u") {
		t.Fatal("empty list must allow every domain")
	}
	c, _ = LoadFrom(env(map[string]string{"CATALYSTCAL_JWT_SECRET": strings.Repeat("x", 32),
		"CATALYSTCAL_ALLOWED_DOMAINS": "Team.Example", "CATALYSTCAL_ALLOWED_USERS": "guest-1@Other.Example"}))
	cases := map[[2]string]bool{
		{"team.example", "a"}:        true,
		{"TEAM.example", "b"}:        true,
		{"other.example", "guest-1"}: true,
		{"other.example", "guest-2"}: false,
		{"evil.example", "a"}:        false,
	}
	for in, want := range cases {
		if got := c.SignInAllowed(in[0], in[1]); got != want {
			t.Errorf("%v: %v, want %v", in, got, want)
		}
	}
}
