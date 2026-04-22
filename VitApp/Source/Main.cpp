#include <JuceHeader.h>

#include <sodium.h>

#include "Core/VitPaths.h"
#include "Service/SharedMemoryTester.h"
#include "Service/ZmqLogger.h"
#include "Service/VitHeadlessService.h"

namespace vit
{

class VitHeadlessApplication final : public juce::JUCEApplication
{
public:
    VitHeadlessApplication() = default;

    const juce::String getApplicationName() override       { return ProjectInfo::projectName; }
    const juce::String getApplicationVersion() override    { return ProjectInfo::versionString; }
    bool moreThanOneInstanceAllowed() override             { return false; }

    void initialise (const juce::String&) override
    {
        initialiseLogger();

        juce::Logger::writeToLog ("VitHeadlessServer: application startup.");

        if (sodium_init() < 0)
            juce::Logger::writeToLog ("VitHeadlessServer: FATAL — libsodium sodium_init() failed.");

        const auto enableSharedMemoryTest = juce::SystemStats::getEnvironmentVariable ("VIT_ENABLE_SHARED_MEMORY_TEST", {})
                                                .trim()
                                                .equalsIgnoreCase ("1");
        if (enableSharedMemoryTest)
        {
            SharedMemoryTester::createTestMemory();
            juce::Logger::writeToLog ("VitHeadlessServer: shared memory test enabled by VIT_ENABLE_SHARED_MEMORY_TEST=1.");
        }

        service = std::make_unique<VitHeadlessService> (ProjectInfo::projectName);

        if (! service->start())
        {
            juce::Logger::writeToLog ("VitHeadlessServer: service startup failed.");
            quit();
            return;
        }

        juce::Logger::writeToLog ("VitHeadlessServer: startup complete, entering background message loop.");
    }

    void shutdown() override
    {
        juce::Logger::writeToLog ("VitHeadlessServer: application shutdown.");

        if (service != nullptr)
            service->stop();

        service.reset();
        SharedMemoryTester::releaseTestMemory();

        juce::Logger::setCurrentLogger (nullptr);
        zmqLogger.reset();
    }

    void systemRequestedQuit() override
    {
        juce::Logger::writeToLog ("VitHeadlessServer: system requested quit.");
        quit();
    }

private:
    void initialiseLogger()
    {
        auto logDirectory = paths::getLogsDirectory();

        if (! logDirectory.isDirectory() && ! logDirectory.createDirectory())
        {
            juce::Logger::writeToLog ("VitHeadlessServer: failed to create log directory: " + logDirectory.getFullPathName());
        }

        auto fileLogger = std::unique_ptr<juce::FileLogger> (juce::FileLogger::createDateStampedLogger (logDirectory.getFullPathName(),
                                                                                                          "VitHeadlessServer",
                                                                                                          ".log",
                                                                                                          "VitHeadlessServer session log"));

        zmqLogger = std::make_unique<ZmqLogger> (std::move (fileLogger));
        juce::Logger::setCurrentLogger (zmqLogger.get());
    }

    std::unique_ptr<ZmqLogger> zmqLogger;
    std::unique_ptr<VitHeadlessService> service;

    JUCE_DECLARE_NON_COPYABLE_WITH_LEAK_DETECTOR (VitHeadlessApplication)
};

} // namespace vit

START_JUCE_APPLICATION (vit::VitHeadlessApplication)
