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

/** Prune empty or broken RackInstance placeholders that would otherwise mute playback. */
bool ensureSingleRackForTrack (te::AudioTrack& track);

/** Prune empty rack placeholders across the edit without creating rack plugins from read/setup paths. */
bool ensureTrackRackGraphForEdit (te::Edit& edit);

} // namespace vit
