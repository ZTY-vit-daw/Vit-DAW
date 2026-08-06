#include <juce_audio_formats/juce_audio_formats.h>
#include <juce_audio_processors/juce_audio_processors.h>
#include <juce_audio_utils/juce_audio_utils.h>
#include <juce_gui_extra/juce_gui_extra.h>

#include <atomic>
#include <cmath>
#include <memory>

namespace
{
constexpr auto defaultSampleRate = 48000.0;
constexpr auto defaultBlockSize = 512;
constexpr auto defaultInputGainDB = -18.0;

juce::File defaultProbeAudioFile()
{
    const auto executable = juce::File::getSpecialLocation (juce::File::currentExecutableFile);
    const auto root = executable.getParentDirectory();
    const auto bundled = root.getChildFile ("assets")
                             .getChildFile ("probe_audio_v1")
                             .getChildFile ("multitone_31band_48k_10s.wav");
    return bundled.existsAsFile() ? bundled : juce::File{};
}

struct LaunchOptions
{
    juce::File pluginFile;
    juce::File audioFile;
    double sampleRate = defaultSampleRate;
    int blockSize = defaultBlockSize;
    double inputGainDB = defaultInputGainDB;
    bool loopSource = true;
    juce::String error;
};

class QuitAfterDialog final : public juce::ModalComponentManager::Callback
{
public:
    void modalStateFinished (int) override
    {
        juce::JUCEApplicationBase::quit();
    }
};

// This is a deliberately small live-monitor chain:
//
//   user-selected local WAV -> input gain -> loaded VST3 -> Windows output device
//
// It lives entirely inside the already-isolated witness process. It has no
// network listener and has no profile, mapping, or Vit project authority. The
// host bypass below is a direct source-to-output A/B switch;
// it never writes the plug-in's own bypass parameter.
class LiveMonitor final : private juce::AudioSource
{
public:
    explicit LiveMonitor (juce::AudioProcessor& processor)
        : plugin (processor), readAheadThread ("Plugin Probe witness source reader")
    {
        formatManager.registerBasicFormats();
        readAheadThread.startThread();

        deviceError = deviceManager.initialise (0, 2, nullptr, true);
        devicePlayer.setSource (this);
        deviceManager.addAudioCallback (&devicePlayer);
        setInputGainDB (defaultInputGainDB);
    }

    ~LiveMonitor() override
    {
        deviceManager.removeAudioCallback (&devicePlayer);
        devicePlayer.setSource (nullptr);
        transport.stop();
        transport.setSource (nullptr);
        readerSource.reset();
        readAheadThread.stopThread (1000);
    }

    bool loadFile (const juce::File& file, juce::String& error)
    {
        if (! file.existsAsFile())
        {
            error = "The selected source file does not exist.";
            return false;
        }

        transport.stop();
        transport.setSource (nullptr);
        readerSource.reset();

        std::unique_ptr<juce::AudioFormatReader> reader (formatManager.createReaderFor (file));
        if (reader == nullptr)
        {
            error = "The selected source cannot be opened as an audio file.";
            return false;
        }

        const auto sourceRate = reader->sampleRate;
        readerSource = std::make_unique<juce::AudioFormatReaderSource> (reader.release(), true);
        readerSource->setLooping (loopSource);
        transport.setSource (readerSource.get(),
                             32768,
                             &readAheadThread,
                             sourceRate,
                             2);
        sourceFile = file;
        return true;
    }

    void playFromStart()
    {
        if (! canPlay())
            return;
        transport.setPosition (0.0);
        transport.start();
    }

    void stop()
    {
        transport.stop();
    }

    bool isPlaying() const noexcept
    {
        return transport.isPlaying();
    }

    bool canPlay() const noexcept
    {
        const auto* device = deviceManager.getCurrentAudioDevice();
        return readerSource != nullptr
            && device != nullptr
            && device->getActiveOutputChannels().countNumberOfSetBits() >= 2;
    }

    void setLooping (bool shouldLoop)
    {
        loopSource = shouldLoop;
        if (readerSource != nullptr)
            readerSource->setLooping (shouldLoop);
    }

    bool isLooping() const noexcept
    {
        return loopSource;
    }

    void setInputGainDB (double decibels)
    {
        inputGainDB = juce::jlimit (-60.0, 0.0, decibels);
        transport.setGain (juce::Decibels::decibelsToGain (static_cast<float> (inputGainDB)));
    }

    double getInputGainDB() const noexcept
    {
        return inputGainDB;
    }

    void setHostBypassed (bool shouldBypass) noexcept
    {
        hostBypassed.store (shouldBypass);
    }

