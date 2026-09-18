#include <juce_audio_formats/juce_audio_formats.h>
#include <juce_audio_processors_headless/juce_audio_processors_headless.h>
#include <juce_events/juce_events.h>

#include <public.sdk/source/vst/hosting/module.h>

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <iostream>
#include <string>
#include <vector>
#if defined(_WIN32)
 #ifndef NOMINMAX
  #define NOMINMAX
 #endif
 #include <windows.h>
 #include <bcrypt.h>
 #include <io.h>
 #include <fcntl.h>
#else
 // macOS observation host: the bcrypt/io.h CRT surface is replaced by
 // CommonCrypto and the POSIX dup/open family. std::filesystem drives the
 // bundle-directory fingerprint walk that single-file Windows shells never
 // needed.
 #include <CommonCrypto/CommonDigest.h>
 #include <fcntl.h>
 #include <unistd.h>
 #include <filesystem>
 #include <system_error>
#endif

namespace
{
constexpr auto workerProtocol = "vit.pluginprobe.vst3_worker.v1";
constexpr size_t maxLogEntries = 256;
constexpr size_t maxLogLength = 512;

using Object = juce::DynamicObject;

juce::var makeObject()
{
    return juce::var (new Object());
}

Object* asObject (juce::var& value)
{
    return value.getDynamicObject();
}

const Object* asObject (const juce::var& value)
{
    return value.getDynamicObject();
}

void set (juce::var& value, const char* name, juce::var item)
{
    if (auto* object = asObject (value))
        object->setProperty (name, std::move (item));
}

juce::String stringProperty (const Object& object, const char* name)
{
    return object.getProperty (name).toString().trim();
}

// WaveShell members and a few third-party wrappers may print diagnostics to
// the inherited stdout stream while JUCE scans or instantiates them. Keep the
// worker's NDJSON protocol isolated from that observation-only plugin output.
class ScopedPluginStdoutSilencer final
{
public:
    ScopedPluginStdoutSilencer()
    {
        std::cout.flush();
#if defined(_WIN32)
        saved = _dup (_fileno (stdout));
        if (saved < 0)
            return;
        const auto nullFile = _open ("NUL", _O_WRONLY);
        if (nullFile < 0)
        {
            _close (saved);
            saved = -1;
            return;
        }
        _dup2 (nullFile, _fileno (stdout));
        _close (nullFile);
#else
        saved = dup (fileno (stdout));
        if (saved < 0)
            return;
        const auto nullFile = ::open ("/dev/null", O_WRONLY);
        if (nullFile < 0)
        {
            ::close (saved);
            saved = -1;
            return;
        }
        ::dup2 (nullFile, fileno (stdout));
        ::close (nullFile);
#endif
        active = true;
    }

    ~ScopedPluginStdoutSilencer()
    {
        if (active)
        {
            std::cout.flush();
#if defined(_WIN32)
            _dup2 (saved, _fileno (stdout));
            _close (saved);
#else
            ::dup2 (saved, fileno (stdout));
            ::close (saved);
#endif
        }
    }

    ScopedPluginStdoutSilencer (const ScopedPluginStdoutSilencer&) = delete;
    ScopedPluginStdoutSilencer& operator= (const ScopedPluginStdoutSilencer&) = delete;

private:
    int saved = -1;
    bool active = false;
};

double numberProperty (const Object& object, const char* name, double fallback = 0.0)
{
    const auto value = object.getProperty (name);
    return value.isDouble() || value.isInt() || value.isInt64() ? static_cast<double> (value) : fallback;
}

int intProperty (const Object& object, const char* name, int fallback = 0)
{
    const auto value = object.getProperty (name);
    if (value.isInt() || value.isInt64() || value.isDouble())
        return static_cast<int> (value);
    const auto text = value.toString().trim();
    if (text.isEmpty())
        return fallback;
    return text.getIntValue();
}

bool boolProperty (const Object& object, const char* name, bool fallback = false)
{
    const auto value = object.getProperty (name);
    return value.isBool() ? static_cast<bool> (value) : fallback;
}

// Incremental SHA-256 over the platform crypto primitive: BCrypt on Windows,
// CommonCrypto on macOS. Both emit the same digest; only the framing below
// decides cross-platform fingerprint stability.
class Sha256Digest
{
public:
    Sha256Digest()
    {
#if defined(_WIN32)
        valid = BCryptOpenAlgorithmProvider (&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0) == 0
            && BCryptCreateHash (algorithm, &hash, nullptr, 0, nullptr, 0, 0) == 0;
#else
        valid = CC_SHA256_Init (&context) == 1;
#endif
    }

