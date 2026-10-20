package ai

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorgerojas26/lazysql/models"
)

func TestProviderStatusDoesNotInitializeCredentialsOrExposeSecrets(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", directory)
	t.Setenv("OPENCODE_API_KEY", "")
	status := ProviderLoginStatus(context.Background(), models.AIConfig{})
	if status["chatgpt"].Label != "Not logged in" || status["opencode-go"].Label != "Not logged in" {
		t.Fatalf("empty provider status=%v", status)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("checking login status created credentials or locks")
	}
	if err := SaveOpenCodeKey("private-saved-key"); err != nil {
		t.Fatal(err)
	}
	status = ProviderLoginStatus(context.Background(), models.AIConfig{})
	if status["opencode-go"].Label != "Logged in" {
		t.Fatal("saved subscription key status missing")
	}
	t.Setenv("OPENCODE_API_KEY", "private-environment-key")
	status = ProviderLoginStatus(context.Background(), models.AIConfig{})
	if status["opencode-go"].Label != "Logged in" {
		t.Fatal("environment credential status missing")
	}
	for _, value := range status {
		if strings.Contains(value.Label, "private-") {
			t.Fatal("credential leaked in status")
		}
	}
	if err := DeleteOpenCodeKey(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_API_KEY", "")
	if status := ProviderLoginStatus(context.Background(), models.AIConfig{}); status["opencode-go"].Label != "Not logged in" {
		t.Fatal("status did not reflect logout")
	}
}

func TestProviderStatusRespectsExplicitEnvironmentSource(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENCODE_API_KEY", "other-account-key")
	t.Setenv("CUSTOM_GO_KEY", "")
	if err := SaveOpenCodeKey("saved-account-key"); err != nil {
		t.Fatal(err)
	}
	config := models.AIConfig{APIKeyEnv: "CUSTOM_GO_KEY"}
	if status := ProviderLoginStatus(context.Background(), config); status["opencode-go"].Label != "Not logged in" {
		t.Fatal("missing custom environment source showed a different account as logged in")
	}
	t.Setenv("CUSTOM_GO_KEY", "configured-key")
	if status := ProviderLoginStatus(context.Background(), config); status["opencode-go"].Label != "Logged in" {
		t.Fatal("custom environment key was ignored")
	}
	t.Setenv("CUSTOM_GO_KEY", "bad key")
	if status := ProviderLoginStatus(context.Background(), config); status["opencode-go"].Label != "Check key" {
		t.Fatal("invalid configured key shown as connected")
	}
}

func TestChatGPTStatusIsReadOnlyAndRecognizesRefreshableSessions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	service, err := newChatGPTService()
	if err != nil {
		t.Fatal(err)
	}
	account := ChatGPTAccount{ClientID: "oaiapp_status", Subject: "subject", Issuer: chatGPTIssuer, AccessToken: "private-access-token", RefreshToken: "private-refresh-token", Scopes: strings.Fields(chatGPTScopes), ExpiresAt: time.Now().Add(time.Hour)}
	set := func(value ChatGPTAccount, active string) {
		t.Helper()
		if err := withChatGPTStore(context.Background(), service.path, func(store *chatGPTStore) (bool, error) {
			store.Active = active
			store.Accounts = []ChatGPTAccount{value}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	set(account, account.ClientID)
	before, err := os.ReadFile(service.path)
	if err != nil {
		t.Fatal(err)
	}
	if status := service.loginStatus(context.Background()); status.Label != "Logged in" {
		t.Fatalf("valid session status=%q", status.Label)
	}
	after, _ := os.ReadFile(service.path)
	if string(before) != string(after) {
		t.Fatal("status check modified stored credentials")
	}
	account.ExpiresAt = time.Now().Add(-time.Hour)
	set(account, account.ClientID)
	if status := service.loginStatus(context.Background()); status.Label != "Logged in" {
		t.Fatal("refreshable session shown as logged out")
	}
	account.RefreshToken = ""
	set(account, account.ClientID)
	if status := service.loginStatus(context.Background()); status.Label != "Sign in again" {
		t.Fatal("expired nonrenewable session shown as logged in")
	}
	account.ExpiresAt = time.Now().Add(time.Hour)
	account.Scopes = nil
	set(account, account.ClientID)
	if status := service.loginStatus(context.Background()); status.Label != "Plan access needed" {
		t.Fatal("missing plan grant not reflected in status")
	}
	account.Scopes = strings.Fields(chatGPTScopes)
	set(account, "")
	if status := service.loginStatus(context.Background()); status.Label != "Account saved" {
		t.Fatal("inactive but saved account was not distinguished")
	}
	clearChatGPTTokens(&account)
	set(account, "")
	if status := service.loginStatus(context.Background()); status.Label != "Not logged in" {
		t.Fatal("signed-out registration shown as connected")
	}
}

func TestBrokenCredentialsDoNotHideTheProviderPicker(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENCODE_API_KEY", "")
	service, err := newChatGPTService()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(service.path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(service.path, []byte("damaged-secret-json"), 0600); err != nil {
		t.Fatal(err)
	}
	keyPath, _ := OpenCodeCredentialPath()
	if err := os.WriteFile(keyPath, []byte("damaged key"), 0600); err != nil {
		t.Fatal(err)
	}
	status := ProviderLoginStatus(context.Background(), models.AIConfig{})
	if status["chatgpt"].Label != "Check credentials" || status["opencode-go"].Label != "Check key" {
		t.Fatalf("damaged credentials status=%v", status)
	}
}
