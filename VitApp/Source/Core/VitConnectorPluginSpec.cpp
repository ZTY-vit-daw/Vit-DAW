#include "VitConnectorPluginSpec.h"

#include "VitMediaPoolManager.h"

namespace vit
{

namespace
{

juce::Array<juce::var> toVarArray (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return out;
}

juce::File getProfilesFile (const juce::File& projectFile)
{
    const auto folders = VitMediaPoolManager::resolveProjectFolders (projectFile);
    return folders.mediaRoot.getChildFile ("connector_profiles.json");
}

juce::var makeSpecVar (const juce::String& specId,
                       const juce::String& displayName,
                       const juce::String& outputKind,
                       const juce::StringArray& supportedOutputKinds)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("spec_id", specId);
    object->setProperty ("display_name", displayName);
    object->setProperty ("connector_kind", "browser_download");
    object->setProperty ("login_mode", "browser_self_login");
    object->setProperty ("ingest_mode", "download_watch");
    object->setProperty ("default_output_kind", outputKind);
    object->setProperty ("supported_output_kinds", juce::var (toVarArray (supportedOutputKinds)));
    return juce::var (object.release());
}

juce::Array<juce::var> readProfilesFile (const juce::File& projectFile)
{
    juce::Array<juce::var> profiles;
    const auto file = getProfilesFile (projectFile);

    if (! file.existsAsFile())
        return profiles;

    const auto parsed = juce::JSON::parse (file.loadFileAsString());
    if (parsed.isArray())
        profiles = *parsed.getArray();

    return profiles;
}

juce::Result writeProfilesFile (const juce::File& projectFile, const juce::Array<juce::var>& profiles)
{
    if (const auto ensureResult = VitMediaPoolManager::ensureProjectFolders (projectFile); ensureResult.failed())
        return ensureResult;

    const auto file = getProfilesFile (projectFile);
    return file.replaceWithText (juce::JSON::toString (juce::var (profiles), true))
               ? juce::Result::ok()
               : juce::Result::fail ("Failed to write connector profile store");
}

juce::String normaliseSpecId (const juce::String& raw)
{
    const auto specId = raw.trim();
    if (specId.isNotEmpty())
        return specId;

    return "browser_download_audio";
}

juce::var makeProfileVar (const juce::String& profileId,
                          const juce::String& displayName,
                          const juce::String& specId,
                          const juce::String& entryUrl,
                          const juce::String& outputKind,
                          const juce::String& downloadPattern,
                          bool enabled,
                          const juce::String& notes)
{
    auto object = std::make_unique<juce::DynamicObject>();
    object->setProperty ("profile_id", profileId);
    object->setProperty ("display_name", displayName);
    object->setProperty ("spec_id", normaliseSpecId (specId));
    object->setProperty ("entry_url", entryUrl);
    object->setProperty ("output_kind", outputKind.isNotEmpty() ? outputKind : "audio");
    object->setProperty ("download_pattern", downloadPattern);
    object->setProperty ("enabled", enabled);
    object->setProperty ("notes", notes);
    object->setProperty ("login_mode", "browser_self_login");
    object->setProperty ("ingest_mode", "download_watch");
    return juce::var (object.release());
}

} // namespace

juce::Array<juce::var> VitConnectorPluginSpec::createBuiltInSpecs()
{
    juce::Array<juce::var> specs;
    specs.add (makeSpecVar ("browser_download_audio", "Browser Audio Connector", "audio", { "audio", "lyrics" }));
    specs.add (makeSpecVar ("browser_download_stems", "Browser Stems Connector", "stems", { "stems", "audio" }));
    specs.add (makeSpecVar ("browser_download_midi", "Browser MIDI Connector", "midi", { "midi", "audio" }));
    return specs;
}

juce::Array<juce::var> VitConnectorPluginSpec::snapshotProfiles (const juce::File& projectFile)
{
    return readProfilesFile (projectFile);
}

juce::Result VitConnectorPluginSpec::upsertProfile (const juce::File& projectFile,
                                                    const juce::DynamicObject& object,
                                                    juce::var& outProfile)
{
    auto profileId = object.getProperty ("profile_id").toString().trim();
    if (profileId.isEmpty())
        profileId = "connector_" + juce::Uuid().toString();

    const auto displayName = object.getProperty ("display_name").toString().trim().isNotEmpty()
                                 ? object.getProperty ("display_name").toString().trim()
                                 : object.getProperty ("name").toString().trim();
    const auto specId = normaliseSpecId (object.getProperty ("spec_id").toString());
    const auto entryUrl = object.getProperty ("entry_url").toString().trim();
    const auto outputKind = object.getProperty ("output_kind").toString().trim();
    const auto downloadPattern = object.getProperty ("download_pattern").toString().trim();
    const auto notes = object.getProperty ("notes").toString();
    const auto enabledVar = object.getProperty ("enabled");
    const auto enabled = enabledVar.isBool() ? static_cast<bool> (enabledVar) : true;

    if (displayName.isEmpty())
        return juce::Result::fail ("connector_upsert_profile requires display_name");

    auto profiles = readProfilesFile (projectFile);
    const auto profileVar = makeProfileVar (profileId,
                                            displayName,
                                            specId,
                                            entryUrl,
                                            outputKind,
                                            downloadPattern,
                                            enabled,
                                            notes);

    bool replaced = false;
    for (int i = 0; i < profiles.size(); ++i)
    {
        auto* item = profiles.getReference (i).getDynamicObject();
        if (item == nullptr)
            continue;

        if (item->getProperty ("profile_id").toString() == profileId)
        {
            profiles.set (i, profileVar);
            replaced = true;
            break;
        }
    }

    if (! replaced)
        profiles.add (profileVar);

    if (const auto writeResult = writeProfilesFile (projectFile, profiles); writeResult.failed())
        return writeResult;

    outProfile = profileVar;
    return juce::Result::ok();
}

juce::Result VitConnectorPluginSpec::removeProfile (const juce::File& projectFile,
                                                    const juce::String& profileId,
                                                    bool& removed)
{
    removed = false;
    auto profiles = readProfilesFile (projectFile);

    for (int i = profiles.size(); --i >= 0;)
    {
        auto* item = profiles.getReference (i).getDynamicObject();
        if (item == nullptr)
            continue;

        if (item->getProperty ("profile_id").toString() == profileId)
        {
            profiles.remove (i);
            removed = true;
        }
    }

    if (! removed)
        return juce::Result::ok();

    return writeProfilesFile (projectFile, profiles);
}

} // namespace vit