    ~Sha256Digest()
    {
#if defined(_WIN32)
        if (hash != nullptr)
            BCryptDestroyHash (hash);
        if (algorithm != nullptr)
            BCryptCloseAlgorithmProvider (algorithm, 0);
#endif
    }

    bool update (const void* data, size_t size)
    {
        if (! valid)
            return false;
#if defined(_WIN32)
        return BCryptHashData (hash, reinterpret_cast<PUCHAR> (const_cast<void*> (data)),
                               static_cast<ULONG> (size), 0) == 0;
#else
        return CC_SHA256_Update (&context, data, static_cast<CC_LONG> (size)) == 1;
#endif
    }

    bool finish (unsigned char (&digest)[32])
    {
        if (! valid)
            return false;
#if defined(_WIN32)
        return BCryptFinishHash (hash, digest, sizeof (digest), 0) == 0;
#else
        return CC_SHA256_Final (digest, &context) == 1;
#endif
    }

    Sha256Digest (const Sha256Digest&) = delete;
    Sha256Digest& operator= (const Sha256Digest&) = delete;

private:
    bool valid = false;
#if defined(_WIN32)
    BCRYPT_ALG_HANDLE algorithm = nullptr;
    BCRYPT_HASH_HANDLE hash = nullptr;
#else
    CC_SHA256_CTX context {};
#endif
};

juce::String encodeDigest (const unsigned char (&digest)[32])
{
    juce::String encoded;
    for (const auto byte : digest)
        encoded += juce::String::toHexString (static_cast<int> (byte)).paddedLeft ('0', 2);
    return "sha256:" + encoded;
}

juce::String hashBytes (const void* data, size_t size)
{
    Sha256Digest digest;
    unsigned char result[32];
    if (! digest.update (data, size) || ! digest.finish (result))
        return {};
    return encodeDigest (result);
}

#if ! defined(_WIN32)
// Mirrors the Go processor attestation FingerprintPath bundle grammar exactly:
// the domain tag "vit-pca-bundle-v1\0", then for every regular file in sorted
// path order a big-endian length-framed slash-separated relative path and a
// big-endian size-framed byte payload. Symlinks anywhere in the bundle are
// rejected fail-closed, matching the Go admission-side behaviour (R4).
juce::String bundleHash (const juce::File& bundleRoot, juce::String& failureReason)
{
    namespace fs = std::filesystem;
    std::error_code error;
    const fs::path root = fs::path (bundleRoot.getFullPathName().toStdString());
    std::vector<fs::path> files;
    for (fs::recursive_directory_iterator it (root, fs::directory_options::none, error), end;
         ! error && it != end;
         it.increment (error))
    {
        const auto status = it->symlink_status (error);
        if (error)
            break;
        if (status.type() == fs::file_type::symlink)
        {
            failureReason = "bundle contains symlink " + juce::String (it->path().string());
            return {};
        }
        if (status.type() == fs::file_type::regular)
            files.push_back (it->path());
    }
    if (error)
    {
        failureReason = "bundle walk failed: " + juce::String (error.message());
        return {};
    }
    if (files.empty())
    {
        failureReason = "bundle contains no regular files";
        return {};
    }
    std::sort (files.begin(), files.end());
    Sha256Digest digest;
    const char domainTag[] = "vit-pca-bundle-v1";
    if (! digest.update (domainTag, sizeof (domainTag))) // includes the trailing NUL
    {
        failureReason = "digest domain tag failed";
        return {};
    }
    for (const auto& file : files)
    {
        const auto relative = fs::relative (file, root, error).generic_string();
        if (error)
        {
            failureReason = "bundle relative path failed for " + juce::String (file.string());
            return {};
        }
        const auto frameLength = [&] (uint64_t value)
        {
            unsigned char frame[8] {};
            for (int index = 0; index < 8; ++index)
                frame[index] = static_cast<unsigned char> (value >> (56 - 8 * index));
            return digest.update (frame, sizeof (frame));
        };
        if (! frameLength (relative.size()) || ! digest.update (relative.data(), relative.size()))
        {
            failureReason = "digest path frame failed for " + juce::String (file.string());
            return {};
        }
        juce::MemoryBlock bytes;
        juce::File input (file.string());
        std::unique_ptr<juce::FileInputStream> stream (input.createInputStream());
        // readIntoMemoryBlock returns the byte count: 0 is a legitimate
        // success for empty marker files such as the macOS "Icon\r".
        if (stream == nullptr || stream->readIntoMemoryBlock (bytes) < 0)
        {
            failureReason = "bundle file unreadable: " + input.getFullPathName();
            return {};
        }
        if (! frameLength (bytes.getSize()) || ! digest.update (bytes.getData(), bytes.getSize()))
        {
            failureReason = "digest payload frame failed for " + input.getFullPathName();
            return {};
        }
    }
    unsigned char result[32];
    if (! digest.finish (result))
    {
        failureReason = "digest finish failed";
        return {};
    }
    return encodeDigest (result);
}
#endif

juce::String fileHash (const juce::File& file, juce::String* failureReason = nullptr)
{
#if ! defined(_WIN32)
    if (file.isDirectory())
    {
        // macOS VST3 shells are bundle directories; the single-file stream
        // below only applies to the Windows file-form shells.
        juce::String reason;
        const auto fingerprint = bundleHash (file, reason);
        if (fingerprint.isEmpty())
        {
            if (failureReason != nullptr)
                *failureReason = reason;
            std::cerr << "pluginprobe worker: installation fingerprint failed for "
                      << file.getFullPathName() << ": " << reason << std::endl;
        }
        return fingerprint;
    }
#endif
    std::unique_ptr<juce::FileInputStream> stream (file.createInputStream());
    if (stream == nullptr)
    {
        if (failureReason != nullptr)
            *failureReason = "file unreadable: " + file.getFullPathName();
        return {};
    }
    juce::MemoryBlock bytes;
    stream->readIntoMemoryBlock (bytes);
    return hashBytes (bytes.getData(), bytes.getSize());
}

class Worker final
{
public:
    Worker()
    {
        juce::addHeadlessDefaultFormatsToManager (formats);
        audioFormats.registerBasicFormats();
        addLog ("worker initialized");
    }

