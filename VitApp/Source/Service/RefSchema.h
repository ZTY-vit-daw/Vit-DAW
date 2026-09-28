#pragma once

#include <JuceHeader.h>

namespace vit::refschema
{

// REFSCHEMA-D1: L0 evidence-ref grammar mirror for the kernel C++ side.
//
// Grammar authority (do not edit the format here without syncing it):
//   agent/internal/agentprotocol/refschema.go  — L0 grammar authority
//   coord/decisions/2026-09-27-g1-ref-schema-ruling.md — G1 ruling BNF
//
//   ref := "vit://" kind "/" scope_kind ":" scope_value "/" window
//          "@" snapshot "#" hash
//   window := "t=all" | "t=" sampleStart ".." sampleEnd   (sample authority)
//   hash   := "sha256:" 16hex | "-"                       ("-" = explicit
//                                                          un-CAS-ed)
//
// Ruling #2/#3 tightening: the window and hash segments must never be
// omitted — full-span refs say "t=all" explicitly and kernel generation
// points carry no CAS content hash, so they say "#-".
//
// Segment escaping mirrors the agent-side escapeSegment: the grammar's own
// structural delimiters (% / @ # :) are percent-escaped with uppercase hex
// inside scope and snapshot segments; every other byte (spaces, Unicode
// UTF-8 sequences included) passes through unchanged.

inline constexpr const char* kRefSchemePrefix = "vit://";
inline constexpr const char* kRefHashUnCASed = "-";

// Registered projection kinds used by kernel generation points (compile-time
// mirror of the agent-side registry in agentprotocol/refschema.go; kind names
// follow docs/QUERY_ENGINE_V1_DESIGN.md §3.3).
inline constexpr const char* kKindL3 = "dad.l3";
inline constexpr const char* kKindL2RenderProbe = "dad.l2_render_probe";
inline constexpr const char* kKindCompressorDualTap = "dad.compressor_dual_tap";

// Legacy prefix the D2 generation points emitted before REFSCHEMA-D2 (the
// agent-side com consumers evidence.go/paired.go have since switched to
// ParseRef and accept both shapes in the same frame). Kept for the grace
// period: the T6 golden test locks the literal so it stays byte-identical
// with the agentprotocol legacy-registry entry translating in-flight
// artifacts.
inline constexpr const char* kLegacyPrefixCompressorDualTap = "dad.compressor_dual_tap:";

// scope_kind vocabulary used by the kernel-side builders below.
inline constexpr const char* kScopeKindTrack = "track";
inline constexpr const char* kScopeKindProject = "project";
inline constexpr const char* kScopeKindFeature = "feature";
inline constexpr const char* kScopeKindBand = "band";

// Percent-escapes the grammar's reserved delimiters inside one segment
// (mirror of agent escapeSegment / refEscapeReserved = "%/@#:").
juce::String escapeRefSegment (const juce::String& segment);

// Serializes one L0 ref. allTime selects the explicit "t=all" window;
// otherwise both sample bounds are emitted. The hash segment is always the
// explicit un-CAS-ed marker (kernel generation points reference data-plane
// positions, not CAS content).
juce::String formatEvidenceRef (const juce::String& kind,
                                const juce::String& scopeKind,
                                const juce::String& scopeValue,
                                bool allTime,
                                juce::int64 sampleStart,
                                juce::int64 sampleEnd,
                                const juce::String& snapshot);

// D1 (VitProductionCoordinator.cpp stampL2ProbeIdentity + start reply):
// vit://dad.l2_render_probe/track:<trackId>/t=all@<renderRevision>#-
// A missing track id (master-tap probe) falls back to the whole-project
// scope the payload itself reports as project_id=current.
juce::String makeL2RenderProbeRef (const juce::String& trackId,
                                   const juce::String& renderRevision);

// D3/D5 (L3AcousticAnalyzer.cpp stampCommon + bare-constant evidence_refs):
// vit://dad.l3/feature:<featureName>/t=all@<sourceRevision|filePath>#-
// The snapshot identity keeps the legacy D3 semantics verbatim — source
// revision when present, file path otherwise.
juce::String makeL3FeatureRef (const juce::String& featureName,
                               const juce::String& sourceRevision,
                               const juce::String& filePath);

// D4 (L3AcousticAnalyzer.cpp band rows):
// vit://dad.l3/band:<bandName>/t=all@<sourceRevision|filePath>#-
juce::String makeL3BandRef (const juce::String& bandName,
                            const juce::String& sourceRevision,
                            const juce::String& filePath);

// D2 (VitProductionCoordinator.cpp stampCompressorDualTapIdentity + start
// reply, CompressorDualTapEvidence.cpp artifact ref):
// vit://dad.compressor_dual_tap/track:<trackId>/t=<start>..<end>@<pairId>#-
// The pair id (snapshot identity) is what the agent-side com consumers key
// on; the window is the exact probe sample window. The probe command only
// accepts audio tracks, so there is no master/project fallback.
juce::String makeCompressorDualTapRef (const juce::String& trackId,
                                       juce::int64 sampleStart,
                                       juce::int64 sampleEnd,
                                       const juce::String& pairId);

} // namespace vit::refschema
