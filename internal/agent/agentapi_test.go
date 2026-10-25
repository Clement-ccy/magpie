package agent

import (
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

func agentAPIHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := provider.Save(provider.Provider{ID: "deepseek", Name: "DeepSeek", Chat: "https://api.deepseek.com/v1", Key: "k", Models: []string{"pro", "flash"}}); err != nil {
		t.Fatal(err)
	}
}

// magpieModels: the API the user set for the agent wins over the one the
// model's provider says, and Auto gives the provider's back.
func TestMagpieModelsAgentAPI(t *testing.T) {
	agentAPIHome(t)
	apis := func(agent string) map[string][]string {
		m := map[string][]string{}
		for _, c := range magpieModels(agent) {
			m[c.ID] = c.APIs
		}
		return m
	}
	if got := apis("hermes"); len(got["deepseek/pro"]) != 1 || got["deepseek/pro"][0] != "chat" {
		t.Fatalf("as its provider says: %v", got)
	}
	if err := provider.SetAgentAPI("hermes", "deepseek/pro", "responses"); err != nil {
		t.Fatal(err)
	}
	got := apis("hermes")
	if len(got["deepseek/pro"]) != 1 || got["deepseek/pro"][0] != "responses" {
		t.Fatalf("the user's: %v", got)
	}
	if len(got["deepseek/flash"]) != 1 || got["deepseek/flash"][0] != "chat" {
		t.Fatalf("the one left on Auto: %v", got)
	}
	if other := apis("codex"); len(other["deepseek/pro"]) != 1 || other["deepseek/pro"][0] != "chat" {
		t.Fatalf("another agent: %v", other)
	}
	// AskedOn is what the list shows beside Auto: no pick of the user's
	if asked := AskedOn("hermes"); asked["deepseek/pro"] != "chat" || asked["deepseek/flash"] != "chat" {
		t.Fatalf("asked on with Auto: %v", asked)
	}
	if err := provider.SetAgentAPI("hermes", "deepseek/pro", ""); err != nil {
		t.Fatal(err)
	}
	if got := apis("hermes"); len(got["deepseek/pro"]) != 1 || got["deepseek/pro"][0] != "chat" {
		t.Fatalf("after Auto: %v", got)
	}
}
