package telemetry

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type preference struct {
	Enabled  *bool  `toml:"enabled"`
	Endpoint string `toml:"endpoint"`
}

// Consent lives in a separate user-global file, never a project config. A new
// collector requires new consent; declining applies to all collectors.
func LoadConsent(path, endpoint string) (enabled, decided bool, err error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, true, err
	}
	var p preference
	if err := toml.Unmarshal(content, &p); err != nil {
		return false, true, err
	}
	if p.Enabled == nil {
		return false, true, errors.New("missing telemetry preference")
	}
	if !*p.Enabled {
		return false, true, nil
	}
	if p.Endpoint != endpoint {
		return false, false, nil
	}
	return true, true, nil
}

func SaveConsent(path, endpoint string, enabled bool) error {
	content, err := toml.Marshal(preference{Enabled: &enabled, Endpoint: endpoint})
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".telemetry-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
