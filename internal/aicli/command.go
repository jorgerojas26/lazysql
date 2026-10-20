package aicli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/term"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

const consoleURL = "https://opencode.ai/console"
const defaultGoModel = "glm-5.3-flash"

var errCancelled = errors.New("cancelled")

type interaction interface {
	Provider(string) (string, error)
	Key(string, string, func(string) error) error
	Recover(string) (bool, error)
	Confirm(string, string, bool) (bool, error)
	Model(string, []ai.Model, string) (string, error)
}

type actions struct {
	login     func(context.Context) error
	logout    func(context.Context) error
	saveKey   func(string) error
	deleteKey func() error
	models    func(context.Context, string) ([]ai.Model, error)
}

type command struct {
	ui         interaction
	actions    actions
	configFile string
	out        io.Writer
}

func Run(ctx context.Context, args []string, configFile string, out io.Writer) error {
	usage := "Usage: lazysql ai <login|logout> [--config path]"
	if len(args) == 0 {
		return errors.New(usage)
	}
	if args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, usage)
		return nil
	}
	action := args[0]
	if action != "login" && action != "logout" {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("ai "+action, flag.ContinueOnError)
	flags.SetOutput(out)
	flags.StringVar(&configFile, "config", configFile, "configuration file to update after explicit consent")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New(usage)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("lazysql ai " + action + " requires an interactive terminal")
	}
	ui := &terminalUI{ctx: ctx, configFile: configFile}
	flow := command{ui: ui, configFile: configFile, out: out, actions: actions{
		login:     ui.loginChatGPT,
		logout:    ui.logoutChatGPT,
		saveKey:   ai.SaveOpenCodeKey,
		deleteKey: ai.DeleteOpenCodeKey,
		models: func(ctx context.Context, provider string) ([]ai.Model, error) {
			if provider == "chatgpt" {
				return ai.ChatGPTModels(ctx)
			}
			return ai.OpenCodeModels(ctx)
		},
	}}
	return flow.run(ctx, action)
}

func (flow *command) run(ctx context.Context, action string) error {
	for {
		theme, err := app.ReadThemeConfig(flow.configFile)
		if err == nil {
			err = app.ApplyTheme(theme)
		}
		if err == nil {
			err = flow.execute(ctx, action)
		}
		if err == nil {
			return nil
		}
		if errors.Is(err, errCancelled) || ctx.Err() != nil {
			fmt.Fprintln(flow.out, "Cancelled.")
			return nil
		}
		retry, recoveryError := flow.ui.Recover(err.Error())
		if errors.Is(recoveryError, errCancelled) {
			return nil
		}
		if recoveryError != nil {
			return recoveryError
		}
		if !retry {
			return nil
		}
	}
}

func (flow *command) execute(ctx context.Context, action string) error {
	provider, err := flow.ui.Provider(action)
	if err != nil {
		return err
	}
	if provider != "chatgpt" && provider != "opencode-go" {
		return errors.New("unknown AI provider")
	}
	if action == "logout" {
		return flow.logout(ctx, provider)
	}
	switch provider {
	case "chatgpt":

		if err := flow.actions.login(ctx); err != nil {
			return err
		}
	case "opencode-go":
		if err := flow.ui.Key("Connect OpenCode Go", consoleURL, flow.actions.saveKey); err != nil {
			return err
		}
		fmt.Fprintln(flow.out, "OpenCode Go subscription key saved privately; it has not been validated with an inference request.")
	}
	fmt.Fprintln(flow.out, "Authentication saved. Logging in does not enable AI or change your default provider.")
	return flow.configure(ctx, provider)
}

