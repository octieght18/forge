// Package config reads and validates process configuration before serving traffic.
package config

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

type Config struct {
	Address           string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// Load accepts the environment lookup function so tests do not mutate process state.
func Load(lookup func(string) (string, bool)) (Config, error) {
	c := Config{Address: "127.0.0.1:8081"}
	if value, ok := lookup("FORGE_HTTP_ADDR"); ok {
		c.Address = value
	}
	durations := []struct {
		name     string
		fallback time.Duration
		target   *time.Duration
	}{
		{"FORGE_HTTP_READ_HEADER_TIMEOUT", 5 * time.Second, &c.ReadHeaderTimeout},
		{"FORGE_HTTP_READ_TIMEOUT", 10 * time.Second, &c.ReadTimeout},
		{"FORGE_HTTP_WRITE_TIMEOUT", 15 * time.Second, &c.WriteTimeout},
		{"FORGE_HTTP_IDLE_TIMEOUT", 60 * time.Second, &c.IdleTimeout},
		{"FORGE_SHUTDOWN_TIMEOUT", 10 * time.Second, &c.ShutdownTimeout},
	}
	for _, setting := range durations {
		*setting.target = setting.fallback
		if value, ok := lookup(setting.name); ok {
			d, err := time.ParseDuration(value)
			if err != nil || d <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive Go duration", setting.name)
			}
			*setting.target = d
		}
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(c.Address)
	if err != nil {
		return fmt.Errorf("FORGE_HTTP_ADDR must be a loopback IP and port")
	}
	ip := net.ParseIP(host)
	p, err := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("FORGE_HTTP_ADDR must be a loopback IP and port from 1 to 65535")
	}
	if c.ReadHeaderTimeout <= 0 || c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return fmt.Errorf("all server timeouts must be positive")
	}
	if c.ReadHeaderTimeout > c.ReadTimeout {
		return fmt.Errorf("header read timeout must not exceed request read timeout")
	}
	return nil
}
