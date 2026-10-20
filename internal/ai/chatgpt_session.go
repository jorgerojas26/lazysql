package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type chatGPTError struct {
	Code      string
	Status    int
	RequestID string
}

func (failure *chatGPTError) Error() string {
	message := "ChatGPT request failed; try again later"
	switch failure.Code {
	case "invalid_grant", "invalid_refresh_token", "token_expired", "refresh_token_expired", "refresh_token_invalidated", "refresh_token_reused", "subscription_sharing_invalid_user":
		message = "ChatGPT session expired or was disconnected; sign in again with lazysql ai login"
	case "invalid_client":
		message = "ChatGPT client registration was rejected; select your saved account and sign in again"
	case "subscription_sharing_usage_limit_exceeded":
		message = "ChatGPT usage limit reached. Manage usage: " + ChatGPTUsageURL
	case "subscription_sharing_user_not_eligible":
		message = "ChatGPT plan use is unavailable for this account or workspace; choose another account or provider"
	case "subscription_sharing_usage_unavailable", "subscription_sharing_user_unavailable":
		message = "ChatGPT usage is temporarily unavailable; retry later"
	case "subscription_sharing_unsupported_capability":
		message = "ChatGPT rejected an unsupported model or request capability; choose another available model"
	case "subscription_sharing_route_not_supported", "chatpass_v2_scope_not_authorized", "chatpass_v2_invalid_authorization_context":
		message = "ChatGPT plan permission or routing was rejected; check your account authorization"
	default:
		switch failure.Status {
		case 401:
			message = "ChatGPT authorization was not accepted; check your account and sign in again"
		case 403:
			message = "ChatGPT access is restricted for this account, workspace or region"
		case 429:
			message = "ChatGPT limit reached. Manage usage: " + ChatGPTUsageURL
		case 503:
			message = "ChatGPT is temporarily unavailable; credentials were preserved. Retry later"
		}
	}
	if failure.Status > 0 {
		message += fmt.Sprintf(" (HTTP %d)", failure.Status)
	}
	if failure.RequestID != "" {
		message += " · request " + failure.RequestID
	}
	return message
}

func oauthRequestError(status int, data []byte) error {
	var response struct {
		Error json.RawMessage `json:"error"`
	}
	code := ""
	if json.Unmarshal(data, &response) == nil {
		if json.Unmarshal(response.Error, &code) != nil {
			var details struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(response.Error, &details) == nil {
				code = details.Code
			}
		}
	}
	return &chatGPTError{Status: status, Code: code}
}

func terminalRefreshError(err error) bool {
	var failure *chatGPTError
	if !errors.As(err, &failure) {
		return false
	}
	switch failure.Code {
	case "invalid_grant", "invalid_refresh_token", "token_expired", "refresh_token_expired", "refresh_token_invalidated", "refresh_token_reused":
		return true
	}
	return false
}

func (service *chatGPTService) activeToken(ctx context.Context) (string, error) {
	var token string
	err := withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		account := store.account(store.Active)
		if account == nil || account.AccessToken == "" {
			return false, errors.New("sign in with ChatGPT using lazysql ai login")
		}
		if account.Issuer != service.issuer || !strings.HasPrefix(account.ClientID, "oaiapp_") {
			return false, errors.New("invalid ChatGPT account registration; sign in again")
		}
		if !hasChatGPTPlan(*account) {
			return false, errors.New("ChatGPT plan permission is missing; run lazysql ai login to grant access")
		}
		if time.Until(account.ExpiresAt) > 2*time.Minute || (time.Now().Before(account.EarliestRefreshAt) && time.Now().Before(account.ExpiresAt)) {
			token = account.AccessToken
			return false, nil
		}
		if account.RefreshToken == "" {
			return false, errors.New("ChatGPT session cannot be renewed; sign in again")
		}
		provider, discovery, err := service.discovery(ctx)
		if err != nil {
			return false, err
		}
		replacement, err := service.tokenRequest(ctx, discovery.Token, url.Values{"grant_type": {"refresh_token"}, "client_id": {account.ClientID}, "refresh_token": {account.RefreshToken}, "resource": {chatGPTResource}})
		if err != nil {
			if terminalRefreshError(err) {
				clearChatGPTTokens(account)
				return true, err
			}
			return false, err
		}
		if replacement.RefreshToken == "" {
			return false, errors.New("ChatGPT refresh did not return a replacement session; sign in again")
		}
		if replacement.IDToken != "" {
			identity, err := service.validateIdentity(ctx, provider, account.ClientID, replacement.IDToken, "")
			if err != nil {
				return false, err
			}
			if identity.Subject != account.Subject || identity.Issuer != account.Issuer {
				return false, errors.New("ChatGPT refresh returned a different account; sign in again")
			}
		}
		applyChatGPTTokens(account, replacement)
		if !hasChatGPTPlan(*account) {
			return true, errors.New("ChatGPT plan permission was removed; sign in again to grant access")
		}
		token = account.AccessToken
		return true, nil
	})
	return token, err
}

func (service *chatGPTService) logout(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	unconfirmed := false
	err := withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		account := store.account(store.Active)
		if account == nil {
			return false, nil
		}
		if account.RefreshToken != "" {
			_, discovery, err := service.discovery(ctx)
			if err != nil {
				unconfirmed = true
			} else {
				revoked := false
				for attempt := 0; attempt < 2; attempt++ {
					request, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.Revocation, strings.NewReader(url.Values{"token": {account.RefreshToken}, "token_type_hint": {"refresh_token"}, "client_id": {account.ClientID}}.Encode()))
					if err != nil {
						break
					}
					request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					response, err := service.http.Do(request)
					if err == nil {
						io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
						response.Body.Close()
						if response.StatusCode == http.StatusOK {
							revoked = true
							break
						}
						if response.StatusCode < 500 {
							break
						}
					}
					if attempt == 0 {
						select {
						case <-ctx.Done():
						case <-time.After(200 * time.Millisecond):
						}
					}
				}
				unconfirmed = !revoked
			}
		}
		clearChatGPTTokens(account)
		store.Active = ""
		return true, nil
	})
	if err != nil {
		return err
	}
	if unconfirmed {
		return errors.New("Signed out locally. Remote revocation was not confirmed; disconnect LazySQL in ChatGPT Settings to end remote access. Other apps are unchanged.")
	}
	return nil
}

func LogoutChatGPT(ctx context.Context) error {
	service, err := newChatGPTService()
	if err != nil {
		return err
	}
	return service.logout(ctx)
}
