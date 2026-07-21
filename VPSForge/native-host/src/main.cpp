#include <juce_audio_formats/juce_audio_formats.h>
#include <juce_audio_processors_headless/juce_audio_processors_headless.h>
#include <juce_events/juce_events.h>

#include <public.sdk/source/vst/hosting/module.h>

#include <algorithm>
#ifndef NOMINMAX
 #define NOMINMAX
#endif
#include <windows.h>
#include <bcrypt.h>
#include <cmath>
#include <iostream>
#include <map>
#include <optional>
#include <string>
#include <vector>

namespace
{
constexpr auto workerProtocol = "vit.vpsforge.vst3_worker.v1";
constexpr size_t maxLogEntries = 256;
constexpr size_t maxLogLength = 512;
constexpr size_t maxStateBytes = 16u * 1024u * 1024u;

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

double numberProperty (const Object& object, const char* name, double fallback = 0.0)
{
    const auto value = object.getProperty (name);
    return value.isDouble() || value.isInt() || value.isInt64() ? static_cast<double> (value) : fallback;
}

bool boolProperty (const Object& object, const char* name, bool fallback = false)
{
    const auto value = object.getProperty (name);
    return value.isBool() ? static_cast<bool> (value) : fallback;
}

juce::String hashBytes (const void* data, size_t size)
{
    BCRYPT_ALG_HANDLE algorithm = nullptr;
    BCRYPT_HASH_HANDLE hash = nullptr;
    DWORD hashLength = 0;
    DWORD bytesWritten = 0;
    if (BCryptOpenAlgorithmProvider (&algorithm, BCRYPT_SHA256_ALGORITHM, nullptr, 0) != 0
        || BCryptGetProperty (algorithm, BCRYPT_HASH_LENGTH, reinterpret_cast<PUCHAR> (&hashLength), sizeof (hashLength), &bytesWritten, 0) != 0)
    {
        if (algorithm != nullptr)
            BCryptCloseAlgorithmProvider (algorithm, 0);
        return {};
    }
    std::vector<unsigned char> digest (hashLength);
    const auto status = BCryptCreateHash (algorithm, &hash, nullptr, 0, nullptr, 0, 0) == 0
        && BCryptHashData (hash, reinterpret_cast<PUCHAR> (const_cast<void*> (data)), static_cast<ULONG> (size), 0) == 0
        && BCryptFinishHash (hash, digest.data(), hashLength, 0) == 0;
    if (hash != nullptr)
        BCryptDestroyHash (hash);
    BCryptCloseAlgorithmProvider (algorithm, 0);
    if (! status)
        return {};
    juce::String encoded;
    for (const auto byte : digest)
        encoded += juce::String::toHexString (static_cast<int> (byte)).paddedLeft ('0', 2);
    return "sha256:" + encoded;
}

juce::String stateHash (const juce::MemoryBlock& state)
{
    return hashBytes (state.getData(), state.getSize());
}

juce::String fileHash (const juce::File& file)
{
    std::unique_ptr<juce::FileInputStream> stream (file.createInputStream());
    if (stream == nullptr)
        return {};
    juce::MemoryBlock bytes;
    stream->readIntoMemoryBlock (bytes);
    return hashBytes (bytes.getData(), bytes.getSize());
}

juce::String makeID (const juce::String& prefix, int serial)
{
    return prefix + "_" + juce::String (serial) + "_" + juce::String::toHexString (juce::Time::getMillisecondCounterHiRes());
}

bool finiteNormalised (double value)
{
    return std::isfinite (value) && value >= 0.0 && value <= 1.0;
}

struct ParameterValue
{
    juce::String id;
    float value = 0.0f;
};

struct Transaction
{
    juce::String id;
    juce::MemoryBlock state;
    std::vector<ParameterValue> parameters;
};

struct StoredState
{
    juce::String id;
    juce::MemoryBlock bytes;
};

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
        if (op == "write") return write (*object);
        if (op == "save_state") return saveState();
        if (op == "restore_state") return restoreState (*object);
        if (op == "roundtrip_state") return roundtripState();
        if (op == "rollback") return rollback (*object);
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
    std::map<std::string, Transaction> transactions;
    std::map<std::string, StoredState> states;
    std::vector<juce::String> logs;
    int serial = 0;

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
        if (! pluginFile.existsAsFile())
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

        juce::OwnedArray<juce::PluginDescription> descriptions;
        format->findAllTypesForFile (descriptions, pluginFile.getFullPathName());
        if (descriptions.isEmpty())
            return error ("scan_failed", "no VST3 audio processor class was found in " + pluginFile.getFullPathName());

        description = *descriptions.getFirst();
        juce::String loadFailure;
        if (! instantiate (loadFailure))
            return error ("load_failed", loadFailure.isNotEmpty() ? loadFailure : "VST3 instance creation failed");

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

