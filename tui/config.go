package tui

import "time"

const DefaultTarget = "http://127.0.0.1:9999/metric"

type Config struct {
	Target          string
	RefreshInterval time.Duration
	Timeout         time.Duration
	Plain           bool
}

func (c Config) normalized() Config {
	if c.Target == "" {
		c.Target = DefaultTarget
	}
	if c.RefreshInterval <= 0 {
		c.RefreshInterval = 2 * time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 1500 * time.Millisecond
	}
	return c
}
