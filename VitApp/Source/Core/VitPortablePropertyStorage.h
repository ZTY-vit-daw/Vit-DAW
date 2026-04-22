#pragma once

#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitPortablePropertyStorage final : public te::PropertyStorage
{
public:
    explicit VitPortablePropertyStorage (juce::String applicationName);

    juce::File getAppCacheFolder() override;
    juce::File getAppPrefsFolder() override;
    void flushSettingsToDisk() override;
    juce::File getDefaultLoadSaveDirectory (juce::StringRef label) override;
    juce::File getDefaultLoadSaveDirectory (te::ProjectItem::Category category) override;

private:
    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitPortablePropertyStorage)
};

} // namespace vit