    juce::var dispatch (const juce::var& request)
    {
        const auto* object = asObject (request);
        if (object == nullptr)
            return error ("invalid_request", "request must be a JSON object");

        const auto op = stringProperty (*object, "op").toLowerCase();
        if (op == "load") return load (*object);
        if (op == "snapshot") return snapshotResponse();
        if (op == "render") return render (*object);
        if (op == "unload") return unload();
        if (op == "shutdown") return success (makeObject());
        return error ("unsupported_operation", "unsupported operation: " + op);
    }

private:
    juce::AudioPluginFormatManager formats;
    juce::AudioFormatManager audioFormats;
    std::unique_ptr<juce::AudioPluginInstance> plugin;
    juce::PluginDescription description;
    juce::File pluginFile;
    double sampleRate = 48000.0;
    int blockSize = 512;
    std::vector<juce::String> logs;

    void addLog (juce::String message)
    {
        message = message.replaceCharacters ("\r\n", "  ").substring (0, static_cast<int> (maxLogLength));
        logs.push_back (juce::Time::getCurrentTime().toISO8601 (true) + " " + message);
        if (logs.size() > maxLogEntries)
            logs.erase (logs.begin(), logs.begin() + static_cast<std::ptrdiff_t> (logs.size() - maxLogEntries));
    }

