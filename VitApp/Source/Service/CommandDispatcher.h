#pragma once

#include <functional>
#include <memory>
#include <unordered_map>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

#include "VspKernelReference.h"

namespace vit
{

namespace te = tracktion;

class VitProductionCoordinator;
class ImportService;
class GeneratedAssetService;
class JobEventService;
class ProjectAudioSettingsService;
class ProjectMarkerService;
class ProjectService;
class TrackGroupService;
class TrackService;
class ClipService;
class MidiService;
class TransportAudioService;
class PluginRackControlService;
class AuditionPreviewService;

class CommandDispatcher final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using BoolAction = std::function<bool()>;
    using PublishAction = std::function<void(const juce::String&)>;
    using RealtimeDataProvider = VspKernelReference::RealtimeDataProvider;

    using RecentProjectsReply = std::function<juce::String()>;
    using NewBlankProjectReply = std::function<juce::String (const juce::DynamicObject&)>;
    using OpenProjectReply = std::function<juce::String (const juce::DynamicObject&, const juce::File&)>;
    using SaveProjectReply = std::function<juce::String (const juce::DynamicObject&)>;
    using SaveAsProjectReply = std::function<juce::String (const juce::DynamicObject&, const juce::File&)>;
    /** Absolute path of the active .vit session, or empty if unsaved / template. */
    using CurrentProjectPathGetter = std::function<juce::String()>;

    CommandDispatcher (EditGetter editGetter,
                       BoolAction reloadProjectAction,
                       BoolAction saveProjectAction,
                       PublishAction publishAction,
                       RecentProjectsReply recentProjectsReply,
                       NewBlankProjectReply newBlankProjectReply,
                       OpenProjectReply openProjectReply,
                       SaveProjectReply saveProjectReply,
                       SaveAsProjectReply saveAsProjectReply,
                       SaveAsProjectReply saveProjectCopyReply,
                       CurrentProjectPathGetter currentProjectPathGetter,
                       RealtimeDataProvider realtimeDataProvider = {},
                       VitProductionCoordinator* productionCoordinator = nullptr,
                       te::Engine* engine = nullptr);
    ~CommandDispatcher();

    juce::String dispatch (const juce::var& command, const juce::String& rawPayload) const;
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

private:
    using Handler = std::function<juce::String(const juce::DynamicObject&, const juce::String&)>;

    void registerBuiltinCommands();

    juce::String handlePing (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetProjectState (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetTempo (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleProjectHealthCheck (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleClearProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleUndo (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRedo (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleProjectSnapshotExport (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleProjectUndoState (const juce::DynamicObject&, const juce::String&) const;
    EditGetter getEdit;
    BoolAction saveProject;
    PublishAction publishMessage;
    CurrentProjectPathGetter getCurrentProjectPath;
    RealtimeDataProvider getRealtimeData;
    VitProductionCoordinator* production = nullptr;
    std::unique_ptr<ImportService> importService;
    std::unique_ptr<GeneratedAssetService> generatedAssetService;
    std::unique_ptr<JobEventService> jobEventService;
    std::unique_ptr<ProjectAudioSettingsService> projectAudioSettingsService;
    std::unique_ptr<ProjectMarkerService> projectMarkerService;
    std::unique_ptr<ProjectService> projectService;
    std::unique_ptr<TrackGroupService> trackGroupService;
    std::unique_ptr<TrackService> trackService;
    std::unique_ptr<ClipService> clipService;
    std::unique_ptr<MidiService> midiService;
    std::unique_ptr<TransportAudioService> transportAudioService;
    std::unique_ptr<PluginRackControlService> pluginRackControlService;
    std::unique_ptr<AuditionPreviewService> auditionPreviewService;
    std::unordered_map<std::string, Handler> handlers;
};

} // namespace vit
