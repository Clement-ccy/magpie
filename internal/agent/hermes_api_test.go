package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/provider"
	"gopkg.in/yaml.v3"
)

// Hermes takes a provider entry's api_mode for every model it serves, so a
// model the user asks on another API goes into an entry of its own, and the
// model names it: magpie-responses for Responses, magpie-anthropic for
// Anthropic, the chat entry as before for the rest.
func TestHermesAPIs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HERMES_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := provider.Save(provider.Provider{ID: "deepseek", Name: "DeepSeek", Chat: "https://api.deepseek.com/v1", Key: "k", Models: []string{"pro", "flash"}}); err != nil {
		t.Fatal(err)
	}
	dir := appdir.HermesHomeIn(home)
	path := filepath.Join(dir, "config.yaml")
	read := func() (map[string]any, map[string]any, string) {
		var c map[string]any
		b, _ := os.ReadFile(path)
		yaml.Unmarshal(b, &c)
		m, _ := c["model"].(map[string]any)
		return c, m, string(b)
	}
	providers := func(c map[string]any) map[string]any { p, _ := c["providers"].(map[string]any); return p }
	entry := func(c map[string]any, id string) map[string]any { e, _ := providers(c)[id].(map[string]any); return e }
	models := func(e map[string]any) []string {
		var out []string
		if e == nil {
			return out
		}
		xl, _ := e["models"].([]any)
		for _, x := range xl {
			out = append(out, x.(string))
		}
		return out
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(path, []byte("model:\n  default: anthropic/claude-opus-5\n  provider: openrouter\nproviders:\n  mine:\n    base_url: https://x\n"), 0o644)

	if err := provider.SetAgentAPI("hermes", "deepseek/pro", "responses"); err != nil {
		t.Fatal(err)
	}
	a := hermes(home)
	f := a.Field("model")
	if err := f.Set("magpie/deepseek/pro"); err != nil {
		t.Fatal(err)
	}
	c, m, raw := read()
	if m["provider"] != "magpie-responses" || m["default"] != "deepseek/pro" {
		t.Fatalf("model:\n%s", raw)
	}
	chat, resp := entry(c, "magpie"), entry(c, "magpie-responses")
	if chat == nil || resp == nil || chat["api_mode"] != "chat_completions" || resp["api_mode"] != "codex_responses" ||
		!strings.HasSuffix(resp["base_url"].(string), "/v1") {
		t.Fatalf("entries:\n%s", raw)
	}
	if got := strings.Join(models(chat), ","); got != "deepseek/flash" {
		t.Fatalf("the chat entry serves %q", got)
	}
	if got := strings.Join(models(resp), ","); got != "deepseek/pro" {
		t.Fatalf("the Responses entry serves %q", got)
	}
	if entry(c, "magpie-anthropic") != nil || providers(c)["mine"] == nil {
		t.Fatalf("an entry with nothing in it, or the user's taken:\n%s", raw)
	}
	if got := f.Get(); got != "magpie/deepseek/pro" {
		t.Fatalf("get %q", got)
	}

	// Auto: back into the chat entry, the one it served going
	if err := provider.SetAgentAPI("hermes", "deepseek/pro", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	c, m, raw = read()
	if m["provider"] != "magpie" || entry(c, "magpie-responses") != nil || !strings.Contains(strings.Join(models(entry(c, "magpie")), ","), "deepseek/pro") {
		t.Fatalf("after Auto:\n%s", raw)
	}

	// the model moved to Anthropic: an entry of its own, the model repointed
	if err := provider.SetAgentAPI("hermes", "deepseek/pro", "anthropic"); err != nil {
		t.Fatal(err)
	}
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	c, m, raw = read()
	if m["provider"] != "magpie-anthropic" || entry(c, "magpie-anthropic")["api_mode"] != "anthropic_messages" ||
		strings.Join(models(entry(c, "magpie")), ",") != "deepseek/flash" {
		t.Fatalf("on Anthropic:\n%s", raw)
	}

	// Disconnect takes every entry magpie wrote
	if err := a.Unwire(); err != nil {
		t.Fatal(err)
	}
	c, _, raw = read()
	if providers(c)["magpie"] != nil || providers(c)["magpie-responses"] != nil || providers(c)["magpie-anthropic"] != nil {
		t.Fatalf("left behind:\n%s", raw)
	}
}
