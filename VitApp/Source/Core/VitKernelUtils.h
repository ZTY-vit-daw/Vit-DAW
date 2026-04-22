#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "../Bridge/VitSemanticBridge.h"

namespace vit
{

namespace te = tracktion;

std::unique_ptr<te::Edit> loadEditFromXmlString (te::Engine& engine,
                                                 const juce::String& xmlText,
                                                 uint32_t numAudioTracks = 1,
                                                 VitSemanticBridge* semanticBridge = nullptr,
                                                 const juce::File& pathResolutionFile = {});

std::unique_ptr<te::Edit> loadEditFromXmlFile (te::Engine& engine,
                                               const juce::File& xmlFile,
                                               uint32_t numAudioTracks = 1,
                                               VitSemanticBridge* semanticBridge = nullptr);

/** After loading an Edit from XML/disk, rebind every wave clip to a direct file and refresh playback nodes. */
void rebindAllWaveClipSourcesToDirectFiles (te::Edit& edit);

/** Ensure an audio track owns a single RackInstance and prune duplicate empty placeholders. */
bool ensureSingleRackForTrack (te::AudioTrack& track);

/** Ensure every audio track in the edit exposes a rack graph that can be serialized to UI clients. */
bool ensureTrackRackGraphForEdit (te::Edit& edit);

} // namespace vit
