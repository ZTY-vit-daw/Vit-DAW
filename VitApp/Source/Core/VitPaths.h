#pragma once

#include <JuceHeader.h>

#ifndef VIT_DEFAULT_PROJECT_XML_RELATIVE
 #define VIT_DEFAULT_PROJECT_XML_RELATIVE "Workspace/default_project.xml"
#endif

namespace vit::paths
{

namespace detail
{
inline juce::String getDefaultProjectXmlTemplate()
{
    return R"VIT(<?xml version='1.0' encoding='utf-8'?>
<EDIT appVersion="Unknown" projectID="0/default0001" creationTime="0" modifiedBy="VitHeadlessServer">
  <TRANSPORT endToEnd="1" />
  <TEMPOSEQUENCE>
    <TEMPO startBeat="0.0" bpm="120.0" curve="1.0" />
    <TIMESIG numerator="4" denominator="4" startBeat="0.0" />
  </TEMPOSEQUENCE>
  <VIDEO />
  <CLICKTRACK level="0.6000000238418579" />
  <MASTERVOLUME>
    <PLUGIN type="volume" id="1001" enabled="1" volume="1.0">
      <MODIFIERASSIGNMENTS />
    </PLUGIN>
  </MASTERVOLUME>
  <RACKS />
  <MASTERPLUGINS />
  <INPUTDEVICES>
    <INPUTDEVICE deviceID="all_midi_in" name="All MIDI Ins" />
  </INPUTDEVICES>
  <TRACKCOMPS />
  <ARRANGERTRACK name="Arranger" id="1002" />
  <CHORDTRACK name="Chord" id="1003" />
  <MARKERTRACK id="1004" name="Marker" />
  <TEMPOTRACK name="Global" id="1005">
    <MODIFIERS />
  </TEMPOTRACK>
  <MASTERTRACK name="Master" id="1006">
    <MODIFIERS />
  </MASTERTRACK>
  <TRACK id="1007" name="Audio Track 1">
    <MODIFIERS />
    <OUTPUTDEVICES>
      <DEVICE name="(default audio output)" />
    </OUTPUTDEVICES>
    <CLIPSLOTS />
    <PLUGIN type="volume" id="1008" enabled="1" remapOnTempoChange="1">
      <MODIFIERASSIGNMENTS />
    </PLUGIN>
    <PLUGIN type="level" id="1009" enabled="1" />
  </TRACK>
  <SCENES />
</EDIT>
)VIT";
}

/** Minimal session: system / master / marker tracks only — no user audio tracks (for IPC new_project). */
inline juce::String getBlankProjectXmlTemplate()
{
    return R"VIT(<?xml version='1.0' encoding='utf-8'?>
<EDIT appVersion="Unknown" projectID="0/default0001" creationTime="0" modifiedBy="VitHeadlessServer">
  <TRANSPORT endToEnd="1" />
  <TEMPOSEQUENCE>
    <TEMPO startBeat="0.0" bpm="120.0" curve="1.0" />
    <TIMESIG numerator="4" denominator="4" startBeat="0.0" />
  </TEMPOSEQUENCE>
  <VIDEO />
  <CLICKTRACK level="0.6000000238418579" />
  <MASTERVOLUME>
    <PLUGIN type="volume" id="1001" enabled="1" volume="1.0">
      <MODIFIERASSIGNMENTS />
    </PLUGIN>
  </MASTERVOLUME>
  <RACKS />
  <MASTERPLUGINS />
  <INPUTDEVICES>
    <INPUTDEVICE deviceID="all_midi_in" name="All MIDI Ins" />
  </INPUTDEVICES>
  <TRACKCOMPS />
  <ARRANGERTRACK name="Arranger" id="1002" />
  <CHORDTRACK name="Chord" id="1003" />
  <MARKERTRACK id="1004" name="Marker" />
  <TEMPOTRACK name="Global" id="1005">
    <MODIFIERS />
  </TEMPOTRACK>
  <MASTERTRACK name="Master" id="1006">
    <MODIFIERS />
  </MASTERTRACK>
  <SCENES />
</EDIT>
)VIT";
}

