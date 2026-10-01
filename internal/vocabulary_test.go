package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestForeignKeyJumpVocabulary(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	root := filepath.Dir(filepath.Dir(filename))

	specifications := []string{
		".beans/lazysql-a9lf--network-performance-progressive-loading.md",
		".beans/lazysql-846w--03-enrich-records-progressively-with-cached-table.md",
	}
	forbidden := []string{
		"fk-jump",
		"fk jump",
		"composite-fk",
		"composite fk",
		"fk drilldown",
		"relation follow",
		"link jump",
		"relation-navigation",
		"relation navigation",
	}

	for _, relativePath := range specifications {
		path := filepath.Join(root, relativePath)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		text := string(contents)
		if !strings.Contains(text, "Foreign Key Jump") {
			t.Errorf("%s does not use the glossary term %q", relativePath, "Foreign Key Jump")
		}
		lowerText := strings.ToLower(text)
		for _, synonym := range forbidden {
			if strings.Contains(lowerText, synonym) {
				t.Errorf("%s contains rejected Foreign Key Jump synonym %q", relativePath, synonym)
			}
		}
	}
}
