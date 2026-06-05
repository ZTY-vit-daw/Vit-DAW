#pragma once

#include <JuceHeader.h>

namespace vit
{

// ===================================================================
// VitPluginGrabberGlobalProfile ? app-wide plugin grabber profile
// storage under Workspace/plugin_grabber_profiles/.
//
// Each known plugin gets one <profileKey>.json file so external
// tools and cross-project reuse are straightforward.
// ===================================================================
class VitPluginGrabberGlobalProfile final
{
public:
    /** Root directory: <workspace>/plugin_grabber_profiles/ */
    static juce::File getGlobalProfilesDirectory();

    /** Filename for a given profile_key (lower-case hex id). */
    static juce::String getProfileFilename (const juce::String& profileKey);

    /** Find a global profile by profile_key.  Returns an empty var if not found. */
    static juce::var findGlobalProfile (const juce::String& profileKey);

    /** Write (upsert) a profile to the global directory.
        profileContent is the full profile var tree (already includes
        plugin_identity, project_default, and any extended fields). */
    static juce::Result writeGlobalProfile (const juce::String& profileKey,
                                            const juce::var& profileContent);

    /** Remove a profile from the global directory. */
    static bool removeGlobalProfile (const juce::String& profileKey);

    /** Read all profiles from the global directory (for snapshot). */
    static juce::Array<juce::var> snapshotGlobalProfiles();

private:
    VitPluginGrabberGlobalProfile() = default;
};

} // namespace vit
