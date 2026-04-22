#include "VitEngineDevice.h"

#include "VitPluginUIBehaviour.h"
#include "VitPortablePropertyStorage.h"

namespace vit
{

void prepareTracktionAudioHardwareForPlayback (te::Engine& engine)
{
    auto& dm = engine.getDeviceManager();

    dm.dispatchPendingUpdates();
    dm.checkDefaultDevicesAreValid();

    juce::AudioIODevice* ad = dm.deviceManager.getCurrentAudioDevice();

    const auto isIoStable = [] (juce::AudioIODevice* d) -> bool
    {
        return d != nullptr
               && d->getCurrentBufferSizeSamples() > 0
               && d->getCurrentSampleRate() > 0.0;
    };

    if (! isIoStable (ad))
    {
        juce::Logger::writeToLog ("VitEngineDevice: audio IO not stable after device flush; rescanning wave device list.");
        dm.rescanWaveDeviceList();
        dm.dispatchPendingUpdates();
        dm.checkDefaultDevicesAreValid();
        ad = dm.deviceManager.getCurrentAudioDevice();
    }

    if (! isIoStable (ad))
    {
        juce::Logger::writeToLog ("VitEngineDevice: WARNING: JUCE AudioIODevice not ready (no device or zero block/rate); "
                                  "EditPlaybackContext may build an empty graph until hardware becomes available.");
        return;
    }

    juce::Logger::writeToLog ("VitEngineDevice: audio IO ready for playback graph: \"" + ad->getName() + "\" "
                              + juce::String (ad->getCurrentSampleRate(), 2) + " Hz, block "
                              + juce::String (ad->getCurrentBufferSizeSamples()) + " samples");
}

VitEngineDevice::VitEngineDevice (juce::String applicationName)
    : engine (std::make_unique<VitPortablePropertyStorage> (std::move (applicationName)),
              std::make_unique<VitPluginUIBehaviour>(),
              nullptr)
{
    initialiseDevices();
}

VitEngineDevice::~VitEngineDevice()
{
    juce::Logger::writeToLog ("VitEngineDevice: closing audio and MIDI devices.");
    engine.getDeviceManager().closeDevices();
}

void VitEngineDevice::initialiseDevices()
{
    juce::Logger::writeToLog ("VitEngineDevice: initialising Tracktion Engine device manager.");
    engine.getPluginManager().setGUIsLockedByDefault (false);
    auto& dm = engine.getDeviceManager();
    dm.initialise();
    prepareTracktionAudioHardwareForPlayback (engine);

    auto& audioDeviceManager = dm.deviceManager;
    const auto setup = audioDeviceManager.getAudioDeviceSetup();
    auto* currentDevice = audioDeviceManager.getCurrentAudioDevice();

    juce::Logger::writeToLog ("VitEngineDevice: audio setup outputDevice=\""
                              + setup.outputDeviceName
                              + "\" inputDevice=\""
                              + setup.inputDeviceName
                              + "\" sampleRate="
                              + juce::String (setup.sampleRate, 2)
                              + " bufferSize="
                              + juce::String (setup.bufferSize)
                              + " inputChannels="
                              + setup.inputChannels.toString (2)
                              + " outputChannels="
                              + setup.outputChannels.toString (2));

    if (currentDevice != nullptr)
    {
        juce::Logger::writeToLog ("VitEngineDevice: current audio device name=\""
                                  + currentDevice->getName()
                                  + "\" type=\""
                                  + currentDevice->getTypeName()
                                  + "\" activeOutputChannels="
                                  + currentDevice->getOutputChannelNames().joinIntoString (", ")
                                  + "\" activeInputChannels="
                                  + currentDevice->getInputChannelNames().joinIntoString (", ")
                                  + "\"");
    }
    else
    {
        juce::Logger::writeToLog ("VitEngineDevice: no current audio device is open.");
    }
}

} // namespace vit
