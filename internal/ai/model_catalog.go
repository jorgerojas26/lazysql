package ai

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/jorgerojas26/lazysql/models"
)

func AvailableModels(ctx context.Context, config models.AIConfig) ([]Model, error) {
	return availableModels(ctx, ProviderLoginStatus(ctx, config), func(ctx context.Context, provider string) ([]Model, error) {
		if provider == "chatgpt" {
			return ChatGPTModels(ctx)
		}
		return OpenCodeModels(ctx)
	})
}

func availableModels(ctx context.Context, status map[string]string, load func(context.Context, string) ([]Model, error)) ([]Model, error) {
	type result struct {
		models []Model
		err    error
	}
	providers := []string{"chatgpt", "opencode-go"}
	results := make([]result, len(providers))
	var wait sync.WaitGroup
	for index, provider := range providers {
		if status[provider] != "Logged in" {
			continue
		}
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index].models, results[index].err = load(ctx, provider)
		}()
	}
	wait.Wait()
	var catalog []Model
	var unavailable []string
	for index, result := range results {
		if result.err != nil {
			unavailable = append(unavailable, providers[index])
			continue
		}
		for _, model := range result.models {
			model.Provider = providers[index]
			catalog = append(catalog, model)
		}
	}
	sort.SliceStable(catalog, func(i, j int) bool {
		if catalog[i].Provider != catalog[j].Provider {
			return catalog[i].Provider < catalog[j].Provider
		}
		return strings.ToLower(catalog[i].Name) < strings.ToLower(catalog[j].Name)
	})
	if len(unavailable) > 0 {
		return catalog, errors.New("Could not load " + strings.Join(unavailable, ", ") + ". Ctrl+R to retry.")
	}
	if len(catalog) == 0 {
		return nil, errors.New("No signed-in models available. Run lazysql ai login, then Ctrl+R to retry.")
	}
	return catalog, nil
}