    bool isHostBypassed() const noexcept
    {
        return hostBypassed.load();
    }

    const juce::File& getSourceFile() const noexcept
    {
        return sourceFile;
    }

    juce::AudioDeviceManager& getDeviceManager() noexcept
    {
        return deviceManager;
    }

    juce::String getStatusText()
    {
        auto* device = deviceManager.getCurrentAudioDevice();
        if (device == nullptr)
            return "No output device is open. Use Audio Device... to select a stereo output.";

        const auto outputChannels = device->getActiveOutputChannels().countNumberOfSetBits();
        if (outputChannels < 2)
            return "A stereo output is required for this witness session. Use Audio Device... to enable two output channels.";

        if (readerSource == nullptr)
            return "Choose Source... to load a local WAV or AIFF file. Playback never starts automatically.";

        const auto sourceName = sourceFile.getFileName();
        const auto state = transport.isPlaying() ? "playing" : "ready";
        const auto route = hostBypassed.load() ? "host bypass A/B" : "through VST3";
        return juce::String ("Live monitor ") + state + " | " + route + " | " + sourceName
             + " | " + device->getName()
             + " | " + juce::String (device->getCurrentSampleRate(), 0) + " Hz";
    }

private:
    juce::AudioProcessor& plugin;
    juce::AudioDeviceManager deviceManager;
    juce::AudioSourcePlayer devicePlayer;
    juce::AudioFormatManager formatManager;
    juce::TimeSliceThread readAheadThread;
    juce::AudioTransportSource transport;
    std::unique_ptr<juce::AudioFormatReaderSource> readerSource;
    juce::File sourceFile;
    std::atomic<bool> hostBypassed { false };
    juce::String deviceError;
    double inputGainDB = defaultInputGainDB;
    bool loopSource = true;
    bool processorPrepared = false;

    void prepareToPlay (int samplesPerBlockExpected, double sampleRate) override
    {
        const auto inputs = juce::jmax (1, plugin.getTotalNumInputChannels());
        const auto outputs = juce::jmax (1, plugin.getTotalNumOutputChannels());
        plugin.setPlayConfigDetails (inputs, outputs, sampleRate, samplesPerBlockExpected);
        plugin.prepareToPlay (sampleRate, samplesPerBlockExpected);
        processorPrepared = true;
        transport.prepareToPlay (samplesPerBlockExpected, sampleRate);
    }

    void releaseResources() override
    {
        transport.releaseResources();
        if (processorPrepared)
        {
            plugin.releaseResources();
            processorPrepared = false;
        }
    }