    juce::var error (const juce::String& code, const juce::String& message)
    {
        addLog ("error " + code + ": " + message);
        auto response = makeObject();
        set (response, "ok", false);
        set (response, "protocol", workerProtocol);
        auto issue = makeObject();
        set (issue, "code", code);
        set (issue, "message", message);
        set (response, "error", issue);
        set (response, "logs", logsArray());
        return response;
    }

    juce::var success (juce::var result)
    {
        auto response = makeObject();
        set (response, "ok", true);
        set (response, "protocol", workerProtocol);
        set (response, "result", std::move (result));
        set (response, "logs", logsArray());
        return response;
    }

    juce::var logsArray() const
    {
        juce::Array<juce::var> result;
        for (const auto& entry : logs)
            result.add (entry);
        return result;
    }

    bool loaded (juce::var& failure)
    {
        if (plugin != nullptr)
            return true;
        failure = error ("plugin_not_loaded", "load a VST3 before this operation");
        return false;
    }

    juce::AudioPluginFormat* vst3Format()
    {
        for (auto* format : formats.getFormats())
            if (format != nullptr && format->getName().equalsIgnoreCase ("VST3"))
                return format;
        return nullptr;
    }

    bool instantiate (juce::String& failure)
    {
        auto instance = formats.createPluginInstance (description, sampleRate, blockSize, failure);
        if (instance == nullptr)
            return false;

        instance->setPlayConfigDetails (std::max (1, description.numInputChannels),
                                        std::max (1, description.numOutputChannels),
                                        sampleRate,
                                        blockSize);
        instance->prepareToPlay (sampleRate, blockSize);
        plugin = std::move (instance);
        return true;
    }

