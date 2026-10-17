#pragma once

#include <functional>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

// KERNEL-SENDROUTE-IMPL-1 (K1): the send/return inter-track routing command
// face (MIX-SENDRETURN-DESIGN-1 §1 command schema + §9 GUI-final semantics).
//
// Dual-bus sandwich on tracktion AuxSend/AuxReturn primitives: an FX track is a
// plain AudioTrack annotated with vit_fx_role="send_return_fx" plus a
// vit_send_bus / vit_return_bus int pair. Tracks without the annotation are
// ordinary tracks (fail-open, §11 compatibility).
//
//   source track:   [content..., SendOut(send_bus, amount), SendIn(return_bus), vol, meter]
//   FX track:       [AuxReturn(send_bus), RackInstance..., SendOut(return_bus, 0dB), vol(-96dB), meter]
//
// The wet signal only flows back through the return bus (the FX track's own
// master feed is pulled to -96dB). Chain order is enforced by construction
// (SendOut strictly before SendIn of the same pair) — the anti-loop gates are:
//   1. command-face topology validation (FX tracks can never be send sources,
//      plus a reachability check so a new edge can never close a cycle);
//   2. chain-order enforcement (this service only ever emits ordered pairs);
//   3. the engine's graph-level send-node culling as the final backstop.
class SendRouteService final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using SaveProjectAction = std::function<bool()>;

    SendRouteService (EditGetter editGetter, SaveProjectAction saveProjectAction);
    ~SendRouteService();

    juce::String handleConnect (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetAmount (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDisconnect (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleList (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetFxRole (const juce::DynamicObject&, const juce::String&) const;

    /** Re-assert FX-track chain order (AuxReturn head / rack / SendOut / volume).
     *  Called after rack mutations that may insert, remove or move RackInstances
     *  on an FX track; idempotent, returns true when a reorder happened. */
    bool normaliseFxChainsForEdit (te::Edit&) const;

private:
    static juce::String makeErrorReply (const juce::String& message);

    EditGetter getEdit;
    SaveProjectAction saveProject;
};

} // namespace vit
