package config

import (
	"fmt"
	"time"
)

type Product struct {
	Enabled                                                        bool
	DatabaseURL, Issuer, Audience, CursorKeyFile, CorpusPolicyFile string
	Timeout                                                        time.Duration
}

func LoadProduct(lookup func(string) (string, bool)) (Product, error) {
	c := Product{Audience: "forge-api", Timeout: 5 * time.Second}
	flag, _ := lookup("FORGE_PRODUCT_API")
	if flag != "" && flag != "false" && flag != "true" {
		return c, fmt.Errorf("FORGE_PRODUCT_API must be true or false")
	}
	c.Enabled = flag == "true"
	settings := []struct {
		name   string
		target *string
	}{{"FORGE_DATABASE_URL", &c.DatabaseURL}, {"FORGE_OIDC_ISSUER", &c.Issuer}, {"FORGE_CURSOR_KEY_FILE", &c.CursorKeyFile}, {"FORGE_CORPUS_POLICY_FILE", &c.CorpusPolicyFile}}
	for _, s := range settings {
		value, ok := lookup(s.name)
		*s.target = value
		if !c.Enabled && ok {
			return c, fmt.Errorf("product settings require FORGE_PRODUCT_API=true")
		}
		if c.Enabled && value == "" {
			return c, fmt.Errorf("%s is required", s.name)
		}
	}
	if value, ok := lookup("FORGE_OPERATION_TIMEOUT"); ok {
		d, err := time.ParseDuration(value)
		if !c.Enabled || err != nil || d <= 0 || d > 10*time.Second {
			return c, fmt.Errorf("FORGE_OPERATION_TIMEOUT requires product mode and a positive duration up to 10s")
		}
		c.Timeout = d
	}
	return c, nil
}
