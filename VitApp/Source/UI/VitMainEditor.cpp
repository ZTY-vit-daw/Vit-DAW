#include "VitMainEditor.h"

#include "../../../tracktion_engine/examples/common/Utilities.h"
#include "../../../tracktion_engine/examples/common/Components.h"

#include "../Core/VitKernelUtils.h"
#include "../Core/VitPaths.h"

namespace vit
{

VitMainEditor::VitMainEditor (VitEngineDevice& device)
    : engineDevice (device),
      selectionManager (engineDevice.getEngine())
{
    addAndMakeVisible (reloadButton);
    addAndMakeVisible (audioSettingsButton);
    addAndMakeVisible (statusLabel);

    statusLabel.setJustificationType (juce::Justification::centredLeft);
    statusLabel.setText ("Waiting for generated_project.xml", juce::dontSendNotification);

    reloadButton.onClick = [this] { reloadEditFromDefaultXml(); };
    audioSettingsButton.onClick = [this] { EngineHelpers::showAudioDeviceSettings (engineDevice.getEngine()); };

    reloadEditFromDefaultXml();
}

VitMainEditor::~VitMainEditor() = default;

void VitMainEditor::paint (juce::Graphics& g)
{
    g.fillAll (getLookAndFeel().findColour (juce::ResizableWindow::backgroundColourId));
}

void VitMainEditor::resized()
{
    auto bounds = getLocalBounds().reduced (8);
    auto topRow = bounds.removeFromTop (32);

    reloadButton.setBounds (topRow.removeFromLeft (120));
    topRow.removeFromLeft (8);
    audioSettingsButton.setBounds (topRow.removeFromLeft (140));
    topRow.removeFromLeft (8);
    statusLabel.setBounds (topRow);

    bounds.removeFromTop (8);

    if (editComponent != nullptr)
        editComponent->setBounds (bounds);
}

void VitMainEditor::reloadEditFromDefaultXml()
{
    const auto xmlFile = paths::getGeneratedProjectXmlFile();

    lastGhostSummary.clear();
    selectionManager.deselectAll();
    editComponent.reset();
    edit.reset();

    if (! xmlFile.existsAsFile())
    {
        setStatusMessage ("XML not found: " + xmlFile.getFullPathName());
        juce::Logger::writeToLog ("VitMainEditor: missing XML file: " + xmlFile.getFullPathName());
        return;
    }

    auto loadedEdit = loadEditFromXmlFile (engineDevice.getEngine(), xmlFile, 1, this);

    if (loadedEdit == nullptr)
    {
        setStatusMessage ("XML parse failed: " + xmlFile.getFileName());
        juce::Logger::writeToLog ("VitMainEditor: failed to load edit from XML.");
        return;
    }

    loadedEdit->playInStopEnabled = true;
    rebindAllWaveClipSourcesToDirectFiles (*loadedEdit);
    ensureTrackRackGraphForEdit (*loadedEdit);
    loadedEdit->dispatchPendingUpdatesSynchronously();
    loadedEdit->getTransport().ensureContextAllocated (true);

    edit = std::move (loadedEdit);
    editComponent = std::make_unique<EditComponent> (*edit, selectionManager);

    auto& viewState = editComponent->getEditViewState();
    viewState.showHeaders = true;
    viewState.showFooters = true;
    viewState.showMidiDevices = true;
    viewState.showWaveDevices = true;

    addAndMakeVisible (*editComponent);

    auto message = "Loaded XML: " + xmlFile.getFileName();
    if (lastGhostSummary.isNotEmpty())
        message << " | " << lastGhostSummary;

    setStatusMessage (message);
    juce::Logger::writeToLog ("VitMainEditor: edit reloaded from " + xmlFile.getFullPathName());

    resized();
}

void VitMainEditor::setStatusMessage (const juce::String& message)
{
    statusLabel.setText (message, juce::dontSendNotification);
}

void VitMainEditor::onEditReloaded (const juce::File& xmlFile)
{
    juce::Logger::writeToLog ("VitMainEditor: semantic reload hook received " + xmlFile.getFullPathName());
}

void VitMainEditor::onGhostTrackDetected (const GhostTrackDescriptor& descriptor)
{
    const auto intent = descriptor.intent.isNotEmpty() ? descriptor.intent : "(none)";
    lastGhostSummary = descriptor.trackName + " intent=" + intent;
}

} // namespace vit