    void getNextAudioBlock (const juce::AudioSourceChannelInfo& info) override
    {
        transport.getNextAudioBlock (info);
        if (! processorPrepared || hostBypassed.load() || info.buffer == nullptr || info.numSamples <= 0)
            return;

        juce::MidiBuffer midi;
        plugin.processBlock (*info.buffer, midi);
    }

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (LiveMonitor)
};

class WitnessContent final : public juce::Component,
                             private juce::Timer
{
public:
    WitnessContent (juce::AudioPluginInstance& instance,
                    const juce::File& initialSource,
                    bool loopSource,
                    double inputGainDB)
        : plugin (instance), monitor (plugin)
    {
        if (plugin.hasEditor())
            editor.reset (plugin.createEditorIfNeeded());
        if (editor == nullptr)
            editor = std::make_unique<juce::GenericAudioProcessorEditor> (plugin);

        playButton.setTooltip ("Start the current source from the beginning. It never starts automatically.");
        stopButton.setTooltip ("Stop live monitoring.");
        loopButton.setTooltip ("Loop the selected source while you operate the plug-in GUI.");
        hostBypassButton.setTooltip ("Direct source-to-output A/B. This does not change the plug-in bypass parameter.");
        sourceButton.setTooltip ("Choose a local WAV or AIFF file for real-time monitoring.");
        deviceButton.setTooltip ("Choose the Windows output device and stereo channels.");

        loopButton.setClickingTogglesState (true);
        loopButton.setToggleState (loopSource, juce::dontSendNotification);
        hostBypassButton.setClickingTogglesState (true);
        inputGainSlider.setRange (-60.0, 0.0, 0.5);
        inputGainSlider.setValue (inputGainDB, juce::dontSendNotification);
        inputGainSlider.setTextBoxStyle (juce::Slider::TextBoxRight, false, 62, 24);
        inputGainSlider.setTextValueSuffix (" dB");

        playButton.onClick = [this] { monitor.playFromStart(); };
        stopButton.onClick = [this] { monitor.stop(); };
        loopButton.onClick = [this] { monitor.setLooping (loopButton.getToggleState()); };
        hostBypassButton.onClick = [this] { monitor.setHostBypassed (hostBypassButton.getToggleState()); };
        sourceButton.onClick = [this] { chooseSource(); };
        deviceButton.onClick = [this] { showAudioSettings(); };
        inputGainSlider.onValueChange = [this]
        {
            monitor.setInputGainDB (inputGainSlider.getValue());
        };

        for (auto* component : { static_cast<juce::Component*> (&playButton),
                                 static_cast<juce::Component*> (&stopButton),
                                 static_cast<juce::Component*> (&loopButton),
                                 static_cast<juce::Component*> (&hostBypassButton),
                                 static_cast<juce::Component*> (&sourceButton),
                                 static_cast<juce::Component*> (&deviceButton),
                                 static_cast<juce::Component*> (&inputGainSlider),
                                 static_cast<juce::Component*> (&sourceLabel),
                                 static_cast<juce::Component*> (&statusLabel),
                                 static_cast<juce::Component*> (editor.get()) })
            addAndMakeVisible (*component);

        sourceLabel.setJustificationType (juce::Justification::centredLeft);
        sourceLabel.setMinimumHorizontalScale (0.7f);
        statusLabel.setJustificationType (juce::Justification::centredLeft);
        statusLabel.setMinimumHorizontalScale (0.7f);
        statusLabel.setColour (juce::Label::textColourId, juce::Colours::lightgrey);

        monitor.setLooping (loopSource);
        monitor.setInputGainDB (inputGainDB);
        if (initialSource.existsAsFile())
        {
            juce::String error;
            if (! monitor.loadFile (initialSource, error))
                sourceMessage = "The bundled Probe Audio could not be loaded: " + error;
        }

        const auto editorWidth = juce::jmax (560, editor->getWidth());
        const auto editorHeight = juce::jmax (300, editor->getHeight());
        setSize (editorWidth, editorHeight + controlsHeight);
        refreshLabels();
        startTimerHz (4);
    }

    bool isEditorResizable() const noexcept
    {
        return editor->isResizable();
    }

    void resized() override
    {
        auto area = getLocalBounds();
        auto controls = area.removeFromTop (controlsHeight).reduced (8, 6);
        const auto firstRow = controls.removeFromTop (28);
        auto buttons = firstRow;
        playButton.setBounds (buttons.removeFromLeft (68));
        buttons.removeFromLeft (5);
        stopButton.setBounds (buttons.removeFromLeft (60));
        buttons.removeFromLeft (5);
        loopButton.setBounds (buttons.removeFromLeft (58));
        buttons.removeFromLeft (5);
        hostBypassButton.setBounds (buttons.removeFromLeft (126));
        buttons.removeFromLeft (5);
        sourceButton.setBounds (buttons.removeFromLeft (94));
        buttons.removeFromLeft (5);
        deviceButton.setBounds (buttons.removeFromLeft (108));
        buttons.removeFromLeft (12);
        inputGainSlider.setBounds (buttons.removeFromLeft (170));

        controls.removeFromTop (5);
        sourceLabel.setBounds (controls.removeFromTop (20));
        statusLabel.setBounds (controls.removeFromTop (20));
        editor->setBounds (area);
    }

private:
    static constexpr int controlsHeight = 86;

    juce::AudioPluginInstance& plugin;
    LiveMonitor monitor;
    std::unique_ptr<juce::AudioProcessorEditor> editor;
    juce::TextButton playButton { "Play" };
    juce::TextButton stopButton { "Stop" };
    juce::ToggleButton loopButton { "Loop" };
    juce::ToggleButton hostBypassButton { "Host Bypass A/B" };
    juce::TextButton sourceButton { "Source..." };
    juce::TextButton deviceButton { "Audio Device..." };
    juce::Slider inputGainSlider { juce::Slider::LinearHorizontal, juce::Slider::TextBoxRight };
    juce::Label sourceLabel;
    juce::Label statusLabel;
    std::unique_ptr<juce::FileChooser> fileChooser;
    juce::String sourceMessage;

    void timerCallback() override
    {
        refreshLabels();
    }

    void refreshLabels()
    {
        const auto source = monitor.getSourceFile();
        if (source.existsAsFile())
            sourceLabel.setText ("Source: " + source.getFullPathName(), juce::dontSendNotification);
        else
            sourceLabel.setText ("Source: none selected (the bundled Probe Audio is loaded automatically when available)", juce::dontSendNotification);

        auto status = monitor.getStatusText();
        if (sourceMessage.isNotEmpty())
            status = sourceMessage + "\n" + status;
        statusLabel.setText (status, juce::dontSendNotification);
        playButton.setEnabled (monitor.canPlay());
        stopButton.setEnabled (monitor.isPlaying());
        hostBypassButton.setToggleState (monitor.isHostBypassed(), juce::dontSendNotification);
    }

    void chooseSource()
    {
        fileChooser = std::make_unique<juce::FileChooser> (
            "Choose a listening source",
            monitor.getSourceFile(),
            "*.wav;*.wave;*.aif;*.aiff");
        const auto safeThis = juce::Component::SafePointer<WitnessContent> (this);
        fileChooser->launchAsync (juce::FileBrowserComponent::openMode | juce::FileBrowserComponent::canSelectFiles,
                                  [safeThis] (const juce::FileChooser& chooser)
                                  {
                                      if (safeThis == nullptr)
                                          return;
                                      const auto file = chooser.getResult();
                                      safeThis->fileChooser.reset();
                                      if (! file.existsAsFile())
                                          return;
                                      safeThis->loadSource (file);
                                  });
    }

    void loadSource (const juce::File& file)
    {
        juce::String error;
        if (! monitor.loadFile (file, error))
        {
            sourceMessage = "Could not load source: " + error;
            return;
        }
        sourceMessage.clear();
        refreshLabels();
    }

    void showAudioSettings()
    {
        auto selector = std::make_unique<juce::AudioDeviceSelectorComponent> (
            monitor.getDeviceManager(), 0, 0, 2, 2, false, false, true, true);
        selector->setSize (520, 340);

        juce::DialogWindow::LaunchOptions options;
        options.dialogTitle = "Plugin Probe Witness Audio Device";
        options.dialogBackgroundColour = juce::Colours::darkgrey;
        options.content.setOwned (selector.release());
        options.componentToCentreAround = this;
        options.useNativeTitleBar = true;
        options.resizable = true;
        options.launchAsync();
    }

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (WitnessContent)
};

class WitnessWindow final : public juce::DocumentWindow
{
public:
    WitnessWindow (const juce::String& title,
                   std::unique_ptr<juce::AudioPluginInstance> instance,
                   const juce::File& initialSource,
                   bool loopSource,
                   double inputGainDB)
        : juce::DocumentWindow (title,
                                juce::Desktop::getInstance().getDefaultLookAndFeel().findColour (backgroundColourId),
                                allButtons),
          plugin (std::move (instance))
    {
        setUsingNativeTitleBar (true);
        auto* content = new WitnessContent (*plugin, initialSource, loopSource, inputGainDB);
        const auto width = content->getWidth();
        const auto height = content->getHeight();
        setContentOwned (content, true);
        setResizable (content->isEditorResizable(), false);
        centreWithSize (width, height);
        setVisible (true);
    }

