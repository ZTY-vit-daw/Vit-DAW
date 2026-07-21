package vsphub

import "sort"

type CapabilityRegistry struct {
	roleCaps map[string]map[string]bool
}

func DefaultCapabilityRegistry() *CapabilityRegistry {
	registry := &CapabilityRegistry{roleCaps: map[string]map[string]bool{}}
	registry.allow("gui",
		"session.hello", "session.heartbeat",
		"command.request", "command.batch",
		"state.snapshot", "state.delta", "state.resync", "state.subscribe",
		"realtime.subscribe", "realtime.frame_request", "realtime.unsubscribe",
		"asset.request", "asset.reference", "asset.manifest_request", "asset.manifest", "asset.materialize_request", "asset.materialized",
		"event.subscribe", "event.poll",
	)
	registry.allow("agent",
		"session.hello", "session.heartbeat",
		"command.request", "command.batch",
		"state.snapshot", "state.delta", "state.resync", "state.subscribe",
		"realtime.subscribe", "realtime.frame_request", "realtime.unsubscribe", "realtime.publish",
		"asset.request", "asset.reference", "asset.manifest_request", "asset.manifest", "asset.materialize_request", "asset.materialized",
		"event.subscribe", "event.poll",
	)
	registry.allow("kernel",
		"session.hello", "session.heartbeat",
		"state.delta",
		"realtime.publish",
		"asset.reference", "asset.manifest",
		"event.subscribe", "event.poll",
	)
	registry.allow("extension",
		"session.hello", "session.heartbeat",
		"extension.register", "extension.unregister",
		"state.snapshot", "state.delta", "state.resync", "state.subscribe",
		"asset.request", "asset.reference", "asset.manifest_request", "asset.manifest", "asset.materialize_request", "asset.materialized",
		"event.subscribe", "event.poll",
	)
	registry.allow("tool",
		"session.hello", "session.heartbeat",
		"state.snapshot", "asset.request", "asset.manifest_request", "asset.materialize_request", "event.subscribe", "event.poll",
	)
	registry.allow("controller",
		"session.hello", "session.heartbeat", "command.request", "state.snapshot", "event.subscribe",
	)
	registry.allow("analyzer",
		"session.hello", "session.heartbeat", "state.snapshot", "asset.request", "asset.manifest_request", "asset.materialize_request", "event.subscribe", "event.poll",
	)
	return registry
}

func (r *CapabilityRegistry) allow(role string, caps ...string) {
	role = normalizeRole(role)
	if r.roleCaps[role] == nil {
		r.roleCaps[role] = map[string]bool{}
	}
	for _, cap := range caps {
		r.roleCaps[role][cap] = true
	}
}

func (r *CapabilityRegistry) CapabilitiesForRole(role string) []string {
	if r == nil {
		return nil
	}
	caps := r.roleCaps[normalizeRole(role)]
	if len(caps) == 0 {
		caps = r.roleCaps["extension"]
	}
	out := make([]string, 0, len(caps))
	for cap := range caps {
		out = append(out, cap)
	}
	sort.Strings(out)
	return out
}

func (r *CapabilityRegistry) Allows(role, capability string) bool {
	if r == nil {
		return false
	}
	caps := r.roleCaps[normalizeRole(role)]
	return caps[capability]
}

func capabilityForEnvelope(env *Envelope) string {
	switch env.Channel() {
	case "session":
		return env.Type()
	case "extension":
		return env.Type()
	case "command":
		if env.Type() == "command.batch" {
			return "command.batch"
		}
		return "command.request"
	case "state":
		switch env.Type() {
		case "state.delta_request":
			return "state.delta"
		case "state.resync_request":
			return "state.resync"
		case "state.subscribe":
			return "state.subscribe"
		default:
			return "state.snapshot"
		}
	case "realtime":
		switch env.Type() {
		case "realtime.publish":
			return "realtime.publish"
		case "realtime.frame_request":
			return "realtime.frame_request"
		case "realtime.unsubscribe":
			return "realtime.unsubscribe"
		default:
			return "realtime.subscribe"
		}
	case "asset":
		switch env.Type() {
		case "asset.materialize_request":
			return "asset.materialize_request"
		case "asset.materialized":
			return "asset.materialized"
		case "asset.manifest_request":
			return "asset.manifest_request"
		case "asset.reference":
			return "asset.reference"
		case "asset.manifest":
			return "asset.manifest"
		default:
			return "asset.request"
		}
	case "event":
		if env.Type() == "event.poll" {
			return "event.poll"
		}
		return "event.subscribe"
	default:
		return env.Channel() + "." + env.Type()
	}
}
