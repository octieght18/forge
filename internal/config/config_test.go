package config

import (
	"strings"
	"testing"
	"time"
)

func environment(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}

func TestLoadConfiguration(t *testing.T) {
	c, err := Load(environment(map[string]string{"FORGE_HTTP_ADDR": "[::1]:8082", "FORGE_SHUTDOWN_TIMEOUT": "2s"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != "[::1]:8082" || c.ShutdownTimeout != 2*time.Second || c.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestRejectUnsafeConfiguration(t *testing.T) {
	cases := []map[string]string{
		{"FORGE_HTTP_ADDR": "0.0.0.0:8081"},
		{"FORGE_HTTP_ADDR": ":8081"},
		{"FORGE_HTTP_ADDR": "localhost:8081"},
		{"FORGE_HTTP_ADDR": "127.0.0.1:0"},
		{"FORGE_HTTP_ADDR": "127.0.0.1:65536"},
		{"FORGE_HTTP_ADDR": "127.0.0.1:http"},
		{"FORGE_HTTP_ADDR": ""},
		{"FORGE_HTTP_READ_TIMEOUT": "0s"},
		{"FORGE_HTTP_WRITE_TIMEOUT": "-1s"},
		{"FORGE_HTTP_IDLE_TIMEOUT": "secret-value"},
		{"FORGE_SHUTDOWN_TIMEOUT": ""},
		{"FORGE_HTTP_READ_HEADER_TIMEOUT": "20s"},
	}
	for i, values := range cases {
		if _, err := Load(environment(values)); err == nil {
			t.Errorf("case %d accepted unsafe config", i)
		} else if strings.Contains(err.Error(), "secret-value") {
			t.Fatal("configuration value leaked")
		}
	}
}
