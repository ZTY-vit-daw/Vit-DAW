// Package instruments is the agent-side instrument semantic face for
// SRC-PRESET-AGENT-1 leg A. It maps instrument words (Chinese or English)
// and General MIDI program names onto installed instrument candidates and
// Surge XT factory preset hints. It is deterministic, read-only, and
// fail-honest: when no candidate exists for an instrument class it returns a
// typed degradation instead of inventing one.
//
// This package has no kernel, VST hosting, or parameter-write authority; the
// runtime preset application channel (kernel plugin_load_preset, pool card
// 291) and the template state-blob path are separate consumers of the
// matching results produced here.
package instruments