    juce::var load (const Object& request)
    {
        if (plugin != nullptr)
            return error ("plugin_already_loaded", "unload the current VST3 before loading another one");

        const auto path = stringProperty (request, "plugin_path");
        if (path.isEmpty())
            return error ("plugin_path_required", "load requires plugin_path");
        pluginFile = juce::File (path);
        // macOS VST3 shells are bundle directories; Windows shells are single
        // files. Both forms are accepted observation targets.
        if (! pluginFile.existsAsFile() && ! pluginFile.isDirectory())
            return error ("plugin_not_found", "VST3 file does not exist: " + pluginFile.getFullPathName());
        if (! pluginFile.hasFileExtension (".vst3"))
            return error ("not_vst3", "only .vst3 files are supported by this worker");

        const auto requestedRate = numberProperty (request, "sample_rate", 48000.0);
        const auto requestedBlockSize = static_cast<int> (numberProperty (request, "block_size", 512.0));
        if (! std::isfinite (requestedRate) || requestedRate < 8000.0 || requestedRate > 384000.0)
            return error ("invalid_sample_rate", "sample_rate must be within 8000..384000");
        if (requestedBlockSize < 16 || requestedBlockSize > 8192)
            return error ("invalid_block_size", "block_size must be within 16..8192");
        sampleRate = requestedRate;
        blockSize = requestedBlockSize;

        auto* format = vst3Format();
        if (format == nullptr)
            return error ("vst3_host_unavailable", "JUCE VST3 host support was not compiled into this worker");

        const auto requestedName = stringProperty (request, "plugin_name");
        const auto requestedUID = stringProperty (request, "plugin_uid");
        juce::OwnedArray<juce::PluginDescription> descriptions;
        if (requestedUID.isNotEmpty())
        {
            description.name = requestedName;
            description.descriptiveName = requestedName;
            description.pluginFormatName = "VST3";
            description.fileOrIdentifier = pluginFile.getFullPathName();
            description.uniqueId = intProperty (request, "plugin_uid");
            description.deprecatedUid = description.uniqueId;
            description.numInputChannels = intProperty (request, "num_inputs", 2);
            description.numOutputChannels = intProperty (request, "num_outputs", 2);
            description.manufacturerName = stringProperty (request, "manufacturer");
            description.version = stringProperty (request, "version");
        }
        else
        {
            {
                ScopedPluginStdoutSilencer silencePluginOutput;
                format->findAllTypesForFile (descriptions, pluginFile.getFullPathName());
            }
            if (descriptions.isEmpty())
                return error ("scan_failed", "no VST3 audio processor class was found in " + pluginFile.getFullPathName());

            if (requestedName.isNotEmpty())
            {
                for (const auto* candidate : descriptions)
                {
                    if (candidate != nullptr
                        && (candidate->name.equalsIgnoreCase (requestedName)
                            || candidate->descriptiveName.equalsIgnoreCase (requestedName)))
                    {
                        description = *candidate;
                        break;
                    }
                }
                if (description.name.isEmpty())
                {
                    juce::StringArray available;
                    for (const auto* candidate : descriptions)
                        if (candidate != nullptr)
                            available.add (candidate->name);
                    return error ("plugin_member_not_found", "no VST3 member named " + requestedName
                        + "; available members: " + available.joinIntoString (", "));
                }
            }
            else
            {
                description = *descriptions.getFirst();
            }
        }
        juce::String loadFailure;
        {
            ScopedPluginStdoutSilencer silencePluginOutput;
            if (! instantiate (loadFailure))
                return error ("load_failed", loadFailure.isNotEmpty() ? loadFailure : "VST3 instance creation failed");
        }

        addLog ("loaded " + description.name + " from " + pluginFile.getFullPathName());
        return snapshotResponse();
    }

    juce::String parameterID (const juce::AudioProcessorParameter& parameter, bool& stable) const
    {
        if (const auto* hosted = dynamic_cast<const juce::HostedAudioProcessorParameter*> (&parameter))
        {
            const auto id = hosted->getParameterID().trim();
            if (id.isNotEmpty())
            {
                stable = true;
                return id;
            }
        }
        stable = false;
        return "host-index:" + juce::String (parameter.getParameterIndex());
    }

    juce::var parameterSnapshot() const
    {
        juce::Array<juce::var> result;
        for (auto* parameter : plugin->getParameters())
        {
            if (parameter == nullptr)
                continue;
            bool stable = false;
            const auto id = parameterID (*parameter, stable);
            auto item = makeObject();
            set (item, "id", id);
            set (item, "id_provenance", stable ? "vst3_hosted_parameter_id" : "host_parameter_index_fallback");
            set (item, "stable_id", stable);
            set (item, "host_label", parameter->getName (1024));
            set (item, "unit", parameter->getLabel());
            set (item, "normalized_value", parameter->getValue());
            set (item, "display_value", parameter->getCurrentValueAsText());
            set (item, "default_normalized_value", parameter->getDefaultValue());
            set (item, "automation", parameter->isAutomatable() ? "automatable" : "not_automatable");
            set (item, "is_discrete", parameter->isDiscrete());
            set (item, "is_boolean", parameter->isBoolean());
            set (item, "is_meta", parameter->isMetaParameter());
            set (item, "num_steps", parameter->getNumSteps());
            set (item, "category", static_cast<int> (parameter->getCategory()));
            juce::Array<juce::var> values;
            for (const auto& value : parameter->getAllValueStrings())
                values.add (value);
            set (item, "display_choices", values);
            result.add (item);
        }
        return result;
    }