    ~WitnessWindow() override
    {
        // The editor and live monitor must be deleted while the processor is
        // still alive. LiveMonitor then releases processor resources before
        // the isolated plug-in instance is destroyed.
        clearContentComponent();
        plugin.reset();
    }

    void closeButtonPressed() override
    {
        juce::JUCEApplicationBase::quit();
    }

private:
    std::unique_ptr<juce::AudioPluginInstance> plugin;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (WitnessWindow)
};

class WitnessApplication final : public juce::JUCEApplication
{
public:
    const juce::String getApplicationName() override    { return JUCE_APPLICATION_NAME_STRING; }
    const juce::String getApplicationVersion() override { return JUCE_APPLICATION_VERSION_STRING; }
    bool moreThanOneInstanceAllowed() override          { return true; }

    void initialise (const juce::String&) override
    {
        options = parseOptions (getCommandLineParameterArray());
        if (options.error.isNotEmpty())
        {
            showFailureAndQuit (options.error);
            return;
        }

        // Let the native message loop enter its normal state before a third
        // party editor is instantiated. Some VST3s display modal UI while
        // opening and require an unblocked message thread on Windows.
        juce::Timer::callAfterDelay (1, [this] { openWitness(); });
    }

    void shutdown() override
    {
        window = nullptr;
    }

    void systemRequestedQuit() override
    {
        quit();
    }

private:
    LaunchOptions options;
    juce::AudioPluginFormatManager formats;
    std::unique_ptr<WitnessWindow> window;

