#include "VitPluginUIBehaviour.h"

namespace vit
{
namespace
{
namespace te = tracktion::engine;

inline bool isDPIAware (te::Plugin&)
{
    return true;
}

//==============================================================================
/** Space toggles transport (aligned with CommandDispatcher play/stop). Plain space only so
    modified chords still reach the plugin. */
class PluginSpaceKeyListener final : public juce::KeyListener
{
public:
    explicit PluginSpaceKeyListener (te::Plugin& pluginRef) : plugin (pluginRef) {}

    bool keyPressed (const juce::KeyPress& key, juce::Component*) override;

private:
    te::Plugin& plugin;
};

//==============================================================================
class PluginWindow : public juce::DocumentWindow
{
public:
    PluginWindow (te::Plugin& p, bool addToDesktopFlag);
    ~PluginWindow() override;

    static std::unique_ptr<juce::Component> create (te::Plugin&);

    void show();

    void setEditor (std::unique_ptr<te::Plugin::EditorComponent>);
    te::Plugin::EditorComponent* getEditor() const { return editor.get(); }

    void recreateEditor();
    void recreateEditorAsync();

private:
    void moved() override;
    void userTriedToCloseWindow() override { plugin.windowState->closeWindowExplicitly(); }
    void closeButtonPressed() override { userTriedToCloseWindow(); }
    float getDesktopScaleFactor() const override { return 1.0f; }

    std::unique_ptr<te::Plugin::EditorComponent> editor;

    te::Plugin& plugin;
    te::PluginWindowState& windowState;
    PluginSpaceKeyListener spaceKeyListener;
    bool updateStoredBounds = false;
};

#if JUCE_LINUX
constexpr bool shouldAddPluginWindowToDesktop = false;
#else
constexpr bool shouldAddPluginWindowToDesktop = true;
#endif

PluginWindow::PluginWindow (te::Plugin& plug, bool addToDesktopFlag)
    : DocumentWindow (plug.getName(),
                      juce::Colours::black,
                      juce::DocumentWindow::closeButton,
                      addToDesktopFlag),
      plugin (plug),
      windowState (*plug.windowState),
      spaceKeyListener (plug)
{
    getConstrainer()->setMinimumOnscreenAmounts (0x10000, 50, 30, 50);
    setResizeLimits (100, 50, 4000, 4000);

    setAlwaysOnTop (true);
    addKeyListener (&spaceKeyListener);

    recreateEditor();

    setBoundsConstrained (getLocalBounds() + plugin.windowState->choosePositionForPluginWindow());

#if JUCE_LINUX
    addToDesktop();
#endif

    updateStoredBounds = true;
}

PluginWindow::~PluginWindow()
{
    removeKeyListener (&spaceKeyListener);
    updateStoredBounds = false;
    plugin.edit.flushPluginStateIfNeeded (plugin);
    setEditor (nullptr);
}

void PluginWindow::show()
{
    setVisible (true);
    toFront (false);
    setBoundsConstrained (getBounds());
}

void PluginWindow::setEditor (std::unique_ptr<te::Plugin::EditorComponent> newEditor)
{
    JUCE_AUTORELEASEPOOL
    {
        setConstrainer (nullptr);
        editor.reset();

        if (newEditor != nullptr)
        {
            editor = std::move (newEditor);
            setContentNonOwned (editor.get(), true);
        }

        setResizable (editor == nullptr || editor->allowWindowResizing(), false);

        if (editor != nullptr && editor->allowWindowResizing())
            setConstrainer (editor->getBoundsConstrainer());
    }
}

std::unique_ptr<juce::Component> PluginWindow::create (te::Plugin& plugin)
{
    if (auto externalPlugin = dynamic_cast<te::ExternalPlugin*> (&plugin))
        if (externalPlugin->getAudioPluginInstance() == nullptr)
            return {};

    std::unique_ptr<PluginWindow> w;

    {
        struct Blocker : public juce::Component
        {
            void inputAttemptWhenModal() override {}
        };

        Blocker blocker;
        blocker.enterModalState (false);

#if JUCE_WINDOWS && JUCE_WIN_PER_MONITOR_DPI_AWARE
        if (! isDPIAware (plugin))
        {
            juce::ScopedDPIAwarenessDisabler disableDPIAwareness;
            w = std::make_unique<PluginWindow> (plugin, shouldAddPluginWindowToDesktop);
        }
        else
#endif
        {
            w = std::make_unique<PluginWindow> (plugin, shouldAddPluginWindowToDesktop);
        }
    }

    if (w == nullptr || w->getEditor() == nullptr)
        return {};

    w->show();

    return w;
}

void PluginWindow::recreateEditor()
{
    setEditor (nullptr);
    setEditor (plugin.createEditor());
}

void PluginWindow::recreateEditorAsync()
{
    setEditor (nullptr);

    juce::Timer::callAfterDelay (50, [this, sp = juce::Component::SafePointer<Component> (this)]
    {
        if (sp != nullptr)
            recreateEditor();
    });
}

void PluginWindow::moved()
{
    if (updateStoredBounds)
    {
        plugin.windowState->lastWindowBounds = getBounds();
        plugin.edit.pluginChanged (plugin);
    }
}

bool PluginSpaceKeyListener::keyPressed (const juce::KeyPress& key, juce::Component*)
{
    if (! key.isKeyCode (juce::KeyPress::spaceKey))
        return false;

    if (key.getModifiers().isAnyModifierKeyDown())
        return false;

    auto& transport = plugin.edit.getTransport();

    if (transport.isPlaying())
    {
        transport.stop (false, false);
        transport.ensureContextAllocated();
    }
    else
    {
        transport.ensureContextAllocated();
        transport.play (false);
    }

    return true;
}

} // namespace

//==============================================================================
std::unique_ptr<juce::Component> VitPluginUIBehaviour::createPluginWindow (te::PluginWindowState& pws)
{
    if (auto ws = dynamic_cast<te::Plugin::WindowState*> (&pws))
        return PluginWindow::create (ws->plugin);

    return {};
}

void VitPluginUIBehaviour::recreatePluginWindowContentAsync (te::Plugin& p)
{
    if (auto* w = dynamic_cast<PluginWindow*> (p.windowState->pluginWindow.get()))
        return w->recreateEditorAsync();

    te::UIBehaviour::recreatePluginWindowContentAsync (p);
}

} // namespace vit
