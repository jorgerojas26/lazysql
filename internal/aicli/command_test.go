package aicli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

type fakeUI struct {
	provider             string
	texts                []string
	confirms             []bool
	confirmationDefaults []bool
	questions            []string
	masked               []bool
	providerErr          error
	modelSelection       string
	modelOptions         []ai.Model
	modelProvider        string
	modelCurrent         string
	textErr              error
	confirmErr           error
	keyErrors            []error
	recovered            []string
	recoverChoices       []bool
	afterRecovery        func()
}

func (ui *fakeUI) Provider(string) (string, error) { return ui.provider, ui.providerErr }
func (ui *fakeUI) Key(title, message string, save func(string) error) error {
	ui.questions = append(ui.questions, title+"\n"+message)
	ui.masked = append(ui.masked, true)
	if ui.textErr != nil {
		return ui.textErr
	}
	for len(ui.texts) > 0 {
		value := ui.texts[0]
		ui.texts = ui.texts[1:]
		if err := save(value); err != nil {
			ui.keyErrors = append(ui.keyErrors, err)
			continue
		}
		return nil
	}
	return errCancelled
}
func (ui *fakeUI) Recover(message string) (bool, error) {
	ui.recovered = append(ui.recovered, message)
	if ui.afterRecovery != nil {
		ui.afterRecovery()
	}
	if len(ui.recoverChoices) == 0 {
		return false, nil
	}
	choice := ui.recoverChoices[0]
	ui.recoverChoices = ui.recoverChoices[1:]
	return choice, nil
}
func (ui *fakeUI) Confirm(title, message string, defaultYes bool) (bool, error) {
	ui.confirmationDefaults = append(ui.confirmationDefaults, defaultYes)
	ui.questions = append(ui.questions, title+"\n"+message)
	if ui.confirmErr != nil {
		return false, ui.confirmErr
	}
	if len(ui.confirms) == 0 {
		return false, errors.New("unexpected confirmation")
	}
	result := ui.confirms[0]
	ui.confirms = ui.confirms[1:]
	return result, nil
}

func (ui *fakeUI) Model(provider string, models []ai.Model, current string) (string, error) {
	ui.modelProvider = provider
	ui.modelOptions = models
	ui.modelCurrent = current
	if ui.modelSelection != "" {
		return ui.modelSelection, nil
	}
	return current, nil
}

func newTestCommand(t *testing.T, ui *fakeUI) (*command, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("OPENCODE_API_KEY", "")
	output := &bytes.Buffer{}
	return &command{ui: ui, configFile: filepath.Join(root, "config.toml"), out: output, actions: actions{
		login:   func(context.Context) error { return errors.New("unexpected ChatGPT login") },
		logout:  func(context.Context) error { return errors.New("unexpected ChatGPT logout") },
		saveKey: ai.SaveOpenCodeKey, deleteKey: ai.DeleteOpenCodeKey,
		models: func(_ context.Context, provider string) ([]ai.Model, error) {
			if provider == "chatgpt" {
				return []ai.Model{{ID: "chatgpt-default", Name: "ChatGPT default"}, {ID: "chatgpt-other", Name: "ChatGPT other"}}, nil
			}
			return []ai.Model{{ID: defaultGoModel, Name: "GLM Flash", Format: "chat-completions"}, {ID: "my-go-model", Name: "My model", Format: "responses"}}, nil
		},
	}}, output
}

