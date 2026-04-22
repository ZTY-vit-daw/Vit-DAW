#pragma once

#include <deque>
#include <memory>
#include <mutex>

#include <JuceHeader.h>

namespace vit
{

class ZmqLogger final : public juce::Logger
{
public:
    struct QueuedLogMessage
    {
        juce::String level;
        juce::String message;
    };

    explicit ZmqLogger (std::unique_ptr<juce::FileLogger> wrappedLogger);

    void logMessage (const juce::String& message) override;

    static std::vector<QueuedLogMessage> drainPendingMessages();

private:
    static juce::String inferLevel (const juce::String& message);
    static void enqueueMessage (QueuedLogMessage message);

    std::unique_ptr<juce::FileLogger> wrapped;

    static std::mutex queueMutex;
    static std::deque<QueuedLogMessage> pendingMessages;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (ZmqLogger)
};

} // namespace vit
