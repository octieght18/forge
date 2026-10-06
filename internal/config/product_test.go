package config

import (
	"strings"
	"testing"
	"time"
)

func TestProductModeRequiresCompleteConfiguration(t *testing.T) {
	c, err := LoadProduct(environment(nil))
	if err != nil || c.Enabled {
		t.Fatal("foundation mode changed")
	}
	values := map[string]string{"FORGE_PRODUCT_API": "true", "FORGE_DATABASE_URL": "private-fixture", "FORGE_OIDC_ISSUER": "https://identity.example/realm", "FORGE_CURSOR_KEY_FILE": "private-file", "FORGE_CORPUS_POLICY_FILE": "policy.json"}
	c, err = LoadProduct(environment(values))
	if err != nil || !c.Enabled || c.Timeout != 5*time.Second {
		t.Fatal("complete configuration rejected", err)
	}
	for _, key := range []string{"FORGE_DATABASE_URL", "FORGE_OIDC_ISSUER", "FORGE_CURSOR_KEY_FILE", "FORGE_CORPUS_POLICY_FILE"} {
		copy := map[string]string{}
		for k, v := range values {
			if k != key {
				copy[k] = v
			}
		}
		if _, err := LoadProduct(environment(copy)); err == nil {
			t.Fatal("partial configuration accepted")
		}
	}
	for _, v := range []map[string]string{{"FORGE_DATABASE_URL": "private-fixture"}, {"FORGE_PRODUCT_API": "yes"}, {"FORGE_OPERATION_TIMEOUT": "30s"}} {
		_, err := LoadProduct(environment(v))
		if err == nil || strings.Contains(err.Error(), "private-fixture") {
			t.Fatal("unsafe product configuration")
		}
	}
}
