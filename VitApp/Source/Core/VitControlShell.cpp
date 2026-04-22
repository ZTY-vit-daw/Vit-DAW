#include "VitControlShell.h"

namespace vit
{

juce::var VitControlShell::buildShellDescriptor (const juce::String& templateRole,
                                                 const juce::Array<juce::var>& recommendedGroups)
{
    auto shell = std::make_unique<juce::DynamicObject>();
    juce::Array<juce::var> defaultTabs;
    juce::Array<juce::var> advancedTabs;

    defaultTabs.add ("Parameters");
    defaultTabs.add ("Search");
    defaultTabs.add ("Recommended");

    advancedTabs.add ("Template");
    advancedTabs.add ("Link");
    advancedTabs.add ("History");
    advancedTabs.add ("Macro");
    advancedTabs.add ("Freeze");
    advancedTabs.add ("Bypass");

    shell->setProperty ("template_role", templateRole);
    shell->setProperty ("default_tabs", juce::var (defaultTabs));
    shell->setProperty ("advanced_tabs", juce::var (advancedTabs));
    shell->setProperty ("recommended_groups", juce::var (recommendedGroups));
    shell->setProperty ("supports_param_grabber", true);
    return juce::var (shell.release());
}

} // namespace vit
