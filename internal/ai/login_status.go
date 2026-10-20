package ai

import (
	"context"
	"os"
	"time"

	"github.com/jorgerojas26/lazysql/models"
)

func ProviderLoginStatus(ctx context.Context, config models.AIConfig) map[string]string {
	status := map[string]string{"chatgpt": "Not logged in", "opencode-go": "Not logged in"}
	service, err := newChatGPTService()
	if err != nil {
		status["chatgpt"] = "Check credentials"
	} else {
		status["chatgpt"] = service.loginStatus(ctx)
	}
	env := config.APIKeyEnv
	if env == "" {
		env = "OPENCODE_API_KEY"
	}
	if key := os.Getenv(env); key != "" {
		if _, err := validateOpenCodeKey(key); err != nil {
			status["opencode-go"] = "Check key"
		} else {
			status["opencode-go"] = "Logged in"
		}
	} else if config.APIKeyEnv == "" || config.APIKeyEnv == "OPENCODE_API_KEY" {
		key, err := LoadOpenCodeKey()
		if err != nil {
			status["opencode-go"] = "Check key"
		} else if key != "" {
			status["opencode-go"] = "Logged in"
		}
	}
	return status
}

func (service *chatGPTService) loginStatus(parent context.Context) string {
	if _, err := os.Lstat(service.path); os.IsNotExist(err) {
		return "Not logged in"
	} else if err != nil {
		return "Check credentials"
	}
	ctx, cancel := context.WithTimeout(parent, 200*time.Millisecond)
	defer cancel()
	status := "Not logged in"
	err := withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		account := store.account(store.Active)
		if account == nil || account.AccessToken == "" {
			for _, saved := range store.Accounts {
				if saved.AccessToken != "" && (saved.ExpiresAt.After(time.Now()) || saved.RefreshToken != "") {
					status = "Account saved"
					break
				}
			}
			return false, nil
		}
		switch {
		case account.Issuer != service.issuer:
			status = "Check credentials"
		case !hasChatGPTPlan(*account):
			status = "Plan access needed"
		case account.ExpiresAt.After(time.Now()) || account.RefreshToken != "":
			status = "Logged in"
		default:
			status = "Sign in again"
		}
		return false, nil
	})
	if ctx.Err() != nil {
		return "Unavailable"
	}
	if err != nil {
		return "Check credentials"
	}
	return status
}
