#include "VitZoneBufferAdapter.h"

#include "VitGraphValidator.h"

namespace vit
{

VitZoneBufferAdapterAdvice VitZoneBufferAdapter::describeConnection (te::Plugin* sourcePlugin,
                                                                     te::Plugin* destPlugin,
                                                                     const juce::String& sourceZoneId,
                                                                     const juce::String& destZoneId)
{
    VitZoneBufferAdapterAdvice advice;

    if (sourcePlugin == nullptr || destPlugin == nullptr)
    {
        advice.note = "Source and destination plugins must be present for zone bridge analysis";
        return advice;
    }

    const auto sourceZone = VitGraphValidator::normaliseZoneId (sourceZoneId);
    const auto destZone = VitGraphValidator::normaliseZoneId (destZoneId);
    const bool destTakesMidi = destPlugin->takesMidiInput() || destPlugin->isSynth();

    if (sourceZone == "Z1" && destZone == "Z2")
    {
        advice.mode = "native_midi_to_instrument";
        advice.passthroughMidi = true;
        advice.note = "MIDI-domain routing flows natively into the instrument layer";
        return advice;
    }

    if (sourceZone == "Z2" && destZone == "Z3")
    {
        advice.mode = "native_audio_flow";
        advice.passthroughAudio = true;
        advice.note = "Instrument audio flows natively into the audio-effect layer";
        return advice;
    }

    if (sourceZone == "Z1" && ! destTakesMidi)
    {
        advice.mode = "dummy_audio_padding";
        advice.requiresDummyAudioPadding = true;
        advice.passthroughMidi = true;
        advice.note = "An audio-oriented node fed from Z1 should receive a silent audio buffer while MIDI continues downstream";
        return advice;
    }

    if (destZone == "Z3" && ! destTakesMidi)
    {
        advice.mode = "audio_fx_passthrough";
        advice.passthroughAudio = true;
        advice.note = "Destination is treated as an audio-stage processor";
        return advice;
    }

    advice.passthroughAudio = true;
    advice.passthroughMidi = destTakesMidi;
    return advice;
}

} // namespace vit
