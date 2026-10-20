package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const openCodeModelsURL = "https://opencode.ai/inference/go/v1/models"
const openCodeMetadataURL = "https://models.dev/api.json"

func OpenCodeModels(ctx context.Context) ([]Model, error) {
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("catalog redirects are not allowed") }}
	return fetchOpenCodeModels(ctx, client, openCodeModelsURL, openCodeMetadataURL)
}

func fetchCatalog(ctx context.Context, client *http.Client, address string, limit int) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "lazysql/ai-sql")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("cannot load OpenCode Go model catalog; check your connection and retry login")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("model catalog is unavailable; retry login later")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, int64(limit+1)))
	if err != nil || len(data) > limit {
		return nil, errors.New("invalid model catalog response")
	}
	return data, nil
}

func fetchOpenCodeModels(ctx context.Context, client *http.Client, modelsURL, metadataURL string) ([]Model, error) {
	data, err := fetchCatalog(ctx, client, modelsURL, 1024*1024)
	if err != nil {
		return nil, err
	}
	var live struct {
		Data []struct {
			ID            string `json:"id"`
			ContextWindow int    `json:"context_window"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &live) != nil || len(live.Data) == 0 {
		return nil, errors.New("OpenCode Go returned an empty or invalid model list")
	}
	data, err = fetchCatalog(ctx, client, metadataURL, 16*1024*1024)
	if err != nil {
		return nil, err
	}
	type providerCatalog struct {
		NPM    string `json:"npm"`
		Models map[string]struct {
			Name     string `json:"name"`
			Provider struct {
				NPM string `json:"npm"`
			} `json:"provider"`
		} `json:"models"`
	}
	var metadata struct {
		Go       providerCatalog `json:"opencode-go"`
		OpenCode providerCatalog `json:"opencode"`
	}
	if json.Unmarshal(data, &metadata) != nil || metadata.Go.NPM == "" {
		return nil, errors.New("OpenCode Go protocol metadata is unavailable; retry login later")
	}
	seen := map[string]bool{}
	var models []Model
	for _, item := range live.Data {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		details, known := metadata.Go.Models[item.ID]
		defaultNPM := metadata.Go.NPM
		if !known {
			if other, exists := metadata.OpenCode.Models[item.ID]; exists {
				details = other
				if metadata.OpenCode.NPM != "" {
					defaultNPM = metadata.OpenCode.NPM
				}
			}
		}
		npm := details.Provider.NPM
		if npm == "" {
			npm = defaultNPM
		}
		format := ""
		switch npm {
		case "@ai-sdk/openai-compatible":
			format = "chat-completions"
		case "@ai-sdk/openai":
			format = "responses"
		case "@ai-sdk/anthropic":
			format = "anthropic"
		case "@ai-sdk/google":
			format = "gemini"
		default:
			return nil, errors.New("a Go model requires an unsupported API protocol; update LazySQL")
		}
		name := details.Name
		if name == "" {
			name = item.ID
		}
		models = append(models, Model{ID: item.ID, Name: name, Format: format, ContextWindow: item.ContextWindow})
	}
	if len(models) == 0 {
		return nil, errors.New("OpenCode Go returned no available models")
	}
	sort.Slice(models, func(i, j int) bool {
		left, right := strings.ToLower(models[i].Name), strings.ToLower(models[j].Name)
		if left == right {
			return models[i].ID < models[j].ID
		}
		return left < right
	})
	return models, nil
}
