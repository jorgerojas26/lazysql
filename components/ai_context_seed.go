package components

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/jorgerojas26/lazysql/internal/ai"
)

func cachedAIContext(loader *schemaLoader, preferred string, schemas []string) ([]string, bool, []ai.TableCatalog, []ai.Table) {
	generation := loader.cache.generationValue()
	read := func(database, table string, kind MetadataKind) (any, bool) {
		state, value, err := loader.cache.result(newMetadataKey(database, table, kind))
		return value, state == MetadataReady && err == nil
	}
	names, known := read("", "", MetadataDatabases)
	databases, ok := names.([]string)
	complete := known && ok
	if !complete {
		databases = nil
	}
	databases = slices.Clone(databases)
	if preferred != "" && !slices.Contains(databases, preferred) {
		databases = append(databases, preferred)
	}
	slices.Sort(databases)
	ordered := slices.Clone(databases)
	if index := slices.Index(ordered, preferred); index > 0 {
		ordered = append([]string{preferred}, append(ordered[:index], ordered[index+1:]...)...)
	}
	catalogs := []ai.TableCatalog{}
	tables := []ai.Table{}
	for _, database := range ordered {
		raw, ready := read(database, "", MetadataTables)
		catalog, valid := raw.(map[string][]string)
		if !ready || !valid {
			continue
		}
		visible := loader.visibleTables(database, catalog, schemas)
		allowed := map[string]bool{}
		entry := ai.TableCatalog{Database: database, Tables: []string{}}
		for _, table := range visible {
			allowed[table.qualifiedName] = true
		}
		for _, table := range visible {
			if len(entry.Tables) == 100 {
				entry.More = true
				break
			}
			entry.Tables = append(entry.Tables, table.qualifiedName)
		}
		catalogs = append(catalogs, entry)
		for _, table := range visible {
			name := table.qualifiedName
			columns, columnsReady := read(database, name, MetadataColumns)
			keys, keysReady := read(database, name, MetadataPrimaryKeys)
			kind := MetadataForeignKeys
			if _, outgoing := loader.driver.(interface {
				GetOutgoingForeignKeys(context.Context, string, string) ([][]string, error)
			}); outgoing {
				kind = MetadataKind("ai_outgoing_foreign_keys")
			}
			foreign, foreignReady := read(database, name, kind)
			incoming, incomingReady := read(database, name, MetadataReferencingTables)
			_, keysValid := keys.([]string)
			if !columnsReady || !keysReady || !keysValid || !foreignReady || !incomingReady {
				continue
			}
			item := ai.Table{Database: database, Name: name, Columns: aiColumns(columns)}
			item.PrimaryKeys, _ = keys.([]string)
			if len(item.Columns) == 0 {
				continue
			}
			relationships := append(aiRelationships(foreign, name, false, loader.driver.UseSchemas()), aiRelationships(incoming, name, true, loader.driver.UseSchemas())...)
			for _, relationship := range relationships {
				if allowed[relationship.FromTable] && allowed[relationship.ToTable] {
					item.Relationships = append(item.Relationships, relationship)
				}
			}
			candidate := append(tables, item)
			encoded, _ := json.Marshal(candidate)
			if len(encoded) <= 12000 {
				tables = candidate
			}
		}
	}
	for {
		encoded, _ := json.Marshal(struct {
			Databases []string
			Catalogs  []ai.TableCatalog
			Tables    []ai.Table
		}{databases, catalogs, tables})
		if len(encoded) <= 16000 {
			break
		}
		if len(catalogs) > 0 {
			catalogs = catalogs[:len(catalogs)-1]
			continue
		}
		if len(databases) > 0 {
			databases = databases[:len(databases)-1]
			complete = false
			continue
		}
		break
	}
	if loader.cache.generationValue() != generation {
		return nil, false, nil, nil
	}
	return databases, complete, catalogs, tables
}
