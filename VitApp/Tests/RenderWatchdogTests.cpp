#include "../Source/Service/VitProductionCoordinator.h"

#include <cassert>
#include <chrono>
#include <string>
#include <thread>
#include <vector>

namespace
{

struct CapturedEvent
{
    juce::String topic;
    juce::String subtopic;
    juce::String jobId;
    juce::String status;
    juce::String source;
};

class CapturingLogger final : public juce::Logger
{
public:
    void logMessage (const juce::String& message) override { messages.push_back (message); }

    bool contains (const juce::String& needle) const
    {
        for (const auto& message : messages)
            if (message.contains (needle))
                return true;
        return false;
    }

    std::vector<juce::String> messages;
};

CapturedEvent parseEvent (const juce::String& published)
{
    const auto parsed = juce::JSON::parse (published);
    const auto* object = parsed.getDynamicObject();
    assert (object != nullptr);

    CapturedEvent event;
    event.topic = object->getProperty ("topic").toString();
    event.subtopic = object->getProperty ("subtopic").toString();
    event.jobId = object->getProperty ("job_id").toString();
    event.status = object->getProperty ("status").toString();
    event.source = object->getProperty ("source").toString();
    return event;
}

void sleepForMs (int ms)
{
    std::this_thread::sleep_for (std::chrono::milliseconds (ms));
}

} // namespace

int main()
{
    CapturingLogger capturingLogger;
    juce::Logger::setCurrentLogger (&capturingLogger);

    std::vector<juce::String> published;
    vit::VitProductionCoordinator coordinator (
        [&published] (const juce::String& message) { published.push_back (message); });

    // 1. Armed watchdog must not fire before its deadline (no false positive).
    coordinator.simulateWedgedRenderForTest (300.0);
    assert (coordinator.isRendering());
    for (int i = 0; i < 3; ++i)
    {
        coordinator.tick();
        assert (coordinator.isRendering());
    }
    assert (published.empty());

    // 2. Wedged render past its deadline: tick clears the rendering flag (the
    //    exact gate CommandDispatcher checks) and publishes one watchdog
    //    render_failed event — subsequent start_render commands are accepted.
    coordinator.simulateWedgedRenderForTest (0.1);
    assert (coordinator.isRendering());
    sleepForMs (250);
    coordinator.tick();
    assert (! coordinator.isRendering());
    assert (published.size() == 1);

    const auto event = parseEvent (published[0]);
    assert (event.topic == "render");
    assert (event.subtopic == "render_failed");
    assert (event.status == "error");
    assert (event.source == "render_watchdog");
    assert (event.jobId.isNotEmpty());

    // 3. One-shot: the watchdog disarms itself; further ticks publish nothing.
    coordinator.tick();
    coordinator.tick();
    assert (! coordinator.isRendering());
    assert (published.size() == 1);

    // 4. Repeated self-heal: a later wedge also recovers.
    coordinator.simulateWedgedRenderForTest (0.05);
    assert (coordinator.isRendering());
    sleepForMs (120);
    coordinator.tick();
    assert (! coordinator.isRendering());
    assert (published.size() == 2);

    // 5. Watchdog events reach the kernel log (juce::Logger) as WARN.
    assert (capturingLogger.contains ("[render_watchdog]"));
    assert (capturingLogger.messages.size() == 2);

    juce::Logger::setCurrentLogger (nullptr);
    return 0;
}
