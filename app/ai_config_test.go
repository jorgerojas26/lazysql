package app

import "testing"

func TestAIDefaultsOffAndProviderDoesNotEnableIt(t *testing.T) {
	if defaultConfig().AI.Enabled {
		t.Fatal("AI must default to disabled")
	}
	loadProjectConfig(t, "[ai]\nprovider = 'opencode-go'\nmodel = 'test-model'\napi_key_env = 'MY_GO_KEY'\n", "")
	config := App.AIConfig()
	if config.Enabled || config.Provider != "opencode-go" || config.Model != "test-model" || config.APIKeyEnv != "MY_GO_KEY" {
		t.Fatalf("unexpected AI config: %+v", config)
	}
}

func TestAIRequiresExplicitEnableAndLocalDisableWins(t *testing.T) {
	loadProjectConfig(t, "[ai]\nenabled = true\nprovider = 'chatgpt'\n", "[ai]\nenabled = false\n")
	if App.AIConfig().Enabled || App.AIConfig().Provider != "chatgpt" {
		t.Fatalf("local opt-out did not override global config: %+v", App.AIConfig())
	}
}

func TestAIExplicitEnable(t *testing.T) {
	loadProjectConfig(t, "[ai]\nenabled = true\nprovider = 'chatgpt'\n", "")
	if !App.AIConfig().Enabled {
		t.Fatal("explicit AI enable was ignored")
	}
}
