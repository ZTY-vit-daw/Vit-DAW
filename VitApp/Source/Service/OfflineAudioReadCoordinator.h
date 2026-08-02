#pragma once

#include <memory>
#include <mutex>
#include <string>
#include <unordered_map>

#include <JuceHeader.h>

namespace vit
{

// JUCE readers are independent objects, but opening the same large source from
// multiple offline projection workers has proven non-deterministic on Windows.
// This coordinator only serialises full-file reads of the same source path;
// unrelated stems may still be prepared in parallel.
class OfflineAudioReadLease final
{
public:
    explicit OfflineAudioReadLease (std::shared_ptr<std::mutex> sourceMutexIn)
        : sourceMutex (std::move (sourceMutexIn)), lock (*sourceMutex)
    {
    }

    OfflineAudioReadLease (OfflineAudioReadLease&&) noexcept = default;
    OfflineAudioReadLease& operator= (OfflineAudioReadLease&&) noexcept = default;

    OfflineAudioReadLease (const OfflineAudioReadLease&) = delete;
    OfflineAudioReadLease& operator= (const OfflineAudioReadLease&) = delete;

private:
    std::shared_ptr<std::mutex> sourceMutex;
    std::unique_lock<std::mutex> lock;
};

class OfflineAudioReadCoordinator final
{
public:
    static OfflineAudioReadLease acquire (const juce::String& filePath)
    {
        const auto key = juce::File (filePath).getFullPathName()
                             .replaceCharacter ('/', '\\')
                             .toLowerCase()
                             .toStdString();

        std::shared_ptr<std::mutex> sourceMutex;
        {
            std::lock_guard<std::mutex> registryLock (registryMutex());
            auto& weak = registry()[key];
            sourceMutex = weak.lock();
            if (sourceMutex == nullptr)
            {
                sourceMutex = std::make_shared<std::mutex>();
                weak = sourceMutex;
            }
        }
        return OfflineAudioReadLease (std::move (sourceMutex));
    }

private:
    static std::mutex& registryMutex()
    {
        static std::mutex value;
        return value;
    }

    static std::unordered_map<std::string, std::weak_ptr<std::mutex>>& registry()
    {
        static std::unordered_map<std::string, std::weak_ptr<std::mutex>> value;
        return value;
    }
};

} // namespace vit
