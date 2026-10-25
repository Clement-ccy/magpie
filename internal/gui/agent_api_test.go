package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/agent"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// The API an agent asks a model on: set from its model list, unanswered for an
// agent whose own files can't say it model by model, and Auto keeps nothing.
func TestAgentAPIHandler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"m1", "m2"}}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	agentModelsAPI(mux)
	call := func(method, target, body string) (int, string) {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)))
		return w.Code, w.Body.String()
	}
	if code, body := call("POST", "/api/agent-api", `{"for":"dsh","ref":"relay/m1","api":"anthropic"}`); code != 200 || !strings.Contains(body, `"api":"anthropic"`) {
		t.Fatalf("%d %s", code, body)
	}
	if got := settings.Load().AgentAPIs["dsh/relay/m1"]; got != "anthropic" {
		t.Fatalf("saved %q", got)
	}
	code, body := call("GET", "/api/agent-models/dsh", "")
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	var out struct {
		Models   []agentModelJSON
		PicksAPI bool
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	var m agentModelJSON
	for _, x := range out.Models {
		if x.ID == "relay/m1" {
			m = x
		}
	}
	if !out.PicksAPI || m.API != "anthropic" || m.Auto != "chat" {
		t.Fatalf("picksAPI %v, %+v", out.PicksAPI, m)
	}
	// Auto: the answer goes, and the model says what its provider says
	if code, body := call("POST", "/api/agent-api", `{"for":"dsh","ref":"relay/m1","api":""}`); code != 200 || !strings.Contains(body, `"api":""`) {
		t.Fatalf("%d %s", code, body)
	}
	if len(settings.Load().AgentAPIs) != 0 {
		t.Fatalf("kept: %v", settings.Load().AgentAPIs)
	}
	// an API no agent's files say per model
	if code, _ := call("POST", "/api/agent-api", `{"for":"dsh","ref":"relay/m1","api":"gemini"}`); code == 200 {
		t.Fatal("gemini taken")
	}
	// an agent whose own files can't say it model by model
	code, body = call("POST", "/api/agent-api", `{"for":"codex","ref":"relay/m1","api":"responses"}`)
	if code == 200 || !strings.Contains(body, "can't say which API") {
		t.Fatalf("%d %s", code, body)
	}
	agent.SyncCatalogSettled()
}
