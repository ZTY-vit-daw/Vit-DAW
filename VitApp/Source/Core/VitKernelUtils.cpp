#include "VitKernelUtils.h"

namespace vit
{

using namespace tracktion::engine;

namespace
{

void annotateGhostTracks (juce::ValueTree state, VitSemanticBridge* semanticBridge)
{
    if (! state.isValid())
        return;

    if (state.getProperty ("vit_type").toString().equalsIgnoreCase ("ghost"))
    {
        auto trackName = state.getProperty ("name").toString();

        if (! trackName.endsWithIgnoreCase (" [GHOST]"))
            trackName << " [GHOST]";

        state.setProperty ("name", trackName, nullptr);

        const auto intent = state.getProperty ("vit_intent").toString();
        juce::Logger::writeToLog ("VitKernelUtils: detected ghost track intent=\"" + intent
                                  + "\" name=\"" + trackName + "\"");

        if (semanticBridge != nullptr)
            semanticBridge->onGhostTrackDetected ({ trackName, intent });
    }

    for (int i = 0; i < state.getNumChildren(); ++i)
        annotateGhostTracks (state.getChild (i), semanticBridge);
}

bool isMonitoringPlugin (const te::Plugin* plugin)
{
    return dynamic_cast<const te::VolumeAndPanPlugin*> (plugin) != nullptr
        || dynamic_cast<const te::LevelMeterPlugin*> (plugin) != nullptr;
}

int getRackInsertionIndex (te::Track& track)
{
    int index = 0;

    for (auto* plugin : track.pluginList.getPlugins())
    {
        if (plugin == nullptr)
            continue;

        if (isMonitoringPlugin (plugin))
            return index;

        ++index;
    }

    return -1;
}

bool rackHasInternalNodes (const te::RackInstance& rack)
{
    return rack.type != nullptr && ! rack.type->getPlugins().isEmpty();
}

} // namespace

std::unique_ptr<Edit> loadEditFromXmlString (Engine& engine,
                                             const juce::String& xmlText,
                                             uint32_t numAudioTracks,
                                             VitSemanticBridge* semanticBridge,
                                             const juce::File& pathResolutionFile)
{
    if (xmlText.isEmpty())
    {
        juce::Logger::writeToLog ("VitKernelUtils: XML string is empty.");
        return nullptr;
    }

    juce::ValueTree state;

    if (auto xml = juce::parseXML (xmlText))
    {
        updateLegacyEdit (*xml);
        state = juce::ValueTree::fromXml (*xml);
    }
    else
    {
        juce::Logger::writeToLog ("VitKernelUtils: XML parse failed.");
        return nullptr;
    }

    if (! state.isValid())
    {
        juce::Logger::writeToLog ("VitKernelUtils: ValueTree is invalid after XML conversion.");
        return nullptr;
    }

    if (! state.hasType (IDs::EDIT))
    {
        juce::Logger::writeToLog ("VitKernelUtils: root node is not EDIT.");
        return nullptr;
    }

    state = updateLegacyEdit (state);
    annotateGhostTracks (state, semanticBridge);

    auto id = ProjectItemID::fromProperty (state, IDs::projectID);
    if (! id.isValid())
        id = ProjectItemID::createNewID (0);

    if (! state.getProperty (IDs::appVersion).toString().isNotEmpty())
        state.setProperty (IDs::appVersion, engine.getPropertyStorage().getApplicationVersion(), nullptr);

    state.setProperty (IDs::projectID, id.toString(), nullptr);

    Edit::EditFileRetriever pathRetriever;

    if (pathResolutionFile.existsAsFile())
    {
        const juce::File pathRoot = pathResolutionFile;
        pathRetriever = [pathRoot]
        {
            return pathRoot;
        };
    }

    Edit::Options options
    {
        engine,
        state,
        id,
        Edit::forEditing,
        nullptr,
        Edit::getDefaultNumUndoLevels(),
        std::move (pathRetriever),
        {},
        numAudioTracks,
        -3.0f
    };

    return Edit::createEdit (std::move (options));
}

std::unique_ptr<Edit> loadEditFromXmlFile (Engine& engine,
                                           const juce::File& xmlFile,
                                           uint32_t numAudioTracks,
                                           VitSemanticBridge* semanticBridge)
{
    if (! xmlFile.existsAsFile())
    {
        juce::Logger::writeToLog ("VitKernelUtils: XML file not found: " + xmlFile.getFullPathName());
        return nullptr;
    }

    const auto xmlText = xmlFile.loadFileAsString();
    auto edit = loadEditFromXmlString (engine, xmlText, numAudioTracks, semanticBridge, xmlFile);

    if (edit != nullptr && semanticBridge != nullptr)
        semanticBridge->onEditReloaded (xmlFile);

    return edit;
}

void rebindAllWaveClipSourcesToDirectFiles (te::Edit& edit)
{
    using namespace tracktion::engine;

    for (auto* track : getAllTracks (edit))
    {
        if (track == nullptr)
            continue;

        const int n = track->getNumTrackItems();

        for (int i = 0; i < n; ++i)
        {
            auto* item = track->getTrackItem (i);
            auto* wave = dynamic_cast<WaveAudioClip*> (item);

            if (wave == nullptr)
                continue;

            juce::File bindFile;
            const juce::File original = wave->getOriginalFile();
            const juce::File current = wave->getCurrentSourceFile();

            if (original.existsAsFile() && AudioFile (edit.engine, original).isValid())
                bindFile = original;
            else if (current.existsAsFile() && AudioFile (edit.engine, current).isValid())
                bindFile = current;
            else
            {
                const juce::String srcProp = wave->state.getProperty (IDs::source).toString();
                juce::File absAttempt;

                if (edit.filePathResolver != nullptr)
                    absAttempt = edit.filePathResolver (srcProp);

                if (! absAttempt.existsAsFile() && juce::File::isAbsolutePath (srcProp))
                    absAttempt = juce::File (srcProp);

                if (absAttempt.existsAsFile() && AudioFile (edit.engine, absAttempt).isValid())
                    bindFile = absAttempt;
            }

            if (! bindFile.existsAsFile())
                continue;

            wave->getSourceFileReference().setToDirectFileReference (bindFile, false);
            wave->sourceMediaChanged();
            juce::Logger::writeToLog ("VitKernelUtils: wave clip " + wave->itemID.toString()
                                      + " forced direct file rebind: " + bindFile.getFullPathName());
        }
    }
}

bool ensureSingleRackForTrack (te::AudioTrack& track)
{
    bool changed = false;
    te::RackInstance* primaryRack = nullptr;

    const auto plugins = track.pluginList.getPlugins();
    for (auto* plugin : plugins)
    {
        auto* rack = dynamic_cast<te::RackInstance*> (plugin);
        if (rack == nullptr)
            continue;

        if (rack->type == nullptr)
        {
            rack->removeFromParent();
            changed = true;
            juce::Logger::writeToLog ("VitKernelUtils: removed broken rack instance without rack type on track "
                                      + track.itemID.toString());
            continue;
        }

        if (! rackHasInternalNodes (*rack))
        {
            rack->removeFromParent();
            changed = true;
            juce::Logger::writeToLog ("VitKernelUtils: removed empty rack instance on track "
                                      + track.itemID.toString());
            continue;
        }

        if (primaryRack == nullptr)
        {
            primaryRack = rack;
            continue;
        }

        juce::Logger::writeToLog ("VitKernelUtils: track " + track.itemID.toString()
                                  + " contains multiple non-empty rack instances; leaving migration for a later phase");
    }

    if (changed)
        track.flushStateToValueTree();

    return changed;
}

bool ensureTrackRackGraphForEdit (te::Edit& edit)
{
    bool changed = false;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack != nullptr)
            changed = ensureSingleRackForTrack (*audioTrack) || changed;
    }

    if (changed)
    {
        edit.dispatchPendingUpdatesSynchronously();
        edit.getTransport().ensureContextAllocated (true);
    }

    return changed;
}

} // namespace vit
