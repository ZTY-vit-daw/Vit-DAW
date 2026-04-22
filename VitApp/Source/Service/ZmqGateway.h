#pragma once

#include <JuceHeader.h>
#include <future>
#include <zmq.hpp>

#include "ZmqLogger.h"

namespace vit
{

class VitDeltaRingBuffer;

class ZmqGateway final : public juce::Thread
{
public:
    using MessageHandler = std::function<juce::String (const juce::var&, const juce::String&)>;

    static constexpr const char* commandEndpoint = "tcp://127.0.0.1:5555";
    static constexpr const char* publishEndpoint = "tcp://127.0.0.1:5556";
    static constexpr const char* logEndpoint = "tcp://127.0.0.1:5557";

    explicit ZmqGateway (MessageHandler messageHandler, VitDeltaRingBuffer* deltaRingBuffer = nullptr);
    ~ZmqGateway() override;

    bool startGateway();
    void stopGateway();
    void publishMessage (const juce::String& payload);

    void run() override;

private:
    bool bindSockets();
    void drainPendingLogMessages();
    void drainDeltaRingBuffer();
    void publishStatusMessage (const juce::String& payload);
    void publishLogMessage (const juce::String& payload);
    juce::String buildErrorReply (const juce::String& message) const;

    MessageHandler messageHandler;
    VitDeltaRingBuffer* deltaRingBuffer = nullptr;
    std::unique_ptr<zmq::context_t> context;
    std::unique_ptr<zmq::socket_t> commandSocket;
    std::unique_ptr<zmq::socket_t> publishSocket;
    std::unique_ptr<zmq::socket_t> logSocket;
    juce::CriticalSection publishSocketLock;
    juce::CriticalSection logSocketLock;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (ZmqGateway)
};

} // namespace vit
