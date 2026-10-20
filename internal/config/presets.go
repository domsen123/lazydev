package config

import (
	"os"
	"path/filepath"
	"slices"
)

// PresetNuxtDrizzle steht für Projekte aus dem Boilerplate ~/development/nuxt/nuxt-drizzle.
// Setup, Teardown und Prüfbefehle des Presets liegen in internal/hooks.
const PresetNuxtDrizzle = "nuxt-drizzle"

var knownPresets = []string{PresetNuxtDrizzle}

func IsKnownPreset(name string) bool {
	return slices.Contains(knownPresets, name)
}

// DetectPreset erkennt das Preset am Dateibestand des Projekts.
// Ohne Treffer liefert DetectPreset einen leeren String.
func DetectPreset(dir string) string {
	if exists(filepath.Join(dir, "drizzle.config.ts")) &&
		exists(filepath.Join(dir, "server", "database", "migrations")) {
		return PresetNuxtDrizzle
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
