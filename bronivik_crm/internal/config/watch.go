package config

import (
	"context"
	"os"
	"time"
)

// WatchCabinets reloads cabinets.yaml on change and calls handler with the latest config.
// It performs an initial load before entering the watch loop.
func WatchCabinets(ctx context.Context, path string, interval time.Duration, onUpdate func(*CabinetsConfig)) error {
	if path == "" {
		path = "configs/cabinets.yaml"
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}

	cfg, err := LoadCabinetsConfig(path)
	if err != nil {
		return err
	}
	if onUpdate != nil {
		onUpdate(cfg)
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	lastMod := info.ModTime()

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(path)
				if err != nil {
					continue // transient errors
				}
				if !info.ModTime().After(lastMod) {
					continue
				}
				cfg, err := LoadCabinetsConfig(path)
				if err != nil {
					continue
				}
				lastMod = info.ModTime()
				if onUpdate != nil {
					onUpdate(cfg)
				}
			}
		}
	}()

	return nil
}
