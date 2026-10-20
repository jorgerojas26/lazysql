package app

import "github.com/pelletier/go-toml/v2"

func ReadThemeConfig(configFile string) (ThemeConfig, error) {
	table, err := readConfigTable(configFile)
	if err != nil {
		return ThemeConfig{}, err
	}
	localPath, err := FindLocalConfig()
	if err != nil {
		return ThemeConfig{}, err
	}
	if localPath != "" {
		local, err := readConfigTable(localPath)
		if err != nil {
			return ThemeConfig{}, err
		}
		table = mergeMaps(table, local)
	}
	if _, ok := table["theme"]; !ok {
		return ThemeConfig{}, nil
	}
	data, err := toml.Marshal(map[string]any{"theme": table["theme"]})
	if err != nil {
		return ThemeConfig{}, err
	}
	var config struct {
		Theme ThemeConfig `toml:"theme"`
	}
	if err := toml.Unmarshal([]byte(expandEnvVars(string(data))), &config); err != nil {
		return ThemeConfig{}, err
	}
	return config.Theme, nil
}