    juce::var busSnapshot (bool input) const
    {
        juce::Array<juce::var> result;
        for (int index = 0; index < plugin->getBusCount (input); ++index)
        {
            const auto* bus = plugin->getBus (input, index);
            if (bus == nullptr)
                continue;
            auto item = makeObject();
            set (item, "index", index);
            set (item, "direction", input ? "input" : "output");
            set (item, "name", bus->getName());
            set (item, "enabled", bus->isEnabled());
            set (item, "channel_count", bus->getNumberOfChannels());
            set (item, "layout", bus->getCurrentLayout().getDescription());
            result.add (item);
        }
        return result;
    }

    juce::var classInfoSnapshot() const
    {
        juce::Array<juce::var> classes;
        std::string moduleError;
        if (auto module = VST3::Hosting::Module::create (pluginFile.getFullPathName().toStdString(), moduleError))
        {
            for (const auto& info : module->getFactory().classInfos())
            {
                auto item = makeObject();
                set (item, "class_id", juce::String (info.ID().toString()));
                set (item, "name", juce::String (info.name()));
                set (item, "vendor", juce::String (info.vendor()));
                set (item, "version", juce::String (info.version()));
                set (item, "category", juce::String (info.category()));
                set (item, "subcategories", juce::String (info.subCategoriesString()));
                classes.add (item);
            }
        }
        else
        {
            juce::ignoreUnused (moduleError);
        }
        return classes;
    }

    juce::var snapshot()
    {
        auto result = makeObject();
        auto identity = makeObject();
        set (identity, "manufacturer", description.manufacturerName);
        set (identity, "name", description.name);
        set (identity, "descriptive_name", description.descriptiveName);
        set (identity, "version", description.version);
        set (identity, "format", "VST3");
        set (identity, "install_path", pluginFile.getFullPathName());
        juce::String fingerprintFailure;
        const auto installationFingerprint = fileHash (pluginFile, &fingerprintFailure);
        if (installationFingerprint.isEmpty() && fingerprintFailure.isNotEmpty())
            addLog ("installation fingerprint rejected: " + fingerprintFailure);
        set (identity, "file_fingerprint", installationFingerprint);
        set (identity, "juce_unique_id", juce::String::toHexString (description.uniqueId));
        set (identity, "juce_identifier", description.createIdentifierString());
        set (identity, "class_ids", classInfoSnapshot());
        set (result, "identity", identity);
        set (result, "parameters", parameterSnapshot());
        set (result, "input_buses", busSnapshot (true));
        set (result, "output_buses", busSnapshot (false));
        set (result, "latency_samples", plugin->getLatencySamples());
        set (result, "tail_seconds", plugin->getTailLengthSeconds());
        set (result, "sample_rate", sampleRate);
        set (result, "block_size", blockSize);
        set (result, "captured_at", juce::Time::getCurrentTime().toISO8601 (true));
        return result;
    }

    juce::var snapshotResponse()
    {
        juce::var failure;
        if (! loaded (failure))
            return failure;
        return success (snapshot());
    }

