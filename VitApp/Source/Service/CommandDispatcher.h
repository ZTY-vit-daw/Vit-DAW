#pragma once

#include <functional>
#include <unordered_map>

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

class VitProductionCoordinator;

class CommandDispatcher final
{
public:
    using EditGetter = std::function<te::Edit*()>;
    using BoolAction = std::function<bool()>;
    using PublishAction = std::function<void(const juce::String&)>;

    using RecentProjectsReply = std::function<juce::String()>;
    using NewBlankProjectReply = std::function<juce::String()>;
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
                       CurrentProjectPathGetter currentProjectPathGetter,
                       VitProductionCoordinator* productionCoordinator = nullptr);

    juce::String dispatch (const juce::var& command, const juce::String& rawPayload) const;
    static juce::String makeStatusReply (const juce::String& status, const juce::String& message);
    static juce::String makeErrorReply (const juce::String& message);

private:
    using Handler = std::function<juce::String(const juce::DynamicObject&, const juce::String&)>;

    void registerBuiltinCommands();

    juce::String handlePing (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleReloadProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleListTracks (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetProjectState (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetTempo (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAppendGhostTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAddTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeleteTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAddAudioClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportAudio (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleMoveClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleCloneClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleResizeClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAddMidiNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAddMidiNotesBulk (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleMutateMidiNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeleteMidiNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetMidiClipNotes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetMidiClipData (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleInsertMidiClip (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRemoveClips (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleImportMediaToTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleWarmWaveformBake (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleAigcRegisterJob (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleBridgeIngestGeneratedAsset (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSwitchAssetTake (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetAsyncGhostState (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetVolume (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetMute (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetPluginParam (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetPluginParamAliases (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlAddNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlAddMacro (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlAddBinding (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlUpdateNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlSetNodeValue (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlUpdateBinding (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlRemoveBinding (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlRemoveNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleControlSetMacroValues (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleConnectorUpsertProfile (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleConnectorRemoveProfile (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleProjectHealthCheck (const juce::DynamicObject&, const juce::String&) const;
    juce::String handlePlay (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStop (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleReturnToZero (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleTransportOptionStopReturnToStart (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleToggleClick (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetClick (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSeek (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleClearProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleUndo (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRedo (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSaveProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetRecentProjects (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleOpenProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleNewProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSaveAsProject (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetAudioDeviceTypes (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetAudioDevices (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleSetAudioDevice (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetWaveInputDevices (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRouteWaveInputToTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleScanPlugins (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleInstantiatePlugin (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleOpenPluginUI (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleGetPluginParameters (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleDeletePlugin (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleMovePlugin (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackAddNode (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackConnectPins (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackRemoveConnection (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleRackSetNodeClipScope (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleArmTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStartRecording (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStopRecording (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleFreezeTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleUnfreezeTrack (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleStartRender (const juce::DynamicObject&, const juce::String&) const;
    juce::String handleCancelRender (const juce::DynamicObject&, const juce::String&) const;

    EditGetter getEdit;
    BoolAction reloadProject;
    BoolAction saveProject;
    PublishAction publishMessage;
    RecentProjectsReply recentProjectsReply;
    NewBlankProjectReply newBlankProjectReply;
    OpenProjectReply openProjectReply;
    SaveProjectReply saveProjectReply;
    SaveAsProjectReply saveAsProjectReply;
    CurrentProjectPathGetter getCurrentProjectPath;
    VitProductionCoordinator* production = nullptr;
    /** When true, handleStop seeks the edit timeline to 0s after stopping (UI stop-return-to-start). */
    mutable bool stopReturnsToZero = false;
    std::unordered_map<std::string, Handler> handlers;
};

} // namespace vit