func TestGoLoginAuthenticationOnlyDoesNotEnableOrWriteConfig(t *testing.T) {
	ui := &fakeUI{provider: "opencode-go", texts: []string{"subscription-secret"}, confirms: []bool{false}}
	flow, output := newTestCommand(t, ui)
	if err := flow.execute(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(flow.configFile); !os.IsNotExist(err) {
		t.Fatal("authentication-only login wrote configuration")
	}
	key, err := ai.LoadOpenCodeKey()
	if err != nil || key != "subscription-secret" {
		t.Fatal("subscription key was not saved")
	}
	if len(ui.masked) != 1 || !ui.masked[0] {
		t.Fatal("subscription key input was not masked")
	}
	if ui.questions[0] != "Connect OpenCode Go\n"+consoleURL {
		t.Fatal("OpenCode Go key entry should show only the console URL")
	}
	if strings.Contains(output.String(), "subscription-secret") || !strings.Contains(output.String(), "AI remains disabled") {
		t.Fatal("login leaked a key or implicitly enabled AI")
	}
}

func TestGoLoginExplicitDefaultAndEnable(t *testing.T) {
	for _, enable := range []bool{false, true} {
		t.Run(map[bool]string{false: "default only", true: "enable"}[enable], func(t *testing.T) {
			ui := &fakeUI{provider: "opencode-go", texts: []string{"key"}, modelSelection: "my-go-model", confirms: []bool{true, enable}}
			flow, _ := newTestCommand(t, ui)
			if err := flow.execute(context.Background(), "login"); err != nil {
				t.Fatal(err)
			}
			config, err := app.ReadAISettings(flow.configFile)
			if err != nil || config.Provider != "opencode-go" || config.Model != "my-go-model" || config.APIFormat != "responses" || config.Enabled != enable {
				t.Fatalf("AI config = %+v %v", config, err)
			}
			data, _ := os.ReadFile(flow.configFile)
			if strings.Contains(string(data), "subscription-key") || strings.Contains(string(data), "api_key =") {
				t.Fatal("credentials were written into configuration")
			}
			if len(ui.confirmationDefaults) != 2 || !ui.confirmationDefaults[0] || ui.confirmationDefaults[1] {
				t.Fatal("only default-provider confirmation should preselect Yes")
			}
			if !strings.Contains(ui.questions[1], "Use OpenCode Go by default?") || strings.Contains(ui.questions[1], flow.configFile) {
				t.Fatal("default-provider prompt is not concise")
			}
			if len(ui.masked) != 1 || !ui.masked[0] || len(ui.modelOptions) != 2 {
				t.Fatal("model selection must use the catalog picker, not a text field")
			}
		})
	}
}

func TestChatGPTLoginResetsCrossProviderModelOnlyWithConsent(t *testing.T) {
	ui := &fakeUI{provider: "chatgpt", confirms: []bool{true, true}}
	flow, _ := newTestCommand(t, ui)
	if err := os.WriteFile(flow.configFile, []byte("[ai]\nprovider='opencode-go'\nmodel='go-only-model'\nenabled=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	flow.actions.login = func(context.Context) error { calls++; return nil }
	if err := flow.execute(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	config, err := app.ReadAISettings(flow.configFile)
	if err != nil || calls != 1 || !config.Enabled || config.Provider != "chatgpt" || config.Model != "chatgpt-default" {
		t.Fatalf("ChatGPT setup = %+v calls=%d err=%v", config, calls, err)
	}
	if key, err := ai.LoadOpenCodeKey(); err != nil || key != "" {
		t.Fatal("ChatGPT login touched OpenCode credentials")
	}
}

func TestLoginDeclinedChangesPreserveExistingProvider(t *testing.T) {
	ui := &fakeUI{provider: "chatgpt", confirms: []bool{false, false}}
	flow, _ := newTestCommand(t, ui)
	original := "[ai]\nprovider='opencode-go'\nmodel='keep-model'\nenabled=false\n"
	if err := os.WriteFile(flow.configFile, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	flow.actions.login = func(context.Context) error { return nil }
	if err := flow.execute(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(flow.configFile)
	if string(data) != original {
		t.Fatal("declining setup altered configuration")
	}
	if len(ui.questions) != 2 || !strings.Contains(ui.questions[1], "using opencode-go") {
		t.Fatal("enable prompt should identify the existing provider when default change is declined")
	}
}

func TestLogoutOnlyRemovesSelectedProviderAndWarnsAboutEnvironment(t *testing.T) {
	ui := &fakeUI{provider: "opencode-go", confirms: []bool{true}}
	flow, output := newTestCommand(t, ui)
	if err := ai.SaveOpenCodeKey("saved-key"); err != nil {
		t.Fatal(err)
	}
	path, _ := ai.OpenCodeCredentialPath()
	unrelated := filepath.Join(filepath.Dir(path), "unrelated-provider")
	if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	original := "[ai]\nenabled=true\nprovider='opencode-go'\nmodel='model'\n"
	if err := os.WriteFile(flow.configFile, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_API_KEY", "environment-secret")
	if err := flow.execute(context.Background(), "logout"); err != nil {
		t.Fatal(err)
	}
	if key, err := ai.LoadOpenCodeKey(); err != nil || key != "" {
		t.Fatal("saved credential survived logout")
	}
	data, _ := os.ReadFile(flow.configFile)
	if string(data) != original {
		t.Fatal("logout changed AI settings")
	}
	if data, err := os.ReadFile(unrelated); err != nil || string(data) != "keep" {
		t.Fatal("logout touched an unrelated provider")
	}
	if !strings.Contains(output.String(), "unset it in your shell") || strings.Contains(output.String(), "environment-secret") {
		t.Fatal("environment authentication warning missing or leaked a key")
	}
}

func TestChatGPTLogoutRequiresConfirmationAndOnlySignsOutOfLazySQL(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		ui := &fakeUI{provider: "chatgpt", confirms: []bool{confirmed}}
		flow, _ := newTestCommand(t, ui)
		calls := 0
		flow.actions.logout = func(context.Context) error { calls++; return nil }
		if err := ai.SaveOpenCodeKey("keep-go-key"); err != nil {
			t.Fatal(err)
		}
		if err := flow.execute(context.Background(), "logout"); err != nil {
			t.Fatal(err)
		}
		if (calls == 1) != confirmed || !strings.Contains(ui.questions[0], "active ChatGPT account in LazySQL") {
			t.Fatal("LazySQL logout was not explicitly confirmed")
		}
		if key, err := ai.LoadOpenCodeKey(); err != nil || key != "keep-go-key" {
			t.Fatal("ChatGPT logout affected OpenCode authentication")
		}
	}
}

func TestCancelledAndFailedLoginDoNotChangeSettings(t *testing.T) {
	ui := &fakeUI{provider: "chatgpt"}
	flow, _ := newTestCommand(t, ui)
	flow.actions.login = func(context.Context) error { return context.Canceled }
	if err := flow.execute(context.Background(), "login"); !errors.Is(err, context.Canceled) {
		t.Fatalf("login error = %v", err)
	}
	if _, err := os.Stat(flow.configFile); !os.IsNotExist(err) {
		t.Fatal("failed login wrote configuration")
	}
	ui.providerErr = errCancelled
	if err := flow.execute(context.Background(), "login"); !errors.Is(err, errCancelled) {
		t.Fatal("selector cancellation ignored")
	}
}

func TestCancelledSetupKeepsAuthenticationButNotPartialConfig(t *testing.T) {
	ui := &fakeUI{provider: "opencode-go", texts: []string{"saved-secret"}, confirmErr: errCancelled}
	flow, _ := newTestCommand(t, ui)
	if err := flow.execute(context.Background(), "login"); !errors.Is(err, errCancelled) {
		t.Fatal("configuration cancellation ignored")
	}
	if key, err := ai.LoadOpenCodeKey(); err != nil || key != "saved-secret" {
		t.Fatal("completed authentication should be retained")
	}
	if _, err := os.Stat(flow.configFile); !os.IsNotExist(err) {
		t.Fatal("cancelled setup wrote partial configuration")
	}
}

func TestCommandHelpAndInvalidArgumentsDoNotStartUI(t *testing.T) {
	if err := Run(context.Background(), []string{"--help"}, "unused", io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"unknown"}, {"login", "unexpected"}, {"logout", "--bad-flag"}} {
		if err := Run(context.Background(), args, "unused", io.Discard); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
}

func TestCatalogFailurePreservesLoginAndDoesNotWritePartialConfig(t *testing.T) {
	ui := &fakeUI{provider: "opencode-go", texts: []string{"saved-key"}, confirms: []bool{true}}
	flow, _ := newTestCommand(t, ui)
	flow.actions.models = func(context.Context, string) ([]ai.Model, error) { return nil, errors.New("catalog unavailable") }
	if err := flow.execute(context.Background(), "login"); err == nil {
		t.Fatal("catalog failure ignored")
	}
	if key, err := ai.LoadOpenCodeKey(); err != nil || key != "saved-key" {
		t.Fatal("catalog failure lost authentication")
	}
	if _, err := os.Stat(flow.configFile); !os.IsNotExist(err) {
		t.Fatal("catalog failure wrote partial config")
	}
}

func TestChatGPTLoginSelectsAndPersistsAnAuthenticatedModel(t *testing.T) {
	ui := &fakeUI{provider: "chatgpt", modelSelection: "account-other", confirms: []bool{true, false}}
	flow, _ := newTestCommand(t, ui)
	if err := os.WriteFile(flow.configFile, []byte("[ai]\nprovider='opencode-go'\nmodel='old-go-model'\napi_format='anthropic'\nenabled=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	authenticated := false
	flow.actions.login = func(context.Context) error { authenticated = true; return nil }
	flow.actions.models = func(_ context.Context, provider string) ([]ai.Model, error) {
		if !authenticated || provider != "chatgpt" {
			t.Fatal("ChatGPT models must be loaded after subscription authentication")
		}
		return []ai.Model{{ID: "account-default", Name: "Default model"}, {ID: "account-other", Name: "Other model"}}, nil
	}
	if err := flow.execute(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	config, err := app.ReadAISettings(flow.configFile)
	if err != nil || config.Provider != "chatgpt" || config.Model != "account-other" || config.APIFormat != "" || config.Enabled {
		t.Fatalf("ChatGPT model configuration = %+v %v", config, err)
	}
	if ui.modelProvider != "ChatGPT" || ui.modelCurrent != "account-default" || len(ui.modelOptions) != 2 {
		t.Fatal("ChatGPT login did not use the shared model selector or account default")
	}
}

func TestChatGPTLoginPreselectsExistingModel(t *testing.T) {
	ui := &fakeUI{provider: "chatgpt", confirms: []bool{true, false}}
	flow, _ := newTestCommand(t, ui)
	if err := os.WriteFile(flow.configFile, []byte("[ai]\nprovider='chatgpt'\nmodel='chatgpt-other'\nenabled=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	flow.actions.login = func(context.Context) error { return nil }
	if err := flow.execute(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	if ui.modelCurrent != "chatgpt-other" {
		t.Fatal("existing ChatGPT model was not preselected")
	}
}
