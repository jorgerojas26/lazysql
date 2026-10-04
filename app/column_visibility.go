package app

import "slices"

func (a *Application) HiddenColumns(connection, database, table string) []string {
	return slices.Clone(a.config.HiddenColumns[connection][database][table])
}

func (a *Application) SaveHiddenColumns(connection, database, table string, columns []string) error {
	configFile := a.config.activeConfigFile()
	configTable, err := readConfigTable(configFile)
	if err != nil {
		return err
	}

	hidden := configSection(configTable, "hidden_columns")
	tables := configSection(configSection(hidden, connection), database)
	tables[table] = append([]string{}, columns...)
	if err := saveConfigTable(configFile, configTable, "hidden_columns", hidden); err != nil {
		return err
	}

	if a.config.HiddenColumns == nil {
		a.config.HiddenColumns = make(map[string]map[string]map[string][]string)
	}
	if a.config.HiddenColumns[connection] == nil {
		a.config.HiddenColumns[connection] = make(map[string]map[string][]string)
	}
	if a.config.HiddenColumns[connection][database] == nil {
		a.config.HiddenColumns[connection][database] = make(map[string][]string)
	}
	a.config.HiddenColumns[connection][database][table] = slices.Clone(columns)
	return nil
}

func configSection(parent map[string]any, key string) map[string]any {
	section, ok := parent[key].(map[string]any)
	if !ok {
		section = make(map[string]any)
		parent[key] = section
	}
	return section
}
