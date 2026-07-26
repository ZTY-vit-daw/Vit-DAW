#pragma once

#include <JuceHeader.h>
#include "VitPluginGrabberProfileFormat.h"
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitPluginGrabberProjectProfile final
{
public:
    struct MergeResult
    {
        // --- Project-level (existing) ---
        bool profileApplied = false;
        juce::String profileSource = "heuristic";
        juce::StringArray quickControlIds;
        juce::StringArray staleParamIds;
        juce::var profile;

        // --- Global / extended profile (Layer 2) ---
        bool globalProfileApplied = false;
        juce::String globalProfileSource;
        juce::var globalProfile;

        juce::String pluginClass;
        juce::Array<juce::var> groups;
        juce::Array<juce::var> virtualControls;
        juce::var safety;

        bool hasExtendedFields() const noexcept
        {
            return pluginClass.isNotEmpty() || groups.size() > 0
                || virtualControls.size() > 0 || safety.isObject();
        }
    };

    static juce::var createPluginIdentity (const te::ExternalPlugin& plugin, const juce::String& pluginId);
    static juce::String profileKeyForPlugin (const te::ExternalPlugin& plugin);

    static juce::Array<juce::var> snapshotProfiles (const juce::File& projectFile);
    static juce::Result upsertProjectDefault (const juce::File& projectFile,
                                              const juce::DynamicObject& object,
                                              const juce::var& pluginIdentity,
                                              juce::var& outProfile);
    static juce::Result removeProfile (const juce::File& projectFile,
                                       const juce::String& profileId,
                                       bool& removed);

    /** Populate the global-profile fields of a MergeResult from the cached global store.
        liveSignature gates the merge against a stale parameter surface, mirroring the
        project-profile signature check in applyProjectDefault. */
    static void populateGlobalInfo (const juce::var& pluginIdentity, MergeResult& result, const juce::String& liveSignature);

    static MergeResult applyProjectDefault (const juce::File& projectFile,
                                            const juce::var& pluginIdentity,
                                            juce::Array<juce::var>& parameterDescriptors);
    static juce::Array<juce::var> buildQuickControlsForIds (const juce::Array<juce::var>& parameterDescriptors,
                                                            const juce::StringArray& quickControlIds);
};

} // namespace vit