func (flow *command) configure(ctx context.Context, provider string) error {
	config, err := app.ReadAISettings(flow.configFile)
	if err != nil {
		return fmt.Errorf("authentication saved, but cannot read AI settings: %w", err)
	}
	providerName := map[string]string{"chatgpt": "ChatGPT", "opencode-go": "OpenCode Go"}[provider]
	message := "Use " + providerName + " by default?"
	if config.Enabled {
		message += "\nAI is enabled."
	}
	makeDefault, err := flow.ui.Confirm("Default AI provider", message, true)
	if err != nil {
		return err
	}
	changes := map[string]any{}
	if makeDefault {
		changes["provider"] = provider
		current := ""
		if provider == "opencode-go" {
			current = defaultGoModel
		}
		if config.Provider == provider && config.Model != "" {
			current = config.Model
		}
		fmt.Fprintln(flow.out, "Loading "+providerName+" models…")
		models, err := flow.actions.models(ctx, provider)
		if err != nil {
			return err
		}
		if len(models) == 0 {
			return errors.New("no models available for " + providerName)
		}
		currentAvailable := false
		for _, model := range models {
			if model.ID == current {
				currentAvailable = true
				break
			}
		}
		if !currentAvailable {
			current = models[0].ID
		}
		selected, err := flow.ui.Model(providerName, models, current)
		if err != nil {
			return err
		}
		found := false
		for _, model := range models {
			if model.ID != selected {
				continue
			}
			changes["model"] = model.ID
			changes["api_format"] = nil
			if provider == "opencode-go" {
				changes["api_format"] = model.Format
				changes["api_key_env"] = nil
			}
			found = true
			break
		}
		if !found {
			return errors.New("select an available " + providerName + " model")
		}
		config.Provider = provider
	}
	if !config.Enabled && config.Provider != "" {
		enable, err := flow.ui.Confirm("Enable AI?", "Allow AI SQL generation using "+config.Provider+"?\nOnly schema context is sent by default, after an explicit generation request.\nThis writes enabled = true in:\n"+flow.configFile, false)
		if err != nil {
			return err
		}
		if enable {
			changes["enabled"] = true
			config.Enabled = true
		}
	}
	if len(changes) > 0 {
		if err := app.UpdateAISettings(flow.configFile, changes); err != nil {
			return fmt.Errorf("authentication saved, but cannot update AI settings: %w", err)
		}
		fmt.Fprintln(flow.out, "AI settings saved to", flow.configFile)
		if local, err := app.FindLocalConfig(); err == nil && local != "" && !samePath(local, flow.configFile) {
			fmt.Fprintln(flow.out, "Local settings in", local, "may override these AI settings.")
		}
	}
	if !config.Enabled {
		fmt.Fprintln(flow.out, "AI remains disabled.")
	}
	if provider == "opencode-go" {
		env := config.APIKeyEnv
		if makeDefault || env == "" {
			env = "OPENCODE_API_KEY"
		}
		if os.Getenv(env) != "" {
			fmt.Fprintln(flow.out, env, "is set and takes precedence over the saved subscription key.")
		}
	}
	return nil
}

func (flow *command) logout(ctx context.Context, provider string) error {
	message := "Remove the OpenCode Go subscription key saved by LazySQL?\nThis does not revoke the key or remove environment variables."
	if provider == "chatgpt" {
		message = "Sign out of the active ChatGPT account in LazySQL?"
	}
	confirmed, err := flow.ui.Confirm("Sign out of "+provider, message, false)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(flow.out, "Logout cancelled. Credentials are unchanged.")
		return nil
	}
	switch provider {
	case "chatgpt":
		if err := flow.actions.logout(ctx); err != nil {
			return err
		}
		fmt.Fprintln(flow.out, "Signed out of ChatGPT in LazySQL.")
	case "opencode-go":
		if err := flow.actions.deleteKey(); err != nil {
			return err
		}
		fmt.Fprintln(flow.out, "Removed the saved OpenCode Go subscription key.")
		environments := []string{"OPENCODE_API_KEY"}
		if config, err := app.ReadAISettings(flow.configFile); err == nil && config.APIKeyEnv != "" && config.APIKeyEnv != "OPENCODE_API_KEY" {
			environments = append(environments, config.APIKeyEnv)
		}
		for _, env := range environments {
			if os.Getenv(env) != "" {
				fmt.Fprintln(flow.out, "Authentication is still available through", env+"; unset it in your shell to finish signing out.")
			}
		}
	}
	fmt.Fprintln(flow.out, "AI configuration is unchanged.")
	return nil
}

func samePath(first, second string) bool {
	firstPath, firstErr := filepath.Abs(first)
	secondPath, secondErr := filepath.Abs(second)
	return firstErr == nil && secondErr == nil && firstPath == secondPath
}