    juce::var render (const Object& request)
    {
        juce::var failureResponse;
        if (! loaded (failureResponse))
            return failureResponse;
        const auto inputPath = stringProperty (request, "input_path");
        const auto outputPath = stringProperty (request, "output_path");
        if (inputPath.isEmpty() || outputPath.isEmpty())
            return error ("render_paths_required", "render requires input_path and output_path");
        const auto source = juce::File (inputPath);
        const auto destination = juce::File (outputPath);
        if (! source.existsAsFile())
            return error ("render_input_missing", "input audio does not exist: " + source.getFullPathName());
        if (! destination.getParentDirectory().createDirectory())
            return error ("render_directory_failed", "could not create output directory");
        destination.deleteFile();
        std::unique_ptr<juce::AudioFormatReader> reader (audioFormats.createReaderFor (source));
        if (reader == nullptr)
            return error ("render_input_unreadable", "could not open input audio");
        if (std::abs (reader->sampleRate - sampleRate) > 0.001)
            return error ("render_sample_rate_mismatch", "probe audio must match the loaded worker sample rate");

        const int outputChannels = std::max (1, plugin->getTotalNumOutputChannels());
        auto stream = destination.createOutputStream();
        if (stream == nullptr)
            return error ("render_output_failed", "could not create output audio");
        juce::WavAudioFormat wav;
        std::unique_ptr<juce::AudioFormatWriter> writer (wav.createWriterFor (stream.release(),
                                                                                sampleRate,
                                                                                static_cast<unsigned int> (outputChannels),
                                                                                24,
                                                                                juce::StringPairArray {},
                                                                                0));
        if (writer == nullptr)
            return error ("render_output_failed", "could not create WAV writer");

        const auto bypass = boolProperty (request, "bypass", false);
        const auto requestedTail = std::max (0.0, numberProperty (request, "tail_seconds", plugin->getTailLengthSeconds()));
        const auto tailFrames = static_cast<juce::int64> (std::min (requestedTail, 30.0) * sampleRate);
        const int channelCount = std::max ({ 2,
                                             static_cast<int> (reader->numChannels),
                                             plugin->getTotalNumInputChannels(),
                                             outputChannels });
        juce::AudioBuffer<float> buffer (channelCount, blockSize);
        juce::MidiBuffer midi;
        juce::int64 position = 0;
        const auto inputFrames = reader->lengthInSamples;
        const auto totalFrames = inputFrames + tailFrames;
        while (position < totalFrames)
        {
            const auto count = static_cast<int> (std::min<juce::int64> (blockSize, totalFrames - position));
            buffer.clear();
            if (position < inputFrames)
                reader->read (&buffer, 0, count, position, true, true);
            midi.clear();
            if (bypass)
                plugin->processBlockBypassed (buffer, midi);
            else
                plugin->processBlock (buffer, midi);
            writer->writeFromAudioSampleBuffer (buffer, 0, count);
            position += count;
        }
        writer.reset();
        auto result = makeObject();
        set (result, "input_path", source.getFullPathName());
        set (result, "input_sha256", fileHash (source));
        set (result, "output_path", destination.getFullPathName());
        set (result, "output_sha256", fileHash (destination));
        set (result, "bypass", bypass);
        set (result, "rendered_frames", position);
        set (result, "tail_seconds_requested", requestedTail);
        set (result, "fresh_readback", snapshot());
        addLog ("offline render completed: " + destination.getFullPathName());
        return success (result);
    }

    juce::var unload()
    {
        if (plugin != nullptr)
        {
            plugin->releaseResources();
            plugin.reset();
        }
        description = {};
        // AppleClang rejects `= {}` against juce::File's assignment set; the
        // default-constructed invalid File is the intended reset either way.
        pluginFile = juce::File();
        addLog ("plugin unloaded");
        auto result = makeObject();
        set (result, "unloaded", true);
        return success (result);
    }
};

void writeResponse (const juce::var& response)
{
    std::cout << juce::JSON::toString (response, true).toStdString() << std::endl;
}
} // namespace

int main()
{
    juce::ScopedJuceInitialiser_GUI juceInitialiser;
    Worker worker;
    std::string line;
    while (std::getline (std::cin, line))
    {
        if (line.empty())
            continue;
        const auto request = juce::JSON::parse (juce::String::fromUTF8 (line.c_str(), static_cast<int> (line.size())));
        if (request.isVoid())
        {
            auto response = makeObject();
            set (response, "ok", false);
            set (response, "protocol", workerProtocol);
            auto error = makeObject();
            set (error, "code", "invalid_json");
            set (error, "message", "input line is not valid JSON");
            set (response, "error", error);
            writeResponse (response);
            continue;
        }
        const auto response = worker.dispatch (request);
        writeResponse (response);
        if (const auto* object = asObject (request); object != nullptr && stringProperty (*object, "op").equalsIgnoreCase ("shutdown"))
            break;
    }
    return 0;
}
