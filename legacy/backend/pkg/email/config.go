package email

import (
	"errors"
	"fmt"
	"strings"

	"auto-pack/backend/pkg/envtypes"
)

// Config holds SMTP connection and sender settings.
type Config struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	From               string `json:"from"`
	FromName           string `json:"from_name,omitempty"`
	TLS                bool   `json:"tls,omitempty"`      // implicit TLS (typical port 465)
	StartTLS           bool   `json:"starttls,omitempty"` // STARTTLS upgrade (typical port 587)
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
	TimeoutSec         int    `json:"timeout_sec,omitempty"`
}

// Normalize fills default port and TLS mode from Port when unset.
func (c *Config) Normalize() {
	c.Host = strings.TrimSpace(c.Host)
	c.Username = strings.TrimSpace(c.Username)
	c.From = strings.TrimSpace(c.From)
	c.FromName = strings.TrimSpace(c.FromName)

	port := envtypes.DefaultPort(c.Port, 587)
	c.Port = port

	if !c.TLS && !c.StartTLS {
		switch port {
		case 465:
			c.TLS = true
		case 587, 25:
			c.StartTLS = true
		}
	}
}

// Validate checks required SMTP fields.
func (c Config) Validate() error {
	cfg := c
	cfg.Normalize()
	if cfg.Host == "" {
		return errors.New("smtp host is required")
	}
	if cfg.From == "" {
		return errors.New("smtp from address is required")
	}
	if err := ValidateAddress(cfg.From); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	return nil
}

// Addr returns host:port for dialing.
func (c Config) Addr() string {
	cfg := c
	cfg.Normalize()
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}

// Timeout returns the dial / SMTP operation timeout.
func (c Config) Timeout() int {
	return envtypes.DefaultTimeoutSec(c.TimeoutSec)
}
