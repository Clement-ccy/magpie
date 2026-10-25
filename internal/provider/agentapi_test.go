package provider

import (
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

// An agent may be set to ask one of magpie's models on an API of its own
// choosing, unlike a model's own API which is the same for every agent. Auto
// leaves it as its provider says, and the pick follows its provider's rename.
func TestSetAgentAPI(t *testing.T) {
	fastHome(t)
	if err := SetAgentAPI("Hermes", "oa/gpt-6.1-sol", "responses"); err != nil {
		t.Fatal(err)
	}
	if got := AgentAPI("hermes", "oa/gpt-6.1-sol"); got != Responses {
		t.Fatalf("asked on %q", got)
	}
	if got := AgentAPI("codex", "oa/gpt-6.1-sol"); got != "" {
		t.Fatalf("another agent: %q", got)
	}
	if got := AgentAPI("hermes", "an/claude-opus-5-5"); got != "" {
		t.Fatalf("another model: %q", got)
	}
	if got := settings.Load().AgentAPIs["hermes/oa/gpt-6.1-sol"]; got != "responses" {
		t.Fatalf("kept %q", got)
	}
	// only the APIs an agent's own files can say per model
	for _, api := range []string{"gemini", "decide", "nope"} {
		if err := SetAgentAPI("hermes", "oa/gpt-6.1-sol", api); err == nil {
			t.Errorf("%q taken", api)
		}
	}
	if err := SetAgentAPI("hermes", "oa/nope", "chat"); err == nil {
		t.Error("a model it is not shown, set")
	}
	if err := SetAgentAPI("", "oa/gpt-6.1-sol", "chat"); err == nil {
		t.Error("no agent, set")
	}
	if err := SetAgentAPI("hermes", "", "chat"); err == nil {
		t.Error("no model, set")
	}
	// Auto: back to what the provider says, and nothing kept
	if err := SetAgentAPI("hermes", "oa/gpt-6.1-sol", ""); err != nil {
		t.Fatal(err)
	}
	if got := AgentAPI("hermes", "oa/gpt-6.1-sol"); got != "" {
		t.Fatalf("after Auto: %q", got)
	}
	if len(settings.Load().AgentAPIs) != 0 {
		t.Fatalf("kept: %v", settings.Load().AgentAPIs)
	}
	// a pick moves with its provider's rename (#954)
	if err := SetAgentAPI("hermes", "oa/gpt-6.1-sol", "anthropic"); err != nil {
		t.Fatal(err)
	}
	st := settings.Load()
	if !renameModelPrefs(&st, "oa", "openai") {
		t.Fatal("the rename moved nothing")
	}
	if err := settings.Save(st); err != nil {
		t.Fatal(err)
	}
	Changed()
	if got := AgentAPI("hermes", "openai/gpt-6.1-sol"); got != Anthropic {
		t.Fatalf("after the rename: %q", got)
	}
	if got := AgentAPI("hermes", "oa/gpt-6.1-sol"); got != "" {
		t.Fatalf("the old id: %q", got)
	}
}
