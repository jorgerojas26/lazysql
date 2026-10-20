package ai

import (
	"context"
	"os"
	"time"

	"github.com/jorgerojas26/lazysql/models"
)

type ProviderStatus struct {
	Label string
	Ready bool
}

func ProviderLoginStatus(ctx context.Context, config models.AIConfig) map[string]ProviderStatus {
	status := map[string]ProviderStatus{"chatgpt": {Label: "Not logged in"}, "opencode-go": {Label: "Not logged in"}}
	service, err := newChatGPTService()
	if err != nil {
		status["chatgpt"] = ProviderStatus{Label: "Check credentials"}
	} else {
		status["chatgpt"] = service.loginStatus(ctx)
	}
	env := config.APIKeyEnv
	if env == "" {
		env = "OPENCODE_API_KEY"
	}
	if key := os.Getenv(env); key != "" {
		if _, err := validateOpenCodeKey(key); err != nil {
			status["opencode-go"] = ProviderStatus{Label: "Check key"}
		} else {
			status["opencode-go"] = ProviderStatus{Label: "Logged in", Ready: true}
		}
	} else if config.APIKeyEnv == "" || config.APIKeyEnv == "OPENCODE_API_KEY" {
		key, err := LoadOpenCodeKey()
		if err != nil {
			status["opencode-go"] = ProviderStatus{Label: "Check key"}
		} else if key != "" {
			status["opencode-go"] = ProviderStatus{Label: "Logged in", Ready: true}
		}
	}
	return status
}

func (service *chatGPTService) loginStatus(parent context.Context) ProviderStatus {
	if _, err := os.Lstat(service.path); os.IsNotExist(err) {
		return ProviderStatus{Label: "Not logged in"}
	} else if err != nil {
		return ProviderStatus{Label: "Check credentials"}
	}
	ctx, cancel := context.WithTimeout(parent, 200*time.Millisecond)
	defer cancel()
	status := ProviderStatus{Label: "Not logged in"}
	err := withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		account := store.account(store.Active)
		if account == nil || account.AccessToken == "" {
			for _, saved := range store.Accounts {
				if saved.AccessToken != "" && (saved.ExpiresAt.After(time.Now()) || saved.RefreshToken != "") {
					status = ProviderStatus{Label: "Account saved"}
					break
				}
			}
			return false, nil
		}
		switch {
		case account.Issuer != service.issuer:
			status = ProviderStatus{Label: "Check credentials"}
		case !hasChatGPTPlan(*account):
			status = ProviderStatus{Label: "Plan access needed"}
		case account.ExpiresAt.After(time.Now()) || account.RefreshToken != "":
			status = ProviderStatus{Label: "Logged in", Ready: true}
		default:
			status = ProviderStatus{Label: "Sign in again"}
		}
		return false, nil
	})
	if ctx.Err() != nil {
		return ProviderStatus{Label: "Unavailable"}
	}
	if err != nil {
		return ProviderStatus{Label: "Check credentials"}
	}
	return status
}