inline bool looksLikeVitAppRoot (const juce::File& directory)
{
    return directory.isDirectory()
        && directory.getChildFile ("CMakeLists.txt").existsAsFile()
        && directory.getChildFile ("Source").isDirectory();
}

inline juce::File climbToVitAppRoot (juce::File directory)
{
    while (directory.exists())
    {
        if (looksLikeVitAppRoot (directory))
            return directory;

        const auto parent = directory.getParentDirectory();
        if (parent == directory)
            break;

        directory = parent;
    }

    return {};
}

} // namespace detail

inline juce::File getWorkspaceDirectory();

inline juce::File getProjectRootDirectory()
{
    if (auto root = detail::climbToVitAppRoot (juce::File::getCurrentWorkingDirectory());
        root.exists())
    {
        return root;
    }

    const auto executableDirectory = juce::File::getSpecialLocation (juce::File::currentExecutableFile)
                                        .getParentDirectory();

    if (auto root = detail::climbToVitAppRoot (executableDirectory);
        root.exists())
    {
        return root;
    }

    return juce::File::getCurrentWorkingDirectory();
}

inline juce::File getLogsDirectory()
{
    return getWorkspaceDirectory().getChildFile ("Logs");
}

inline juce::File getWorkspaceDirectory()
{
    return getProjectRootDirectory().getChildFile ("Workspace");
}

inline juce::File getSettingsDirectory()
{
    return getWorkspaceDirectory().getChildFile ("Settings");
}

inline juce::File getCacheDirectory()
{
    return getWorkspaceDirectory().getChildFile ("Cache");
}

inline bool ensureDirectoryExists (const juce::File& directory, const juce::String& label)
{
    if (directory.isDirectory())
        return true;

    if (directory.createDirectory())
    {
        juce::Logger::writeToLog ("VitPaths: created " + label + " directory: " + directory.getFullPathName());
        return true;
    }

    juce::Logger::writeToLog ("VitPaths: failed to create " + label + " directory: " + directory.getFullPathName());
    return false;
}

inline juce::File getDefaultProjectXmlFile()
{
    if (const auto overridePath = juce::SystemStats::getEnvironmentVariable ("VIT_PROJECT_XML", {});
        overridePath.isNotEmpty())
    {
        return juce::File (overridePath);
    }

    if (const auto legacyOverridePath = juce::SystemStats::getEnvironmentVariable ("VIT_GENERATED_PROJECT_XML", {});
        legacyOverridePath.isNotEmpty())
    {
        return juce::File (legacyOverridePath);
    }

    return getProjectRootDirectory().getChildFile (juce::String (VIT_DEFAULT_PROJECT_XML_RELATIVE));
}

inline juce::File ensureDefaultProjectXmlFileExists()
{
    const auto workspaceDirectory = getWorkspaceDirectory();
    ensureDirectoryExists (workspaceDirectory, "workspace");
    ensureDirectoryExists (getSettingsDirectory(), "settings");
    ensureDirectoryExists (getCacheDirectory(), "cache");

    const auto xmlFile = getDefaultProjectXmlFile();

    if (xmlFile.existsAsFile())
        return xmlFile;

    if (xmlFile.replaceWithText (detail::getDefaultProjectXmlTemplate()))
    {
        juce::Logger::writeToLog ("VitPaths: created default project XML: " + xmlFile.getFullPathName());
    }
    else
    {
        juce::Logger::writeToLog ("VitPaths: failed to create default project XML: " + xmlFile.getFullPathName());
    }

    return xmlFile;
}

inline juce::File getGeneratedProjectXmlFile()
{
    return getDefaultProjectXmlFile();
}

} // namespace vit::paths
