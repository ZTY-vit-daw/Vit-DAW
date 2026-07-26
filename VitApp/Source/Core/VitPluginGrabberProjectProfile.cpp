#include "VitPluginGrabberProjectProfile.h"

#include "VitMediaPoolManager.h"
#include "VitPluginGrabberGlobalProfile.h"
#include "VitPluginGrabberProfileFormat.h"

#include <unordered_set>

namespace vit
{

namespace
{

constexpr int kSchemaVersion = 1;

juce::File getProfilesFile (const juce::File& projectFile)
{
    const auto folders = VitMediaPoolManager::resolveProjectFolders (projectFile);
    return folders.mediaRoot.getChildFile ("plugin_grabber_profiles.json");
}

juce::String cleanText (const juce::var& value)
{
    return value.toString().trim();
}

juce::StringArray toStringArray (const juce::var& value)
{
    juce::StringArray out;
    if (auto* arr = value.getArray())
    {
        for (const auto& item : *arr)
        {
            const auto text = cleanText (item);
            if (text.isNotEmpty())
                out.addIfNotAlreadyThere (text);
        }
    }
    return out;
}

juce::Array<juce::var> toVarArray (const juce::StringArray& values)
{
    juce::Array<juce::var> out;
    for (const auto& value : values)
        out.add (value);
    return out;
}

juce::var sanitisedStringMap (const juce::var& value)
{
    auto out = std::make_unique<juce::DynamicObject>();
    if (auto* object = value.getDynamicObject())
    {
        const auto& properties = object->getProperties();
        for (int i = 0; i < properties.size(); ++i)
        {
            const auto key = properties.getName (i).toString().trim();
            const auto mapped = properties.getValueAt (i).toString().trim();
            if (key.isNotEmpty() && mapped.isNotEmpty())
                out->setProperty (juce::Identifier (key), mapped);
        }
    }
    return juce::var (out.release());
}

juce::var emptyObject()
{
    return juce::var (new juce::DynamicObject());
}

juce::DynamicObject* objectOrNull (const juce::var& value)
{
    return value.getDynamicObject();
}

juce::String propertyString (const juce::var& objectVar, const juce::String& propertyName)
{
    if (auto* object = objectOrNull (objectVar))
        return object->getProperty (propertyName).toString().trim();
    return {};
}

juce::var getObjectPropertyOrEmpty (const juce::var& objectVar, const juce::String& propertyName)
{
    if (auto* object = objectOrNull (objectVar))
    {
        if (auto* child = object->getProperty (propertyName).getDynamicObject())
        {
            auto out = std::make_unique<juce::DynamicObject>();
            const auto& properties = child->getProperties();
            for (int i = 0; i < properties.size(); ++i)
                out->setProperty (properties.getName (i), properties.getValueAt (i));
            return juce::var (out.release());
        }
    }
    return emptyObject();
}

void copyProfileProperty (juce::DynamicObject& profile,
                          const juce::DynamicObject& incoming,
                          const juce::var& existing,
                          const juce::Identifier& propertyName)
{
    if (incoming.hasProperty (propertyName))
    {
        profile.setProperty (propertyName, incoming.getProperty (propertyName));
        return;
    }

    if (auto* existingObject = existing.getDynamicObject())
        if (existingObject->hasProperty (propertyName))
            profile.setProperty (propertyName, existingObject->getProperty (propertyName));
}

juce::String mapLookup (const juce::var& mapVar, const juce::String& key)
{
    if (auto* object = mapVar.getDynamicObject())
        return object->getProperty (juce::Identifier (key)).toString().trim();
    return {};
}

void collectMapStaleIds (const juce::var& mapVar,
                         const std::unordered_set<std::string>& validParamIds,
                         juce::StringArray& staleParamIds)
{
    if (auto* object = mapVar.getDynamicObject())
    {
        const auto& properties = object->getProperties();
        for (int i = 0; i < properties.size(); ++i)
        {
            const auto key = properties.getName (i).toString().trim();
            if (key.isNotEmpty() && ! validParamIds.contains (key.toStdString()))
                staleParamIds.addIfNotAlreadyThere (key);
        }
    }
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
               : juce::Result::fail ("Failed to write plugin grabber profile store");
}

juce::var findProfileById (const juce::Array<juce::var>& profiles, const juce::String& profileId)
{
    for (const auto& profile : profiles)
        if (auto* object = profile.getDynamicObject())
            if (object->getProperty ("profile_id").toString() == profileId)
                return profile;
    return {};
}

juce::String canonicalPluginPath (const te::ExternalPlugin& plugin)
{
    const auto raw = plugin.desc.fileOrIdentifier.trim();
    if (raw.isEmpty())
        return {};

    juce::File file (raw);
    return file.exists() ? file.getFullPathName() : raw;
}

juce::String makeProfileKey (const te::ExternalPlugin& plugin)
{
    const auto path = canonicalPluginPath (plugin).toLowerCase();
    const auto seed = plugin.desc.pluginFormatName.trim().toLowerCase()
                    + "|" + plugin.desc.manufacturerName.trim().toLowerCase()
                    + "|" + plugin.getName().trim().toLowerCase()
                    + "|" + path;
    return "plugin_" + juce::String::toHexString (static_cast<juce::int64> (seed.hashCode64()));
}

juce::String widgetForRole (const juce::DynamicObject& object)
{
    const auto normalizedRole = object.getProperty ("normalized_role").toString();
    if (static_cast<bool> (object.getProperty ("is_boolean"))
        || normalizedRole == "common_bypass"
        || normalizedRole.contains ("enable"))
        return "toggle";

    if (normalizedRole.contains ("freq") || normalizedRole.contains ("cutoff") || normalizedRole.contains ("resonance")
        || normalizedRole.contains ("pan") || normalizedRole.contains ("ratio"))
        return "knob";

    return "slider";
}

juce::String labelForParameter (const juce::DynamicObject& object)
{
    auto label = object.getProperty ("alias").toString().trim();
    if (label.isNotEmpty())
        return label;
    label = object.getProperty ("name").toString().trim();
    if (label.isNotEmpty())
        return label;
    return object.getProperty ("id").toString().trim();
}

juce::var makeQuickControl (const juce::DynamicObject& object)
{
    auto control = std::make_unique<juce::DynamicObject>();
    control->setProperty ("param_id", object.getProperty ("id"));
    control->setProperty ("raw_param_id", object.getProperty ("raw_param_id"));
    control->setProperty ("label", labelForParameter (object));
    control->setProperty ("widget", widgetForRole (object));
    control->setProperty ("normalized_role", object.getProperty ("normalized_role"));
    control->setProperty ("display_group", object.getProperty ("display_group"));
    control->setProperty ("host_controllable", object.getProperty ("host_controllable"));
    control->setProperty ("control_relevance", object.getProperty ("control_relevance"));
    control->setProperty ("control_priority", object.getProperty ("control_priority"));
    control->setProperty ("value", object.getProperty ("value"));
    return juce::var (control.release());
}

void mergeExtendedProfileFields (const juce::DynamicObject& profile,
                                 VitPluginGrabberProjectProfile::MergeResult& result,
                                 bool overwriteExisting)
{
    const auto cls = profile.getProperty (profileField::pluginClass).toString().trim();
    if (cls.isNotEmpty() && (overwriteExisting || result.pluginClass.isEmpty()))
        result.pluginClass = cls;

    if (auto* groups = profile.getProperty (profileField::groups).getArray())
        if (groups->size() > 0 && (overwriteExisting || result.groups.size() == 0))
            result.groups = *groups;

    if (auto* controls = profile.getProperty (profileField::virtualControls).getArray())
        if (controls->size() > 0 && (overwriteExisting || result.virtualControls.size() == 0))
            result.virtualControls = *controls;

    if (auto* safety = profile.getProperty (profileField::safety).getDynamicObject())
        if (overwriteExisting || ! result.safety.isObject())
            result.safety = juce::var (safety->clone().release());
}

} // namespace

juce::var VitPluginGrabberProjectProfile::createPluginIdentity (const te::ExternalPlugin& plugin,
                                                                const juce::String& pluginId)
{
    auto identity = std::make_unique<juce::DynamicObject>();
    identity->setProperty ("profile_key", makeProfileKey (plugin));
    identity->setProperty ("plugin_id", pluginId);
    identity->setProperty ("plugin_name", plugin.getName());
    identity->setProperty ("plugin_format", plugin.desc.pluginFormatName);
    identity->setProperty ("plugin_path", canonicalPluginPath (plugin));
    identity->setProperty ("manufacturer", plugin.desc.manufacturerName);
    identity->setProperty ("version", plugin.desc.version);
    return juce::var (identity.release());
}

juce::String VitPluginGrabberProjectProfile::profileKeyForPlugin (const te::ExternalPlugin& plugin)
{
    return makeProfileKey (plugin);
}

juce::Array<juce::var> VitPluginGrabberProjectProfile::snapshotProfiles (const juce::File& projectFile)
{
    return readProfilesFile (projectFile);
}

juce::Result VitPluginGrabberProjectProfile::upsertProjectDefault (const juce::File& projectFile,
                                                                   const juce::DynamicObject& object,
                                                                   const juce::var& pluginIdentity,
                                                                   juce::var& outProfile)
{
    const auto profileId = propertyString (pluginIdentity, "profile_key");
    if (profileId.isEmpty())
        return juce::Result::fail ("plugin_grabber_upsert_project_profile could not resolve plugin profile key");

    auto profiles = readProfilesFile (projectFile);
    auto existing = findProfileById (profiles, profileId);
    auto projectDefault = getObjectPropertyOrEmpty (existing, "project_default");

    auto* projectDefaultObject = projectDefault.getDynamicObject();
    if (projectDefaultObject == nullptr)
        return juce::Result::fail ("plugin_grabber_upsert_project_profile failed to create project_default");

    if (object.hasProperty ("quick_control_ids"))
        projectDefaultObject->setProperty ("quick_control_ids", juce::var (toVarArray (toStringArray (object.getProperty ("quick_control_ids")))));
    if (object.hasProperty ("aliases"))
        projectDefaultObject->setProperty ("aliases", sanitisedStringMap (object.getProperty ("aliases")));
    if (object.hasProperty ("display_groups"))
        projectDefaultObject->setProperty ("display_groups", sanitisedStringMap (object.getProperty ("display_groups")));
    if (object.hasProperty ("normalized_roles"))
        projectDefaultObject->setProperty ("normalized_roles", sanitisedStringMap (object.getProperty ("normalized_roles")));

    if (! projectDefaultObject->hasProperty ("quick_control_ids"))
        projectDefaultObject->setProperty ("quick_control_ids", juce::var (juce::Array<juce::var>()));
    if (! projectDefaultObject->hasProperty ("aliases"))
        projectDefaultObject->setProperty ("aliases", emptyObject());
    if (! projectDefaultObject->hasProperty ("display_groups"))
        projectDefaultObject->setProperty ("display_groups", emptyObject());
    if (! projectDefaultObject->hasProperty ("normalized_roles"))
        projectDefaultObject->setProperty ("normalized_roles", emptyObject());

    auto profile = std::make_unique<juce::DynamicObject>();
    profile->setProperty ("schema_version", kSchemaVersion);
    profile->setProperty ("profile_id", profileId);
    profile->setProperty ("plugin_identity", pluginIdentity);
    profile->setProperty ("project_default", projectDefault);
    profile->setProperty ("instances", getObjectPropertyOrEmpty (existing, "instances"));
    copyProfileProperty (*profile, object, existing, "class");
    copyProfileProperty (*profile, object, existing, "groups");
    copyProfileProperty (*profile, object, existing, "virtual_controls");
    copyProfileProperty (*profile, object, existing, "safety");
    copyProfileProperty (*profile, object, existing, "param_signature_hash");
    copyProfileProperty (*profile, object, existing, "parameter_snapshot");
    copyProfileProperty (*profile, object, existing, "plugin_skill");
    copyProfileProperty (*profile, object, existing, "plugin_skill_validator_warnings");
    profile->setProperty ("updated_at", juce::Time::getCurrentTime().toISO8601 (true));
    const auto profileVar = juce::var (profile.release());

    bool replaced = false;
    for (int i = 0; i < profiles.size(); ++i)
    {
        if (auto* item = profiles.getReference (i).getDynamicObject())
        {
            if (item->getProperty ("profile_id").toString() == profileId)
            {
                profiles.set (i, profileVar);
                replaced = true;
                break;
            }
        }
    }

    if (! replaced)
        profiles.add (profileVar);

    if (const auto writeResult = writeProfilesFile (projectFile, profiles); writeResult.failed())
        return writeResult;

    outProfile = profileVar;
    return juce::Result::ok();
}

juce::Result VitPluginGrabberProjectProfile::removeProfile (const juce::File& projectFile,
                                                            const juce::String& profileId,
                                                            bool& removed)
{
    removed = false;
    auto profiles = readProfilesFile (projectFile);

    for (int i = profiles.size(); --i >= 0;)
    {
        if (auto* item = profiles.getReference (i).getDynamicObject())
        {
            if (item->getProperty ("profile_id").toString() == profileId)
            {
                profiles.remove (i);
                removed = true;
            }
        }
    }

    if (! removed)
        return juce::Result::ok();

    return writeProfilesFile (projectFile, profiles);
}

void VitPluginGrabberProjectProfile::populateGlobalInfo (const juce::var& pluginIdentity, MergeResult& result, const juce::String& liveSignature)
{
    const auto profileKey = propertyString (pluginIdentity, "profile_key");
    if (profileKey.isEmpty())
        return;

    const auto globalProfile = VitPluginGrabberGlobalProfile::findGlobalProfile (profileKey);
    auto* globalProfileObject = globalProfile.getDynamicObject();
    if (globalProfileObject == nullptr)
        return;

    // Same profile_key staleness hazard as the project profile: reject a global
    // profile whose recorded parameter signature no longer matches the live plugin.
    const auto storedSignature = paramSignature::loadFromProfile (globalProfile);
    if (storedSignature.isNotEmpty() && liveSignature.isNotEmpty() && storedSignature != liveSignature)
        return;

    result.globalProfileApplied = true;
    result.globalProfileSource = "global_profile";
    result.globalProfile = globalProfile;
    mergeExtendedProfileFields (*globalProfileObject, result, false);
}

VitPluginGrabberProjectProfile::MergeResult VitPluginGrabberProjectProfile::applyProjectDefault (
    const juce::File& projectFile,
    const juce::var& pluginIdentity,
    juce::Array<juce::var>& parameterDescriptors)
{
    MergeResult result;
    const auto profileId = propertyString (pluginIdentity, "profile_key");
    if (profileId.isEmpty())
        return result;

    // profile_key only hashes plugin format/manufacturer/name/path, so a
    // plugin binary update that renumbers ParamIDs (e.g. sequential -> native
    // VST3 IDs) keeps the same profile_key. Gate the alias/group merge on the
    // stored parameter signature so a stale profile cannot splice old param
    // IDs onto a live plugin with a different parameter surface.
    const auto liveSignature = paramSignature::computeHash (parameterDescriptors);

    const auto profile = findProfileById (readProfilesFile (projectFile), profileId);
    auto* profileObject = profile.getDynamicObject();
    // A stale-signature project profile must not block the independent
    // global-profile lookup below: only skip applying THIS profile's
    // alias/group/virtual-control fields, don't early-return the function.
    const auto storedSignature = profileObject != nullptr ? paramSignature::loadFromProfile (profile) : juce::String();
    const auto projectProfileIsStale = storedSignature.isNotEmpty() && storedSignature != liveSignature;
    if (projectProfileIsStale)
        result.profileSource = "stale_signature_mismatch";
    if (profileObject != nullptr && ! projectProfileIsStale)
    {
        result.profile = profile;
        mergeExtendedProfileFields (*profileObject, result, true);

        if (auto* defaults = profileObject->getProperty ("project_default").getDynamicObject())
        {
            result.profileApplied = true;
            result.profileSource = "project_profile";

            const auto aliases = defaults->getProperty ("aliases");
            const auto displayGroups = defaults->getProperty ("display_groups");
            const auto normalizedRoles = defaults->getProperty ("normalized_roles");
            result.quickControlIds = toStringArray (defaults->getProperty ("quick_control_ids"));

            std::unordered_set<std::string> validParamIds;
            for (auto& parameter : parameterDescriptors)
            {
                auto* object = parameter.getDynamicObject();
                if (object == nullptr)
                    continue;

                const auto paramId = object->getProperty ("id").toString().trim();
                if (paramId.isEmpty())
                    continue;

                validParamIds.insert (paramId.toStdString());

                if (const auto alias = mapLookup (aliases, paramId); alias.isNotEmpty())
                {
                    object->setProperty ("alias", alias);
                    object->setProperty ("alias_source", "project_profile");
                }
                if (const auto group = mapLookup (displayGroups, paramId); group.isNotEmpty())
                {
                    object->setProperty ("display_group", group);
                    object->setProperty ("display_group_source", "project_profile");
                }
                if (const auto role = mapLookup (normalizedRoles, paramId); role.isNotEmpty())
                {
                    object->setProperty ("normalized_role", role);
                    object->setProperty ("normalized_role_source", "project_profile");
                }
            }

            for (const auto& paramId : result.quickControlIds)
                if (! validParamIds.contains (paramId.toStdString()))
                    result.staleParamIds.addIfNotAlreadyThere (paramId);
            collectMapStaleIds (aliases, validParamIds, result.staleParamIds);
            collectMapStaleIds (displayGroups, validParamIds, result.staleParamIds);
            collectMapStaleIds (normalizedRoles, validParamIds, result.staleParamIds);
        }
    }

    populateGlobalInfo (pluginIdentity, result, liveSignature);

    return result;
}

juce::Array<juce::var> VitPluginGrabberProjectProfile::buildQuickControlsForIds (
    const juce::Array<juce::var>& parameterDescriptors,
    const juce::StringArray& quickControlIds)
{
    juce::Array<juce::var> controls;
    juce::StringArray seen;

    for (const auto& requestedId : quickControlIds)
    {
        if (requestedId.isEmpty() || seen.contains (requestedId))
            continue;

        for (const auto& parameter : parameterDescriptors)
        {
            auto* object = parameter.getDynamicObject();
            if (object == nullptr)
                continue;

            if (object->getProperty ("id").toString() != requestedId)
                continue;

            controls.add (makeQuickControl (*object));
            seen.add (requestedId);
            break;
        }
    }

    return controls;
}

} // namespace vit


