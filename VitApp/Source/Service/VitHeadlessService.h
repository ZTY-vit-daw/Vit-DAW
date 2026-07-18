#pragma once

#include <JuceHeader.h>
#include <atomic>
#include <memory>
#include <unordered_map>
#include <tracktion_engine/tracktion_engine.h>

#include "../Bridge/VitSemanticBridge.h"
#include "../Core/VitDeltaProbe.h"
#include "../Core/VitEngineDevice.h"
#include "../Core/VitGlobalProjectConfig.h"
#include "CommandDispatcher.h"
#include "VitProductionCoordinator.h"
#include "ZmqGateway.h"

namespace vit
{

namespace te = tracktion;

class VitHeadlessService final : private VitSemanticBridge,
                                 private juce::Timer
{
public:
    explicit VitHeadlessService (juce::String applicationName);

    bool start();
    void stop();

    te::Engine& getEngine() noexcept { return engineDevice.getEngine(); }
    const te::Edit* getEdit() const noexcept { return edit.get(); }

    /** 供 ZMQ 等后台模块消费 ValueTree 增量队列（非 const：tryPop 会修改队列）。 */
    VitDeltaRingBuffer& getDeltaRingBuffer() noexcept { return deltaHub.getBuffer(); }

private:
    struct TrackLevelClient
    {
        te::LevelMeterPlugin* plugin = nullptr;
        std::unique_ptr<te::LevelMeasurer::Client> client;
        std::unique_ptr<te::LevelMeasurer::Client> vspClient;
    };

    static constexpr int telemetryIntervalMs = 50;

    bool reloadProjectFromDefaultXml();
    bool saveProjectToDefaultXml();
    bool saveProjectToCurrentPathOrDefaultXml();
    bool loadProjectFromFile (const juce::File& projectFile);
    bool loadEncryptedProjectFromFile (const juce::File& projectFile);
    bool applyLoadedEdit (std::unique_ptr<te::Edit> loadedEdit, const juce::File& openedProject);
    juce::String ipcGetRecentProjects();
    juce::String ipcNewBlankProject (const juce::DynamicObject& object);
    juce::String ipcOpenProjectAt (const juce::DynamicObject& object, const juce::File& projectFile);
    juce::String ipcSaveProjectWithPayload (const juce::DynamicObject& object);
    juce::String ipcSaveEncryptedProjectToPath (const juce::File& fileFromPayload,
                                                bool createNewProjectIdentity,
                                                const juce::DynamicObject& object);
    juce::String ipcSaveProjectToCurrentPath (const juce::DynamicObject* object = nullptr);
    juce::String ipcSaveAsProjectAt (const juce::DynamicObject& object, const juce::File& targetFile);
    void clearLevelMeterClients();
    void syncLevelMeterClients();
    void timerCallback() override;
    void broadcastTelemetry();
    void broadcastTransportTelemetry();
    void broadcastLevelsTelemetry();
    juce::var buildVspRealtimeDataForStream (const juce::DynamicObject& stream);
    juce::String handleIncomingCommandOnMessageThread (const juce::var& command, const juce::String& payload);
    void onEditReloaded (const juce::File& xmlFile) override;
    void onGhostTrackDetected (const GhostTrackDescriptor& descriptor) override;

    VitEngineDevice engineDevice;
    std::unique_ptr<te::Edit> edit;
    std::unique_ptr<VitGlobalProjectConfig> globalProjectConfig;
    juce::File currentProjectPath;
    std::unique_ptr<VitProductionCoordinator> productionCoordinator;
    std::unique_ptr<CommandDispatcher> commandDispatcher;
    std::unique_ptr<ZmqGateway> zmqGateway;
    std::unordered_map<std::string, TrackLevelClient> trackLevelClients;
    std::shared_ptr<std::atomic<bool>> callbackGuard { std::make_shared<std::atomic<bool>> (true) };
    VitEditStateDeltaHub deltaHub;
    bool lastTransportRecording = false;
    /** transport 50ms；录中波形每 N tick 附帶一帧以降低 UDP/JSON 负载（仍保持走带每 tick）。 */
    int transportTelemetryTick = 0;
    static constexpr int recordingWaveformTelemetryStride = 2;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitHeadlessService)
};

} // namespace vit