    static LaunchOptions parseOptions (const juce::StringArray& args)
    {
        LaunchOptions parsed;
        juce::String pluginPath;
        bool audioFileWasExplicit = false;

        for (int index = 0; index < args.size(); ++index)
        {
            const auto argument = args[index].trim();
            const auto valueFor = [&] (const juce::String& flag) -> juce::String
            {
                if (index + 1 >= args.size())
                {
                    parsed.error = flag + " requires a value";
                    return {};
                }
                return args[++index].trim();
            };

            if (argument == "--plugin-path" || argument == "--plugin")
                pluginPath = valueFor (argument);
            else if (argument == "--audio-file" || argument == "--source")
            {
                parsed.audioFile = juce::File (valueFor (argument));
                audioFileWasExplicit = true;
            }
            else if (argument == "--sample-rate")
                parsed.sampleRate = valueFor (argument).getDoubleValue();
            else if (argument == "--block-size")
                parsed.blockSize = valueFor (argument).getIntValue();
            else if (argument == "--input-gain-db")
                parsed.inputGainDB = valueFor (argument).getDoubleValue();
            else if (argument == "--no-loop")
                parsed.loopSource = false;
            else if (argument == "--help" || argument == "-h")
                parsed.error = "Usage: pluginprobe_vst3_witness.exe --plugin-path <plugin.vst3> [--audio-file <source.wav>] [--no-loop] [--input-gain-db -18] [--sample-rate 48000] [--block-size 512]";
            else if (argument.isNotEmpty())
                parsed.error = "unsupported argument: " + argument;

            if (parsed.error.isNotEmpty())
                return parsed;
        }

        parsed.pluginFile = juce::File (pluginPath);
        if (pluginPath.isEmpty())
            parsed.error = "--plugin-path is required";
        else if (! parsed.pluginFile.existsAsFile())
            parsed.error = "VST3 file does not exist: " + parsed.pluginFile.getFullPathName();
        else if (! parsed.pluginFile.hasFileExtension (".vst3"))
            parsed.error = "only .vst3 plugins are supported by the witness host";
        else if (! std::isfinite (parsed.sampleRate) || parsed.sampleRate < 8000.0 || parsed.sampleRate > 384000.0)
            parsed.error = "--sample-rate must be within 8000..384000";
        else if (parsed.blockSize < 16 || parsed.blockSize > 8192)
            parsed.error = "--block-size must be within 16..8192";
        else if (! std::isfinite (parsed.inputGainDB) || parsed.inputGainDB < -60.0 || parsed.inputGainDB > 0.0)
            parsed.error = "--input-gain-db must be within -60..0";
        else if (audioFileWasExplicit && ! parsed.audioFile.existsAsFile())
            parsed.error = "--audio-file does not exist: " + parsed.audioFile.getFullPathName();
        else if (! audioFileWasExplicit)
            parsed.audioFile = defaultProbeAudioFile();
        return parsed;
    }

    void openWitness()
    {
        juce::addDefaultFormatsToManager (formats);

        juce::AudioPluginFormat* vst3 = nullptr;
        for (auto* format : formats.getFormats())
            if (format != nullptr && format->getName().equalsIgnoreCase ("VST3"))
                vst3 = format;

        if (vst3 == nullptr)
        {
            showFailureAndQuit ("The witness host was built without VST3 support.");
            return;
        }

        juce::OwnedArray<juce::PluginDescription> descriptions;
        vst3->findAllTypesForFile (descriptions, options.pluginFile.getFullPathName());
        if (descriptions.isEmpty())
        {
            showFailureAndQuit ("No VST3 audio processor class was found in:\n" + options.pluginFile.getFullPathName());
            return;
        }

        juce::String failure;
        auto plugin = formats.createPluginInstance (*descriptions.getFirst(), options.sampleRate, options.blockSize, failure);
        if (plugin == nullptr)
        {
            showFailureAndQuit ("Could not load the VST3:\n" + (failure.isNotEmpty() ? failure : options.pluginFile.getFullPathName()));
            return;
        }

        plugin->setPlayConfigDetails (juce::jmax (1, descriptions.getFirst()->numInputChannels),
                                      juce::jmax (1, descriptions.getFirst()->numOutputChannels),
                                      options.sampleRate,
                                      options.blockSize);
        plugin->prepareToPlay (options.sampleRate, options.blockSize);

        const auto title = "Plugin Probe Witness - " + descriptions.getFirst()->name + " (live, isolated)";
        window = std::make_unique<WitnessWindow> (title,
                                                  std::move (plugin),
                                                  options.audioFile,
                                                  options.loopSource,
                                                  options.inputGainDB);
    }

    void showFailureAndQuit (const juce::String& message)
    {
        juce::AlertWindow::showMessageBoxAsync (juce::MessageBoxIconType::WarningIcon,
                                                 "Plugin Probe Witness",
                                                 message,
                                                 "Close",
                                                 nullptr,
                                                 new QuitAfterDialog());
    }
};
} // namespace

START_JUCE_APPLICATION (WitnessApplication)
