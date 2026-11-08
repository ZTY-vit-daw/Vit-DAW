package chat

import (
	"context"
	"strings"
	"testing"
)

// PLUGIN-CONTROL-EVAL-BAREFREE-1 (card 318) paired seam tests, chat side.
// The B-R1 payload dissection proved T3 (Plates Damper) and T4 (H-Delay
// delay time) generic writes were rejected by the typed transient-shaper
// guard's fail-closed branch ("typed_transient_shaper_surface_unavailable:
// cannot prove this parameter is outside the typed transient-shaper
// surface: unresolved_transient_shaper_surface: ...") — correct encodings
// included. These tests pin both directions on the replayed 300-S0
// forensics fixtures (the same surfaces the T3/T4 rounds hit):
//  1. seam unset / non-"1" → every typed generic-write guard keeps its
//     fail-closed rejection byte-for-byte (production behavior unchanged);
//  2. seam exactly "1" → the same requests pass all seven guards and the
//     guard short-circuits before consulting the kernel surface at all.

func TestBarefreeSeamClosedKeepsTypedGuardsFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		paramID string
		caseRef string
	}{
		{name: "plates damper (B-R1 T3 replay)", fixture: writeguardPlatesReply, paramID: "1", caseRef: "T3"},
		{name: "h-delay delay sec (B-R1 T4 replay)", fixture: writeguardHDelayReply, paramID: "0", caseRef: "T4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, _, req := writeguardFixtureServer(t, tc.fixture)
			req.Args["param_id"] = tc.paramID
			resp, blocked := server.guardTransientShaperOwnedGenericParameterWrite(context.Background(), req)
			if !blocked {
				t.Fatalf("closed seam must keep the transient-shaper guard rejecting the %s write", tc.caseRef)
			}
			if !strings.Contains(resp.Error, "typed_transient_shaper_surface_unavailable") {
				t.Fatalf("closed seam must reproduce the B-R1 rejection code; got %q", resp.Error)
			}
			if !strings.Contains(resp.Error, "unresolved_transient_shaper_surface") {
				t.Fatalf("closed seam must keep the fail-closed unresolved classification; got %q", resp.Error)
			}
		})
	}
}

func TestBarefreeSeamOpenPassesAllTypedGuardsWithoutKernelRead(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	for _, tc := range []struct {
		name    string
		fixture string
	}{{name: "plates", fixture: writeguardPlatesReply}, {name: "h-delay", fixture: writeguardHDelayReply}} {
		t.Run(tc.name, func(t *testing.T) {
			server, fake, req := writeguardFixtureServer(t, tc.fixture)
			type guardFn func(*Server, context.Context) (string, bool)
			invocations := map[string]guardFn{
				"transient-shaper": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardTransientShaperOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
				"gate-expander": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardGateExpanderOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
				"limiter": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardLimiterOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
				"de-esser": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardDeEsserOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
				"compressor": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardCompressorOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
				"eq": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardEQOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
				"multiband": func(s *Server, ctx context.Context) (string, bool) {
					resp, blocked := s.guardMultibandOwnedGenericParameterWrite(ctx, req)
					return resp.Error, blocked
				},
			}
			for guard, invoke := range invocations {
				if errDetail, blocked := invoke(server, context.Background()); blocked {
					t.Fatalf("open seam must pass the %s guard for a generic parameter write; got %q", guard, errDetail)
				}
			}
			if fake.reads != 0 {
				t.Fatalf("open seam must short-circuit before any kernel surface read; reads=%d", fake.reads)
			}
		})
	}
}

func TestBarefreeSeamChatOnlyExactValueOpens(t *testing.T) {
	for _, value := range []string{"", "0", "true", "off", "1x"} {
		t.Setenv(pluginControlEvalSeamBareWriteEnv, value)
		if pluginControlEvalSeamBareWriteOpen() {
			t.Fatalf("env value %q must not open the barewrite seam", value)
		}
		server, _, req := writeguardFixtureServer(t, writeguardHDelayReply)
		if _, blocked := server.guardTransientShaperOwnedGenericParameterWrite(context.Background(), req); !blocked {
			t.Fatalf("env value %q must keep the transient-shaper guard closed", value)
		}
	}
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	if !pluginControlEvalSeamBareWriteOpen() {
		t.Fatal("exactly \"1\" must open the seam")
	}
}
