package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/yetone/magpie/internal/settings"
)

// An agent may be asked on one API for some of magpie's models and another for
// others, set model by model on the Agents page (settings.AgentAPIs). Unlike a
// model's own API (SetModelAPI), which says what the model itself answers on,
// this says only what the agent and the gateway say to each other: the agent's
// own files are written each model under the API asked for there — DeepSeek
// Harness's routes, Pi's and omp's models — and the gateway relays what it is
// asked to the API the model's provider speaks. So no URL of the provider's is
// needed for it, as one is for a model's own API: the gateway translates.

// AgentAPI is the API the user set for the agent to ask id on, "" when they set
// none: the model is asked as its provider says (Provider.Native).
func AgentAPI(agent, id string) Protocol {
	if agent == "" || id == "" {
		return ""
	}
	if s := heldSettings(); len(s.AgentAPIs) > 0 {
		proto := Protocol(s.AgentAPIs[agentAPIKey(agent, id)])
		if slices.Contains(Protocols, proto) {
			return proto
		}
	}
	return ""
}

// agentAPIKey is where the user's answer for agent's id is kept: the agent's
// id and the entry's, as the settings' keys are.
func agentAPIKey(agent, id string) string {
	return strings.ToLower(strings.TrimSpace(agent)) + "/" + strings.TrimSpace(id)
}

// SetAgentAPI says the API the agent asks id on — chat, responses or
// anthropic — for an agent whose own settings can say it per model; "" leaves
// it as its provider says. The agents are told (catalog.Touched), so the ones
// that keep model files write them again with it.
func SetAgentAPI(agent, id, api string) error {
	return touchedIf(setAgentAPI(agent, id, api))
}

func setAgentAPI(agent, id, api string) (bool, error) {
	agent = strings.ToLower(strings.TrimSpace(agent))
	id = strings.TrimSpace(id)
	if agent == "" {
		return false, errors.New("no agent")
	}
	if id == "" {
		return false, errors.New("no model")
	}
	proto := Protocol(strings.TrimSpace(api))
	if proto != "" && !slices.Contains(Protocols, proto) {
		// Gemini is a URL a model is asked on by magpie itself, which no
		// agent's own files say per model
		return false, fmt.Errorf("an agent asks a model on chat, responses or anthropic, not %q", api)
	}
	listed, _ := ListedFor(agent)
	if !slices.ContainsFunc(listed, func(e Entry) bool { return e.ID == id }) {
		return false, fmt.Errorf("magpie shows %s no model %s", agent, id)
	}
	s := settings.Load()
	key := agentAPIKey(agent, id)
	if s.AgentAPIs[key] == string(proto) {
		return false, nil
	}
	if proto == "" {
		delete(s.AgentAPIs, key)
	} else {
		if s.AgentAPIs == nil {
			s.AgentAPIs = map[string]string{}
		}
		s.AgentAPIs[key] = string(proto)
	}
	if err := settings.Save(s); err != nil {
		return false, err
	}
	return true, nil
}
