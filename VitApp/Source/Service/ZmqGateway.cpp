#include "ZmqGateway.h"

#include "../Core/VitDeltaProbe.h"

#include <cstdio>

namespace vit
{

namespace
{

juce::String toJuceString (const zmq::message_t& message)
{
    return juce::String::fromUTF8 (static_cast<const char*> (message.data()),
                                   static_cast<int> (message.size()));
}

bool sendString (zmq::socket_t& socket, const juce::String& payload, zmq::send_flags flags)
{
    const auto utf8 = payload.toStdString();
    return socket.send (zmq::buffer (utf8), flags).has_value();
}

juce::String toJsonResponse (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String buildLogPayload (const ZmqLogger::QueuedLogMessage& logMessage)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("type", "log");
    response->setProperty ("level", logMessage.level);
    response->setProperty ("message", logMessage.message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String buildDeltaUpdateJson (const DeltaEvent& event)
{
    auto o = std::make_unique<juce::DynamicObject>();
    o->setProperty ("type", "delta_update");
    o->setProperty ("seq_id", (juce::int64) event.seq_id);
    o->setProperty ("action", event.action);
    o->setProperty ("target_uid", event.target_uid);
    o->setProperty ("value", event.value);
    o->setProperty ("timestamp", event.timestamp);
    return juce::JSON::toString (juce::var (o.release()));
}

int timeoutMsFromVar (const juce::var& value, int fallback)
{
    if (value.isInt() || value.isInt64() || value.isDouble())
        return juce::roundToInt (static_cast<double> (value));

    const auto text = value.toString().trim();
    if (text.isEmpty())
        return fallback;

    const auto parsed = text.getIntValue();
    return parsed > 0 ? parsed : fallback;
}

int commandTimeoutMsForPayload (const juce::String& payload)
{
    constexpr int defaultTimeoutMs = 5000;
    constexpr int minTimeoutMs = 1000;
    constexpr int maxTimeoutMs = 300000;

    const auto parsed = juce::JSON::parse (payload);
    auto* object = parsed.getDynamicObject();

    if (object == nullptr)
        return defaultTimeoutMs;

    auto requested = timeoutMsFromVar (object->getProperty ("command_timeout_ms"), 0);
    if (requested <= 0)
        requested = timeoutMsFromVar (object->getProperty ("kernel_command_timeout_ms"), 0);

    return requested > 0 ? juce::jlimit (minTimeoutMs, maxTimeoutMs, requested)
                         : defaultTimeoutMs;
}

void writeToStdErr (const juce::String& message)
{
    std::fputs ((message + "\n").toRawUTF8(), stderr);
    std::fflush (stderr);
}

} // namespace

ZmqGateway::ZmqGateway (MessageHandler handler, VitDeltaRingBuffer* ring)
    : juce::Thread ("Vit ZMQ Gateway"),
      messageHandler (std::move (handler)),
      deltaRingBuffer (ring)
{
}

ZmqGateway::~ZmqGateway()
{
    stopGateway();
}

bool ZmqGateway::startGateway()
{
    if (isThreadRunning())
        return true;

    context = std::make_unique<zmq::context_t> (1);

    if (! bindSockets())
    {
        stopGateway();
        return false;
    }

    startThread();

    juce::Logger::writeToLog ("ZmqGateway: started network thread.");
    publishStatusMessage (R"({"event":"gateway_started","pub":"tcp://127.0.0.1:5556"})");
    return true;
}

void ZmqGateway::stopGateway()
{
    signalThreadShouldExit();
    waitForThreadToExit (2000);

    if (commandSocket != nullptr)
    {
        commandSocket->close();
        commandSocket.reset();
    }

    {
        const juce::ScopedLock lock (publishSocketLock);

        if (publishSocket != nullptr)
        {
            publishSocket->close();
            publishSocket.reset();
        }
    }

    {
        const juce::ScopedLock lock (logSocketLock);

        if (logSocket != nullptr)
        {
            logSocket->close();
            logSocket.reset();
        }
    }

    context.reset();
}

void ZmqGateway::run()
{
    constexpr auto commandPollTimeout = std::chrono::milliseconds (20);

    while (! threadShouldExit())
    {
        drainPendingLogMessages();
        drainDeltaRingBuffer();

        try
        {
            zmq::pollitem_t pollItems[] =
            {
                { *commandSocket, 0, ZMQ_POLLIN, 0 }
            };

            zmq::poll (&pollItems[0], 1, commandPollTimeout);
            drainPendingLogMessages();
            drainDeltaRingBuffer();

            if ((pollItems[0].revents & ZMQ_POLLIN) == 0)
                continue;

            zmq::message_t request;
            auto result = commandSocket->recv (request, zmq::recv_flags::dontwait);

            if (! result.has_value())
            {
                drainPendingLogMessages();
                continue;
            }

            const auto payload = toJuceString (request);
            juce::Logger::writeToLog ("ZmqGateway: received command payload: " + payload);
            const auto commandTimeoutMs = commandTimeoutMsForPayload (payload);
            if (commandTimeoutMs != 5000)
                juce::Logger::writeToLog ("ZmqGateway: command timeout budget ms=" + juce::String (commandTimeoutMs));

            auto responsePromise = std::make_shared<std::promise<juce::String>>();
            auto responseFuture = responsePromise->get_future();

            const auto posted = juce::MessageManager::callAsync ([handler = messageHandler, payload, responsePromise]
            {
                juce::Logger::writeToLog ("ZmqGateway: dispatching payload to JUCE message thread.");

                try
                {
                    const auto parsed = juce::JSON::parse (payload);

                    if (parsed.isVoid())
                    {
                        responsePromise->set_value (toJsonResponse ("error", "Invalid JSON payload"));
                        return;
                    }

                    // Future Edit/Engine mutations must always run on the JUCE message thread.
                    if (handler)
                    {
                        responsePromise->set_value (handler (parsed, payload));
                        return;
                    }

                    responsePromise->set_value (toJsonResponse ("error", "No command handler registered"));
                }
                catch (const std::exception& error)
                {
                    responsePromise->set_value (toJsonResponse ("error", "Command dispatch failed: " + juce::String (error.what())));
                }
            });

            juce::String replyPayload;

            if (! posted)
            {
                replyPayload = buildErrorReply ("Failed to post command to JUCE message thread");
            }
            else if (responseFuture.wait_for (std::chrono::milliseconds (commandTimeoutMs)) == std::future_status::ready)
            {
                replyPayload = responseFuture.get();
            }
            else
            {
                replyPayload = buildErrorReply ("Timed out waiting for JUCE command handling");
            }

            sendString (*commandSocket, replyPayload, zmq::send_flags::none);
            publishStatusMessage (R"({"event":"command_received","pub":"tcp://127.0.0.1:5556"})");
            drainPendingLogMessages();
        }
        catch (const zmq::error_t& error)
        {
            if (threadShouldExit())
                break;

            juce::Logger::writeToLog ("ZmqGateway: recv loop error: " + juce::String (error.what()));
            juce::Thread::sleep (50);
        }
    }

    drainPendingLogMessages();
    drainDeltaRingBuffer();
    juce::Logger::writeToLog ("ZmqGateway: network thread exiting.");
    drainPendingLogMessages();
}

bool ZmqGateway::bindSockets()
{
    try
    {
        commandSocket = std::make_unique<zmq::socket_t> (*context, zmq::socket_type::rep);
        publishSocket = std::make_unique<zmq::socket_t> (*context, zmq::socket_type::pub);
        logSocket = std::make_unique<zmq::socket_t> (*context, zmq::socket_type::pub);

        commandSocket->set (zmq::sockopt::linger, 0);
        publishSocket->set (zmq::sockopt::linger, 0);
        logSocket->set (zmq::sockopt::linger, 0);

        commandSocket->bind (commandEndpoint);
        publishSocket->bind (publishEndpoint);
        logSocket->bind (logEndpoint);

        juce::Logger::writeToLog ("ZmqGateway: bound command endpoint to " + juce::String (commandEndpoint));
        juce::Logger::writeToLog ("ZmqGateway: bound publish endpoint to " + juce::String (publishEndpoint));
        juce::Logger::writeToLog ("ZmqGateway: bound log endpoint to " + juce::String (logEndpoint));
        return true;
    }
    catch (const zmq::error_t& error)
    {
        juce::Logger::writeToLog ("ZmqGateway: failed to bind sockets: " + juce::String (error.what()));
        return false;
    }
}

void ZmqGateway::publishStatusMessage (const juce::String& payload)
{
    publishMessage (payload);
}

void ZmqGateway::drainPendingLogMessages()
{
    for (const auto& logMessage : ZmqLogger::drainPendingMessages())
        publishLogMessage (buildLogPayload (logMessage));
}

void ZmqGateway::drainDeltaRingBuffer()
{
    if (deltaRingBuffer == nullptr)
        return;

    const auto droppedCount = deltaRingBuffer->getDroppedCount();
    if (droppedCount != lastReportedDeltaDropCount)
    {
        juce::Logger::writeToLog ("ZmqGateway: delta ring dropped "
                                  + juce::String ((juce::int64) droppedCount)
                                  + " event(s) total; ready="
                                  + juce::String (deltaRingBuffer->getNumReady()));
        lastReportedDeltaDropCount = droppedCount;
    }

    DeltaEvent event;

    while (deltaRingBuffer->tryPop (event))
        publishMessage (buildDeltaUpdateJson (event));
}

void ZmqGateway::publishMessage (const juce::String& payload)
{
    const juce::ScopedLock lock (publishSocketLock);

    if (publishSocket == nullptr)
        return;

    try
    {
        if (! sendString (*publishSocket, payload, zmq::send_flags::dontwait))
        {
            ++publishSendFailureCount;
            juce::Logger::writeToLog ("ZmqGateway: PUB dontwait send returned false; failures="
                                      + juce::String ((juce::int64) publishSendFailureCount));
        }
    }
    catch (const zmq::error_t& error)
    {
        ++publishSendFailureCount;
        juce::Logger::writeToLog ("ZmqGateway: failed to publish status: "
                                  + juce::String (error.what())
                                  + "; failures="
                                  + juce::String ((juce::int64) publishSendFailureCount));
    }
}

void ZmqGateway::publishLogMessage (const juce::String& payload)
{
    const juce::ScopedLock lock (logSocketLock);

    if (logSocket == nullptr)
        return;

    try
    {
        sendString (*logSocket, payload, zmq::send_flags::dontwait);
    }
    catch (const zmq::error_t& error)
    {
        writeToStdErr ("ZmqGateway: failed to publish log: " + juce::String (error.what()));
    }
}

juce::String ZmqGateway::buildErrorReply (const juce::String& message) const
{
    return toJsonResponse ("error", message);
}

} // namespace vit
