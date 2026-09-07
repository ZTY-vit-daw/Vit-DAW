package harness

import "context"

// ActiveAudioTrack is one shadow-visible user audio track that carries
// material, resolved by the same rule the CCB masking observation uses.
type ActiveAudioTrack struct {
	ID   string
	Name string
}

// ActiveAudioTracks returns the shadow-visible audio tracks (folder/container
// tracks excluded, clips or an audio-track flag required) in stable ID order.
// Consumers must treat this as observation, never as target selection: the
// free-state targeting-coverage gate uses it only to know which tracks exist,
// not which track deserves attention.
func (h *Harness) ActiveAudioTracks(ctx context.Context) []ActiveAudioTrack {
	if h == nil {
		return nil
	}
	ids, names := maskingObservationTracks(h.UserStateSummary(ctx))
	out := make([]ActiveAudioTrack, 0, len(ids))
	for _, id := range ids {
		out = append(out, ActiveAudioTrack{ID: id, Name: names[id]})
	}
	return out
}
