#include "ZmqLogger.h"

namespace vit
{

std::mutex ZmqLogger::queueMutex;
std::deque<ZmqLogger::QueuedLogMessage> ZmqLogger::pendingMessages;

ZmqLogger::ZmqLogger (std::unique_ptr<juce::FileLogger> wrappedLogger)
    : wrapped (std::move (wrappedLogger))
{
}

void ZmqLogger::logMessage (const juce::String& message)
{
    enqueueMessage ({ inferLevel (message), message });

    if (wrapped != nullptr)
        wrapped->logMessage (message);
}

std::vector<ZmqLogger::QueuedLogMessage> ZmqLogger::drainPendingMessages()
{
    std::lock_guard<std::mutex> lock (queueMutex);

    std::vector<QueuedLogMessage> drained;
    drained.reserve (pendingMessages.size());

    while (! pendingMessages.empty())
    {
        drained.push_back (std::move (pendingMessages.front()));
        pendingMessages.pop_front();
    }

    return drained;
}

juce::String ZmqLogger::inferLevel (const juce::String& message)
{
    const auto lower = message.toLowerCase();

    if (lower.contains ("error") || lower.contains ("failed") || lower.contains ("fatal"))
        return "ERROR";

    if (lower.contains ("warn"))
        return "WARN";

    return "INFO";
}

void ZmqLogger::enqueueMessage (QueuedLogMessage message)
{
    std::lock_guard<std::mutex> lock (queueMutex);
    pendingMessages.push_back (std::move (message));
}

} // namespace vit
