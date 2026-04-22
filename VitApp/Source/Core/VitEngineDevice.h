#pragma once

#include <JuceHeader.h>
#include <tracktion_engine/tracktion_engine.h>

namespace vit
{

namespace te = tracktion;

/**
 * 在分配 EditPlaybackContext / 构建播放图之前调用。
 * Tracktion 在 DeviceManager::initialise() 里通过 rescanWaveDeviceList() 异步重建 Wave 设备表；
 * 必须先 dispatchPendingUpdates()，否则 waveOutputs / 默认输出可能仍为空，导致首帧建图为空。
 */
void prepareTracktionAudioHardwareForPlayback (te::Engine& engine);

class VitEngineDevice final
{
public:
    explicit VitEngineDevice (juce::String applicationName);
    ~VitEngineDevice();

    te::Engine& getEngine() noexcept { return engine; }
    const te::Engine& getEngine() const noexcept { return engine; }

private:
    void initialiseDevices();

    te::Engine engine;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitEngineDevice)
};

} // namespace vit