    juce::var snapshot() const
    {
        auto result = makeObject();
        auto identity = makeObject();
        set (identity, "manufacturer", description.manufacturerName);
        set (identity, "name", description.name);
        set (identity, "descriptive_name", description.descriptiveName);
        set (identity, "version", description.version);
        set (identity, "format", "VST3");
        set (identity, "install_path", pluginFile.getFullPathName());
        set (identity, "file_fingerprint", fileHash (pluginFile));
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

    std::vector<ParameterValue> captureParameters() const
    {
        std::vector<ParameterValue> values;
        for (auto* parameter : plugin->getParameters())
        {
            if (parameter == nullptr)
                continue;
            bool unused = false;
            values.push_back ({ parameterID (*parameter, unused), parameter->getValue() });
        }
        return values;
    }

    juce::MemoryBlock captureState() const
    {
        juce::MemoryBlock state;
        plugin->getStateInformation (state);
        return state;
    }

    std::optional<size_t> parameterIndexForID (const juce::String& id) const
    {
        const auto parameters = plugin->getParameters();
        for (int index = 0; index < parameters.size(); ++index)
        {
            auto* parameter = parameters.getUnchecked (index);
            if (parameter == nullptr)
                continue;
            bool unused = false;
            if (parameterID (*parameter, unused) == id)
                return static_cast<size_t> (index);
        }
        return std::nullopt;
    }

    Transaction captureTransaction()
    {
        Transaction transaction;
        transaction.id = makeID ("tx", ++serial);
        transaction.state = captureState();
        transaction.parameters = captureParameters();
        return transaction;
    }

    juce::var transactionSummary (const Transaction& transaction) const
    {
        auto result = makeObject();
        set (result, "transaction_id", transaction.id);
        set (result, "preimage_state_sha256", stateHash (transaction.state));
        set (result, "preimage_parameter_count", static_cast<int> (transaction.parameters.size()));
        return result;
    }

    void dispatchMessages() const
    {
        if (auto* manager = juce::MessageManager::getInstanceWithoutCreating())
            manager->runDispatchLoopUntil (10);
    }

    // A hosted VST3 can expose a freshly changed controller value before its
    // processor component has consumed the corresponding automation event.
    // Advance one silent, prepared block before serialising state so that a
    // later reload/restore tests the same processor state that will render.
    // The surrounding write transaction has already captured a complete
    // preimage, and rollback restores that preimage after this isolated probe.
    void flushParameterChangesToProcessor()
    {
        if (plugin == nullptr)
            return;

        const auto channels = std::max (1, std::max (plugin->getTotalNumInputChannels(), plugin->getTotalNumOutputChannels()));
        juce::AudioBuffer<float> silence (channels, blockSize);
        silence.clear();
        juce::MidiBuffer midi;
        plugin->processBlock (silence, midi);
        dispatchMessages();
    }

    juce::var write (const Object& request)
    {
        juce::var failure;
        if (! loaded (failure))
            return failure;
        const auto changes = request.getProperty ("changes");
        const auto* values = changes.getArray();
        if (values == nullptr || values->isEmpty())
            return error ("changes_required", "write requires a non-empty changes array");

        auto transaction = captureTransaction();
        if (transaction.state.getSize() > maxStateBytes)
            return error ("state_too_large", "complete preimage state exceeds the configured 16 MiB limit");

        for (const auto& change : *values)
        {
            const auto* item = asObject (change);
            if (item == nullptr)
                return error ("invalid_change", "every parameter change must be an object");
            const auto id = stringProperty (*item, "id");
            const auto value = numberProperty (*item, "normalized", -1.0);
            if (id.isEmpty() || ! finiteNormalised (value))
                return error ("invalid_change", "changes require an id and normalized value within 0..1");
            const auto index = parameterIndexForID (id);
            if (! index.has_value())
                return error ("unknown_parameter", "parameter is not present in the fresh host surface: " + id);
            auto* parameter = plugin->getParameters().getUnchecked (static_cast<int> (*index));
            if (! parameter->isAutomatable())
                return error ("parameter_not_automatable", "refusing to write non-automatable parameter: " + id);
        }

        for (const auto& change : *values)
        {
            const auto* item = asObject (change);
            const auto index = parameterIndexForID (stringProperty (*item, "id"));
            auto* parameter = plugin->getParameters().getUnchecked (static_cast<int> (*index));
            parameter->beginChangeGesture();
            parameter->setValueNotifyingHost (static_cast<float> (numberProperty (*item, "normalized")));
            parameter->endChangeGesture();
        }
        flushParameterChangesToProcessor();

        transactions.emplace (transaction.id.toStdString(), transaction);
        addLog ("write completed with complete preimage " + transaction.id);
        auto result = transactionSummary (transaction);
        set (result, "fresh_readback", snapshot());
        return success (result);
    }

    juce::var saveState()
    {
        juce::var failure;
        if (! loaded (failure))
            return failure;
        auto state = captureState();
        if (state.getSize() > maxStateBytes)
            return error ("state_too_large", "serialized state exceeds the configured 16 MiB limit");
        StoredState saved { makeID ("state", ++serial), state };
        states.emplace (saved.id.toStdString(), saved);
        auto result = makeObject();
        set (result, "state_id", saved.id);
        set (result, "state_sha256", stateHash (saved.bytes));
        set (result, "state_bytes", static_cast<juce::int64> (saved.bytes.getSize()));
        set (result, "state_base64", juce::Base64::toBase64 (saved.bytes.getData(), saved.bytes.getSize()));
        return success (result);
    }

    bool decodeState (const Object& request, juce::MemoryBlock& state, juce::String& failure) const
    {
        const auto stateID = stringProperty (request, "state_id");
        if (stateID.isNotEmpty())
        {
            const auto found = states.find (stateID.toStdString());
            if (found == states.end())
            {
                failure = "unknown saved state: " + stateID;
                return false;
            }
            state = found->second.bytes;
            return true;
        }
        const auto encoded = stringProperty (request, "state_base64");
        if (encoded.isEmpty())
        {
            failure = "restore_state requires state_id or state_base64";
            return false;
        }
        juce::MemoryOutputStream stream (state, false);
        if (! juce::Base64::convertFromBase64 (stream, encoded) || state.getSize() == 0 || state.getSize() > maxStateBytes)
        {
            failure = "state_base64 is invalid, empty, or exceeds the 16 MiB limit";
            return false;
        }
        return true;
    }

    juce::var restoreState (const Object& request)
    {
        juce::var failureResponse;
        if (! loaded (failureResponse))
            return failureResponse;
        juce::MemoryBlock state;
        juce::String decodeFailure;
        if (! decodeState (request, state, decodeFailure))
            return error ("invalid_state", decodeFailure);
        plugin->setStateInformation (state.getData(), static_cast<int> (state.getSize()));
        dispatchMessages();
        auto result = makeObject();
        set (result, "restored_state_sha256", stateHash (state));
        set (result, "fresh_readback", snapshot());
        addLog ("state restored");
        return success (result);
    }

    bool recreate (juce::String& failure)
    {
        if (plugin != nullptr)
        {
            plugin->releaseResources();
            plugin.reset();
        }
        return instantiate (failure);
    }

    bool sameParameters (const std::vector<ParameterValue>& expected, juce::Array<juce::var>& mismatches) const
    {
        bool allMatch = true;
        for (const auto& value : expected)
        {
            const auto index = parameterIndexForID (value.id);
            const auto actual = index.has_value() ? plugin->getParameters().getUnchecked (static_cast<int> (*index))->getValue() : -1.0f;
            if (! index.has_value() || std::abs (actual - value.value) > 0.00001f)
            {
                allMatch = false;
                auto mismatch = makeObject();
                set (mismatch, "id", value.id);
                set (mismatch, "expected_normalized", value.value);
                set (mismatch, "actual_normalized", actual);
                mismatches.add (mismatch);
            }
        }
        return allMatch;
    }

    juce::var roundtripState()
    {
        juce::var failureResponse;
        if (! loaded (failureResponse))
            return failureResponse;
        const auto before = captureTransaction();
        if (before.state.getSize() > maxStateBytes)
            return error ("state_too_large", "serialized state exceeds the configured 16 MiB limit");
        juce::String recreateFailure;
        if (! recreate (recreateFailure))
            return error ("reload_failed", recreateFailure);
        plugin->setStateInformation (before.state.getData(), static_cast<int> (before.state.getSize()));
        dispatchMessages();
        juce::Array<juce::var> mismatches;
        const auto parametersMatch = sameParameters (before.parameters, mismatches);
        const auto after = captureState();
        auto result = makeObject();
        set (result, "pre_reload_state_sha256", stateHash (before.state));
        set (result, "post_restore_state_sha256", stateHash (after));
        set (result, "parameter_readback_matches_preimage", parametersMatch);
        set (result, "parameter_mismatches", mismatches);
        set (result, "fresh_readback", snapshot());
        addLog ("state roundtrip completed");
        return success (result);
    }

    juce::var rollback (const Object& request)
    {
        juce::var failureResponse;
        if (! loaded (failureResponse))
            return failureResponse;
        const auto transactionID = stringProperty (request, "transaction_id");
        const auto found = transactions.find (transactionID.toStdString());
        if (transactionID.isEmpty() || found == transactions.end())
            return error ("unknown_transaction", "rollback requires a prior transaction_id");
        const auto& transaction = found->second;
        plugin->setStateInformation (transaction.state.getData(), static_cast<int> (transaction.state.getSize()));
        dispatchMessages();
        juce::Array<juce::var> mismatches;
        const auto matches = sameParameters (transaction.parameters, mismatches);
        auto result = makeObject();
        set (result, "transaction_id", transaction.id);
        set (result, "rollback_verified", matches);
        set (result, "parameter_mismatches", mismatches);
        set (result, "fresh_readback", snapshot());
        addLog ("rollback " + transaction.id + (matches ? " verified" : " mismatch"));
        return success (result);
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
        pluginFile = {};
        transactions.clear();
        states.clear();
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
