#include "SendRouteService.h"

#include <set>

namespace vit
{

namespace
{

using namespace tracktion::engine;

// ---------------------------------------------------------------------------
// Property schema (design §1.2). Old projects without these attributes read
// back as ordinary tracks — fail-open by construction: every accessor treats
// a missing property as "not an FX track / no bus".
// ---------------------------------------------------------------------------

constexpr const char* kFxRoleProperty   = "vit_fx_role";
constexpr const char* kFxRoleValue      = "send_return_fx";
constexpr const char* kSendBusProperty  = "vit_send_bus";
constexpr const char* kReturnBusProperty = "vit_return_bus";

// Design §7.4: aux bus pair number range 0-63 → up to 32 FX tracks.
constexpr int kMaxBusPairs = 32;

// FX track → return bus at unity; the FX master feed is pulled to -96dB so
// the wet signal only reaches the mix through the return bus (裁定 1).
constexpr float kFxBusSendDb = 0.0f;
constexpr float kFxMasterKillerDb = -96.0f;

// Design §1.3: default source send amount.
constexpr float kDefaultAmountDb = -6.0f;

// Design §1.2: bus names are ≤20 chars, taken from the FX track name.
constexpr int kMaxBusNameChars = 20;

// ---------------------------------------------------------------------------
// Basic lookups
// ---------------------------------------------------------------------------

te::AudioTrack* findAudioTrackByID (te::Edit& edit, const juce::String& trackID)
{
    const auto trimmed = trackID.trim();

    for (auto* track : te::getAllTracks (edit))
        if (track != nullptr && track->itemID.toString() == trimmed)
            return dynamic_cast<te::AudioTrack*> (track);

    return nullptr;
}

te::AudioTrack* findAudioTrackByName (te::Edit& edit, const juce::String& name)
{
    for (auto* track : te::getAllTracks (edit))
        if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (track))
            if (audioTrack->getName() == name)
                return audioTrack;

    return nullptr;
}

bool hasFxRole (const te::AudioTrack& track)
{
    return track.state.getProperty (kFxRoleProperty).toString() == kFxRoleValue;
}

int fxSendBusOf (const te::AudioTrack& track)
{
    return (int) track.state.getProperty (kSendBusProperty, -1);
}

int fxReturnBusOf (const te::AudioTrack& track)
{
    return (int) track.state.getProperty (kReturnBusProperty, -1);
}

te::AudioTrack* findFxTrackBySendBus (te::Edit& edit, int bus)
{
    if (bus < 0)
        return nullptr;

    for (auto* track : te::getAllTracks (edit))
        if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (track))
            if (hasFxRole (*audioTrack) && fxSendBusOf (*audioTrack) == bus)
                return audioTrack;

    return nullptr;
}

te::AuxSendPlugin* findAuxSend (te::AudioTrack& track, int bus)
{
    if (bus < 0)
        return nullptr;

    for (auto* plugin : track.pluginList.getPlugins())
        if (auto* send = dynamic_cast<te::AuxSendPlugin*> (plugin))
            if (send->getBusNumber() == bus)
                return send;

    return nullptr;
}

te::AuxReturnPlugin* findAuxReturn (te::AudioTrack& track, int bus)
{
    if (bus < 0)
        return nullptr;

    for (auto* plugin : track.pluginList.getPlugins())
        if (auto* ret = dynamic_cast<te::AuxReturnPlugin*> (plugin))
            if (ret->busNumber == bus)
                return ret;

    return nullptr;
}

bool isMonitoringPlugin (const te::Plugin* plugin)
{
    return dynamic_cast<const te::VolumeAndPanPlugin*> (plugin) != nullptr
        || dynamic_cast<const te::LevelMeterPlugin*> (plugin) != nullptr;
}

int pluginIndexOf (te::Plugin& plugin, const te::AudioTrack& track)
{
    const auto plugins = track.pluginList.getPlugins();

    for (int i = 0; i < plugins.size(); ++i)
        if (plugins[i].get() == &plugin)
            return i;

    return -1;
}

/** Index of the first volume/meter plugin — the boundary between the user
 *  chain (content, sends) and the fixed monitoring tail. */
int monitoringInsertIndex (const te::AudioTrack& track)
{
    const auto plugins = track.pluginList.getPlugins();

    for (int i = 0; i < plugins.size(); ++i)
        if (plugins[i] != nullptr && isMonitoringPlugin (plugins[i].get()))
            return i;

    return plugins.size();
}

/** Default source-pair position (§9.0-5: 成对加载默认链尾): after all content
 *  plugins, before the volume/meter tail — SendOut lands there and SendIn
 *  immediately after it, so the pair is emitted ordered by construction. */
int sourcePairInsertIndex (const te::AudioTrack& track)
{
    return monitoringInsertIndex (track);
}

void movePluginToIndex (te::Plugin& plugin, te::AudioTrack& track, int newIndex, juce::UndoManager& um)
{
    const auto plugins = track.pluginList.getPlugins();

    if (newIndex < 0 || newIndex >= plugins.size())
        return;

    const auto currentIndex = pluginIndexOf (plugin, track);

    if (currentIndex < 0 || currentIndex == newIndex)
        return;

    auto parent = plugin.state.getParent();

    if (! parent.isValid())
        return;

    const auto stateIndex = parent.indexOf (plugin.state);

    if (stateIndex < 0)
        return;

    // Convert list index to a state-child index by anchoring on the plugin
    // that will follow the moved one (same mapping PluginList::insertPlugin
    // uses); the plugin object follows its state.
    int targetStateIndex;

    if (newIndex >= plugins.size() - 1 || plugins[newIndex].get() == &plugin)
        targetStateIndex = parent.getNumChildren();
    else
        targetStateIndex = parent.indexOf (plugins[newIndex]->state);

    if (targetStateIndex > stateIndex)
        --targetStateIndex;

    if (targetStateIndex != stateIndex)
        parent.moveChild (stateIndex, targetStateIndex, &um);
}

// ---------------------------------------------------------------------------
// Bus pair allocation
// ---------------------------------------------------------------------------

/** Every aux bus number in live use: FX role properties plus the busNum of
 *  every aux plugin in the edit. Dangling SendOuts (their FX track deleted)
 *  keep their number reserved so a recycled pair can never silently re-route
 *  them onto a new FX track. */
std::set<int> occupiedBusNumbers (te::Edit& edit)
{
    std::set<int> occupied;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        if (hasFxRole (*audioTrack))
        {
            const int sendBus = fxSendBusOf (*audioTrack);
            const int returnBus = fxReturnBusOf (*audioTrack);

            if (sendBus >= 0)
                occupied.insert (sendBus);

            if (returnBus >= 0)
                occupied.insert (returnBus);
        }

        for (auto* plugin : audioTrack->pluginList.getPlugins())
        {
            if (auto* send = dynamic_cast<te::AuxSendPlugin*> (plugin))
                occupied.insert (send->getBusNumber());
            else if (auto* ret = dynamic_cast<te::AuxReturnPlugin*> (plugin))
                occupied.insert ((int) ret->busNumber);
        }
    }

    return occupied;
}

/** Smallest free even/odd pair, or {-1,-1} when the range is exhausted. */
std::pair<int, int> allocateBusPair (te::Edit& edit)
{
    const auto occupied = occupiedBusNumbers (edit);

    for (int k = 0; k < kMaxBusPairs; ++k)
    {
        const int sendBus = 2 * k;
        const int returnBus = 2 * k + 1;

        if (occupied.count (sendBus) == 0 && occupied.count (returnBus) == 0)
            return { sendBus, returnBus };
    }

    return { -1, -1 };
}

bool anyTrackHasAuxSend (te::Edit& edit, int bus)
{
    for (auto* track : te::getAllTracks (edit))
        if (auto* audioTrack = dynamic_cast<te::AudioTrack*> (track))
            if (findAuxSend (*audioTrack, bus) != nullptr)
                return true;

    return false;
}

// ---------------------------------------------------------------------------
// Topology (anti-loop gate 1)
// ---------------------------------------------------------------------------

struct SendEdgeSnapshot
{
    te::AudioTrack* source = nullptr;
    te::AudioTrack* fx = nullptr;
    float amountDb = 0.0f;
};

/** Edges derived from live state: a SendOut on track T with busNum B is the
 *  edge T → findFxTrackBySendBus(B). This is the same derivation
 *  track.send.list reports, so validation and disclosure can't diverge. */
juce::Array<SendEdgeSnapshot> collectSendEdges (te::Edit& edit)
{
    juce::Array<SendEdgeSnapshot> edges;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr || hasFxRole (*audioTrack))
            continue;

        for (auto* plugin : audioTrack->pluginList.getPlugins())
        {
            auto* send = dynamic_cast<te::AuxSendPlugin*> (plugin);

            if (send == nullptr)
                continue;

            if (auto* fx = findFxTrackBySendBus (edit, send->getBusNumber()))
            {
                SendEdgeSnapshot edge;
                edge.source = audioTrack;
                edge.fx = fx;
                edge.amountDb = send->getGainDb();
                edges.add (edge);
            }
        }
    }

    return edges;
}

/** Would adding source→fx close a cycle? (Reachability from fx back to source
 *  over existing edges — VitDagChecker's idea, lifted from rack nodes to
 *  inter-track edges. Gate A normally keeps FX tracks source-free, so this is
 *  the defensive second line for hand-mutated state.) */
bool wouldCreateCycle (te::Edit& edit, te::AudioTrack& source, te::AudioTrack& fx)
{
    const auto edges = collectSendEdges (edit);
    std::set<te::AudioTrack*> visited;
    juce::Array<te::AudioTrack*> frontier { &fx };

    while (! frontier.isEmpty())
    {
        auto* current = frontier.removeAndReturn (0);

        if (current == &source)
            return true;

        if (! visited.insert (current).second)
            continue;

        for (const auto& edge : edges)
            if (edge.source == current)
                frontier.add (edge.fx);
    }

    return false;
}

// ---------------------------------------------------------------------------
// FX-track lifecycle
// ---------------------------------------------------------------------------

void setBusNames (te::Edit& edit, const te::AudioTrack& fx, int sendBus, int returnBus)
{
    const auto baseName = fx.getName().substring (0, kMaxBusNameChars);
    edit.setAuxBusName (sendBus, baseName);
    edit.setAuxBusName (returnBus, (baseName + " Ret").substring (0, kMaxBusNameChars));
}

void ensureMonitoringPlugins (te::AudioTrack& track)
{
    auto& edit = track.pluginList.getEdit();

    if (track.getVolumePlugin() == nullptr)
        if (auto plugin = edit.getPluginCache().createNewPlugin (te::VolumeAndPanPlugin::xmlTypeName, {}))
            track.pluginList.insertPlugin (plugin, -1, nullptr);

    if (track.getLevelMeterPlugin() == nullptr)
        if (auto plugin = edit.getPluginCache().createNewPlugin (te::LevelMeterPlugin::xmlTypeName, {}))
            track.pluginList.insertPlugin (plugin, -1, nullptr);
}

/** Anti-loop gate 2 (FX side): AuxReturn(send_bus) at the head, SendOut
 *  (return_bus) after any RackInstance but before the volume plugin. Called
 *  after every mutation that touches the FX track and from the dispatcher's
 *  post-rack-mutation hook, because rack_add_node inserts a fresh
 *  RackInstance at the first monitoring index which can land after the send.
 */
void normaliseFxChain (te::AudioTrack& fx, juce::UndoManager& um)
{
    if (! hasFxRole (fx))
        return;

    const int sendBus = fxSendBusOf (fx);
    const int returnBus = fxReturnBusOf (fx);

    if (sendBus < 0 || returnBus < 0)
        return;

    const auto plugins = fx.pluginList.getPlugins();

    int lastRackIndex = -1;

    for (int i = 0; i < plugins.size(); ++i)
        if (dynamic_cast<te::RackInstance*> (plugins[i].get()) != nullptr)
            lastRackIndex = i;

    if (auto* receiver = findAuxReturn (fx, sendBus))
    {
        const auto currentIndex = pluginIndexOf (*receiver, fx);

        if (currentIndex > 0)
            movePluginToIndex (*receiver, fx, 0, um);
    }

    if (auto* wetSend = findAuxSend (fx, returnBus))
    {
        const auto currentIndex = pluginIndexOf (*wetSend, fx);

        // After the last rack (post-effects wet), else before the monitoring
        // tail; always strictly after the dry receiver.
        int targetIndex = lastRackIndex >= 0 ? lastRackIndex + 1 : monitoringInsertIndex (fx);

        if (auto* receiver = findAuxReturn (fx, sendBus))
            targetIndex = juce::jmax (targetIndex, pluginIndexOf (*receiver, fx) + 1);

        if (currentIndex >= 0 && targetIndex < currentIndex)
            movePluginToIndex (*wetSend, fx, targetIndex, um);
    }

    if (auto* volume = fx.getVolumePlugin())
        volume->setVolumeDb (kFxMasterKillerDb);
}

/** Bring a plain track up to FX-role readiness: allocate the bus pair,
 *  install the FX-side aux pair, pull the volume to -96dB, annotate the
 *  state and name the buses. Idempotent — an already-initialised track only
 *  gets its chain re-normalised. Empty on success, else the error message. */
juce::String initialiseFxTrack (te::AudioTrack& fx, juce::UndoManager& um)
{
    if (! hasFxRole (fx))
    {
        const auto [sendBus, returnBus] = allocateBusPair (fx.pluginList.getEdit());

        if (sendBus < 0)
            return "No free aux bus pair (all " + juce::String (kMaxBusPairs) + " pairs in use)";

        fx.state.setProperty (kFxRoleProperty, kFxRoleValue, &um);
        fx.state.setProperty (kSendBusProperty, sendBus, &um);
        fx.state.setProperty (kReturnBusProperty, returnBus, &um);
    }

    const int sendBus = fxSendBusOf (fx);
    const int returnBus = fxReturnBusOf (fx);

    ensureMonitoringPlugins (fx);

    if (findAuxReturn (fx, sendBus) == nullptr)
    {
        auto receiver = fx.pluginList.getEdit().getPluginCache().createNewPlugin (te::AuxReturnPlugin::xmlTypeName, {});

        if (receiver == nullptr)
            return "Failed to create AuxReturn plugin";

        receiver->state.setProperty ("busNum", sendBus, &um);
        fx.pluginList.insertPlugin (receiver, 0, nullptr);
    }

    if (findAuxSend (fx, returnBus) == nullptr)
    {
        auto wetSend = fx.pluginList.getEdit().getPluginCache().createNewPlugin (te::AuxSendPlugin::xmlTypeName, {});

        if (wetSend == nullptr)
            return "Failed to create AuxSend plugin";

        wetSend->state.setProperty ("busNum", returnBus, &um);
        fx.pluginList.insertPlugin (wetSend, monitoringInsertIndex (fx), nullptr);
        dynamic_cast<te::AuxSendPlugin*> (wetSend.get())->setGainDb (kFxBusSendDb);
    }

    normaliseFxChain (fx, um);
    setBusNames (fx.pluginList.getEdit(), fx, sendBus, returnBus);
    fx.flushStateToValueTree();
    return {};
}

/** Release the FX role: remove the FX-side aux pair, restore the volume to
 *  0dB (leaving a track at -96dB would be a silent-track trap) and clear the
 *  annotations. The bus pair is implicitly recycled — allocation only scans
 *  live state. */
void dismantleFxTrack (te::AudioTrack& fx, juce::UndoManager& um)
{
    const int sendBus = fxSendBusOf (fx);
    const int returnBus = fxReturnBusOf (fx);

    if (auto* receiver = findAuxReturn (fx, sendBus))
        receiver->removeFromParent();

    if (auto* wetSend = findAuxSend (fx, returnBus))
        wetSend->removeFromParent();

    if (auto* volume = fx.getVolumePlugin())
        volume->setVolumeDb (0.0f);

    fx.state.removeProperty (kFxRoleProperty, &um);
    fx.state.removeProperty (kSendBusProperty, &um);
    fx.state.removeProperty (kReturnBusProperty, &um);
    fx.flushStateToValueTree();
}

// ---------------------------------------------------------------------------
// Source-side pair
// ---------------------------------------------------------------------------

/** Install (or retune) the source-side pair for one FX connection. Existing
 *  SendOut only gets its amount updated (idempotent reconnect, §1.3); a
 *  missing SendIn is re-inserted immediately after its SendOut so the pair is
 *  always emitted ordered (gate 2). */
void ensureSourceSendPair (te::AudioTrack& source, te::AudioTrack& fx, float amountDb, juce::UndoManager& um)
{
    const int sendBus = fxSendBusOf (fx);
    const int returnBus = fxReturnBusOf (fx);

    auto* sendOut = findAuxSend (source, sendBus);

    if (sendOut == nullptr)
    {
        auto plugin = source.pluginList.getEdit().getPluginCache().createNewPlugin (te::AuxSendPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            plugin->state.setProperty ("busNum", sendBus, &um);
            source.pluginList.insertPlugin (plugin, sourcePairInsertIndex (source), nullptr);
            dynamic_cast<te::AuxSendPlugin*> (plugin.get())->setGainDb (amountDb);
        }

        sendOut = findAuxSend (source, sendBus);
    }
    else
    {
        sendOut->setGainDb (amountDb);
    }

    if (findAuxReturn (source, returnBus) == nullptr && sendOut != nullptr)
    {
        auto plugin = source.pluginList.getEdit().getPluginCache().createNewPlugin (te::AuxReturnPlugin::xmlTypeName, {});

        if (plugin != nullptr)
        {
            plugin->state.setProperty ("busNum", returnBus, &um);
            const auto afterSend = pluginIndexOf (*sendOut, source) + 1;
            source.pluginList.insertPlugin (plugin, afterSend, nullptr);
        }
    }
    source.flushStateToValueTree();
}

void removeSourceSendPair (te::AudioTrack& source, te::AudioTrack& fx)
{
    if (auto* sendOut = findAuxSend (source, fxSendBusOf (fx)))
        sendOut->removeFromParent();

    if (auto* sendIn = findAuxReturn (source, fxReturnBusOf (fx)))
        sendIn->removeFromParent();

    source.flushStateToValueTree();
}

// ---------------------------------------------------------------------------
// Read-back (the single read source for the observation plane)
// ---------------------------------------------------------------------------

juce::var makeSendRow (te::AudioTrack& source, te::AuxSendPlugin& sendOut, te::AudioTrack* fx)
{
    auto* row = new juce::DynamicObject();
    row->setProperty ("source_track_id", source.itemID.toString());
    row->setProperty ("source_name", source.getName());
    row->setProperty ("fx_track_id", fx != nullptr ? juce::var (fx->itemID.toString()) : juce::var());
    row->setProperty ("fx_name", fx != nullptr ? juce::var (fx->getName()) : juce::var());
    row->setProperty ("amount_db", sendOut.getGainDb());
    row->setProperty ("send_bus", sendOut.getBusNumber());
    row->setProperty ("return_bus", fx != nullptr ? juce::var (fxReturnBusOf (*fx)) : juce::var (-1));
    return juce::var (row);
}

juce::var makeFxTrackRow (te::AudioTrack& fx, int sourceCount)
{
    auto* row = new juce::DynamicObject();
    row->setProperty ("track_id", fx.itemID.toString());
    row->setProperty ("name", fx.getName());
    row->setProperty ("send_bus", fxSendBusOf (fx));
    row->setProperty ("return_bus", fxReturnBusOf (fx));
    row->setProperty ("source_count", sourceCount);
    return juce::var (row);
}

/** Sidechain (control-send, §9.0-2) read-back: every plugin in the edit with
 *  a live sidechainSourceID, resolved to its owning track. */
juce::Array<juce::var> collectSidechainRows (te::Edit& edit)
{
    juce::Array<juce::var> rows;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        for (auto* plugin : audioTrack->pluginList.getPlugins())
        {
            if (plugin == nullptr)
                continue;

            const auto sourceId = plugin->getSidechainSourceID();

            if (! sourceId.isValid())
                continue;

            auto* row = new juce::DynamicObject();
            row->setProperty ("source_track_id", sourceId.toString());
            row->setProperty ("track_id", audioTrack->itemID.toString());
            row->setProperty ("track_name", audioTrack->getName());
            row->setProperty ("plugin_item_id", plugin->itemID.toString());
            row->setProperty ("plugin_name", plugin->getName());
            rows.add (juce::var (row));
        }
    }

    return rows;
}

void appendTopologySnapshot (juce::DynamicObject& reply, te::Edit& edit, te::AudioTrack* filterFx)
{
    juce::Array<juce::var> sends;
    juce::Array<juce::var> dangling;
    juce::Array<juce::var> fxTracks;
    juce::Array<juce::var> sidechain = collectSidechainRows (edit);

    for (auto* track : te::getAllTracks (edit))
    {
        auto* fx = dynamic_cast<te::AudioTrack*> (track);

        if (fx != nullptr && hasFxRole (*fx))
        {
            int sourceCount = 0;

            for (const auto& edge : collectSendEdges (edit))
                if (edge.fx == fx)
                    ++sourceCount;

            if (filterFx == nullptr || filterFx == fx)
                fxTracks.add (makeFxTrackRow (*fx, sourceCount));
        }
    }

    for (auto* track : te::getAllTracks (edit))
    {
        auto* source = dynamic_cast<te::AudioTrack*> (track);

        if (source == nullptr || hasFxRole (*source))
            continue;

        for (auto* plugin : source->pluginList.getPlugins())
        {
            auto* sendOut = dynamic_cast<te::AuxSendPlugin*> (plugin);

            if (sendOut == nullptr)
                continue;

            auto* fx = findFxTrackBySendBus (edit, sendOut->getBusNumber());

            if (fx == nullptr)
            {
                dangling.add (makeSendRow (*source, *sendOut, nullptr));
                continue;
            }

            if (filterFx == nullptr || filterFx == fx)
                sends.add (makeSendRow (*source, *sendOut, fx));
        }
    }

    reply.setProperty ("sends", sends);
    reply.setProperty ("dangling", dangling);
    reply.setProperty ("fx_tracks", fxTracks);
    reply.setProperty ("sidechain", sidechain);
}

// ---------------------------------------------------------------------------
// Shared mutation plumbing
// ---------------------------------------------------------------------------

void finishRenderInteraction (te::Edit& edit)
{
    edit.invalidateStoredLength();
    edit.dispatchPendingUpdatesSynchronously();
    edit.getTransport().ensureContextAllocated (true);
}

bool parseAmountDb (const juce::DynamicObject& object, float& amountOut)
{
    const auto& value = object.getProperty ("amount_db");

    if (value.isDouble() || value.isInt() || value.isInt64())
    {
        amountOut = (float) (double) value;
        return true;
    }

    amountOut = kDefaultAmountDb;
    return false;
}

juce::String uniqueFxTrackName (te::Edit& edit, const juce::String& base)
{
    if (findAudioTrackByName (edit, base) == nullptr)
        return base;

    for (int suffix = 2; suffix < 1000; ++suffix)
    {
        const auto candidate = base + " " + juce::String (suffix);

        if (findAudioTrackByName (edit, candidate) == nullptr)
            return candidate;
    }

    return base + " " + juce::String (juce::Random::getSystemRandom().nextInt (10000));
}

te::Plugin* findPluginByIdInEdit (te::Edit& edit, const juce::String& pluginItemId)
{
    const auto wanted = te::EditItemID::fromString (pluginItemId.trim());

    for (auto* track : te::getAllTracks (edit))
    {
        auto* audioTrack = dynamic_cast<te::AudioTrack*> (track);

        if (audioTrack == nullptr)
            continue;

        for (auto* plugin : audioTrack->pluginList.getPlugins())
        {
            if (plugin == nullptr)
                continue;

            if (plugin->itemID == wanted)
                return plugin;

            if (auto* rack = dynamic_cast<te::RackInstance*> (plugin))
                if (rack->type != nullptr)
                    for (auto* inner : rack->type->getPlugins())
                        if (inner != nullptr && inner->itemID == wanted)
                            return inner;
        }
    }

    return nullptr;
}

} // namespace

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

SendRouteService::SendRouteService (EditGetter editGetter, SaveProjectAction saveProjectAction)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction))
{
}

SendRouteService::~SendRouteService() = default;

juce::String SendRouteService::makeErrorReply (const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "error");
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String SendRouteService::handleConnect (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    // Control send (sidechain) variant: native Plugin::setSidechainSourceByName,
    // no amount, no wet injection (§9.0-2 — tap is fixed post-chain by the engine).
    if (auto* target = object.getProperty ("target").getDynamicObject())
    {
        const auto kind = target->getProperty ("kind").toString().trim();

        if (! kind.equalsIgnoreCase ("sidechain"))
            return makeErrorReply ("track.send.connect target.kind must be 'sidechain' (got: " + kind + ")");

        const auto sourceTrackID = object.getProperty ("source_track_id").toString().trim();

        if (sourceTrackID.isEmpty())
            return makeErrorReply ("track.send.connect requires source_track_id");

        auto* source = findAudioTrackByID (*edit, sourceTrackID);

        if (source == nullptr)
            return makeErrorReply ("Source track not found for source_track_id: " + sourceTrackID);

        const auto pluginItemId = target->getProperty ("plugin_item_id").toString().trim();

        if (pluginItemId.isEmpty())
            return makeErrorReply ("sidechain connect requires target.plugin_item_id");

        auto* plugin = findPluginByIdInEdit (*edit, pluginItemId);

        if (plugin == nullptr)
            return makeErrorReply ("Plugin not found for plugin_item_id: " + pluginItemId);

        auto& undo = edit->getUndoManager();
        undo.beginNewTransaction ("Connect sidechain send");

        // Native sidechainSourceID face, addressed by ID: the engine's own
        // setSidechainSourceByName matches a "%d. name" prefixed display form
        // and silently resets on a bare track name, so the command face sets
        // the same native state directly (rename-robust, same persistence).
        plugin->setSidechainSourceID (source->itemID);

        if (plugin->getNumWires() == 0)
            plugin->guessSidechainRouting();

        finishRenderInteraction (*edit);

        if (saveProject && ! saveProject())
            return makeErrorReply ("Sidechain source set in memory but failed to save project");

        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "ok");
        response->setProperty ("message", "Sidechain send connected");
        response->setProperty ("source_track_id", source->itemID.toString());
        response->setProperty ("source_name", source->getName());
        response->setProperty ("plugin_item_id", plugin->itemID.toString());
        response->setProperty ("plugin_name", plugin->getName());
        response->setProperty ("sidechain_source_id", plugin->getSidechainSourceID().toString());
        response->setProperty ("kind", "sidechain");
        response->setProperty ("sidechain", collectSidechainRows (*edit));
        return juce::JSON::toString (juce::var (response.release()));
    }

    const auto sourceTrackID = object.getProperty ("source_track_id").toString().trim();

    if (sourceTrackID.isEmpty())
        return makeErrorReply ("track.send.connect requires source_track_id");

    auto* source = findAudioTrackByID (*edit, sourceTrackID);

    if (source == nullptr)
        return makeErrorReply ("Source track not found for source_track_id: " + sourceTrackID);

    if (hasFxRole (*source))
        return makeErrorReply ("FX track " + source->getName() + " cannot be a send source (anti-loop gate 1)");

    const auto fxTrackID = object.getProperty ("fx_track_id").toString().trim();
    float amountDb = kDefaultAmountDb;
    parseAmountDb (object, amountDb);

    const auto& autoCreateVar = object.getProperty ("auto_create_fx_track");
    juce::String autoCreateName;

    if (autoCreateVar.isString())
        autoCreateName = autoCreateVar.toString().trim();
    else if (auto* autoCreate = autoCreateVar.getDynamicObject())
        autoCreateName = autoCreate->getProperty ("name").toString().trim();

    auto& undo = edit->getUndoManager();
    te::AudioTrack* fx = nullptr;
    bool createdFxTrack = false;
    bool transactionOpen = false;

    // Exactly one undo transaction per command: for auto_create it covers
    // add_track + FX-side init + edge wiring, so a single undo reverts the
    // whole creation (§9.2 undo atomicity).
    auto beginTransaction = [&] (const char* name)
    {
        if (! transactionOpen)
        {
            undo.beginNewTransaction (name);
            transactionOpen = true;
        }
    };

    if (autoCreateName.isNotEmpty())
    {
        if (fxTrackID.isNotEmpty())
            return makeErrorReply ("track.send.connect takes either fx_track_id or auto_create_fx_track, not both");

        // Idempotency anchor (§9.2): an existing FX track with the requested
        // name is reused — a resent identical command never builds a second
        // track. Any other name collision gets a sequence suffix.
        if (auto* existing = findAudioTrackByName (*edit, autoCreateName); existing != nullptr && hasFxRole (*existing))
        {
            fx = existing;
        }
        else
        {
            beginTransaction ("Create send FX track");
            fx = edit->insertNewAudioTrack (te::TrackInsertPoint (source->getParentFolderTrack(), source),
                                            nullptr, true).get();

            if (fx == nullptr)
                return makeErrorReply ("Failed to insert new FX track");

            fx->setName (uniqueFxTrackName (*edit, autoCreateName));
            createdFxTrack = true;
        }
    }
    else
    {
        if (fxTrackID.isEmpty())
            return makeErrorReply ("track.send.connect requires fx_track_id or auto_create_fx_track");

        fx = findAudioTrackByID (*edit, fxTrackID);

        if (fx == nullptr)
            return makeErrorReply ("FX track not found for fx_track_id: " + fxTrackID);
    }

    if (fx == source)
        return makeErrorReply ("A track cannot send to itself");

    // A track that is itself a send source cannot become the FX target — the
    // anti-loop invariant keeps FX tracks strictly out of the source role.
    if (! hasFxRole (*fx))
        for (auto* plugin : fx->pluginList.getPlugins())
            if (dynamic_cast<te::AuxSendPlugin*> (plugin) != nullptr)
                return makeErrorReply ("Target track " + fx->getName()
                                       + " is itself a send source and cannot become an FX track");

    if (wouldCreateCycle (*edit, *source, *fx))
        return makeErrorReply ("Connect would create a send topology cycle (anti-loop gate 1)");

    beginTransaction (createdFxTrack ? "Create send FX track" : "Connect send route");

    if (const auto error = initialiseFxTrack (*fx, undo); error.isNotEmpty())
        return makeErrorReply (error);

    const bool alreadyConnected = findAuxSend (*source, fxSendBusOf (*fx)) != nullptr;
    ensureSourceSendPair (*source, *fx, amountDb, undo);
    normaliseFxChain (*fx, undo);
    finishRenderInteraction (*edit);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Send route connected in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", alreadyConnected ? "Send amount updated (connection already present)"
                                                       : (createdFxTrack ? "Send route connected with new FX track"
                                                                         : "Send route connected"));
    response->setProperty ("source_track_id", source->itemID.toString());
    response->setProperty ("source_name", source->getName());
    response->setProperty ("fx_track_id", fx->itemID.toString());
    response->setProperty ("fx_name", fx->getName());
    response->setProperty ("amount_db", findAuxSend (*source, fxSendBusOf (*fx)) != nullptr
                                            ? juce::var (findAuxSend (*source, fxSendBusOf (*fx))->getGainDb())
                                            : juce::var (amountDb));
    response->setProperty ("send_bus", fxSendBusOf (*fx));
    response->setProperty ("return_bus", fxReturnBusOf (*fx));
    response->setProperty ("created_fx_track", createdFxTrack);
    appendTopologySnapshot (*response, *edit, fx);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String SendRouteService::handleSetAmount (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto sourceTrackID = object.getProperty ("source_track_id").toString().trim();
    const auto fxTrackID = object.getProperty ("fx_track_id").toString().trim();

    if (sourceTrackID.isEmpty() || fxTrackID.isEmpty())
        return makeErrorReply ("track.send.set_amount requires source_track_id and fx_track_id");

    float amountDb = kDefaultAmountDb;

    if (! parseAmountDb (object, amountDb))
        return makeErrorReply ("track.send.set_amount requires a numeric amount_db");

    auto* source = findAudioTrackByID (*edit, sourceTrackID);

    if (source == nullptr)
        return makeErrorReply ("Source track not found for source_track_id: " + sourceTrackID);

    auto* fx = findAudioTrackByID (*edit, fxTrackID);

    if (fx == nullptr)
        return makeErrorReply ("FX track not found for fx_track_id: " + fxTrackID);

    auto* sendOut = findAuxSend (*source, fxSendBusOf (*fx));

    if (sendOut == nullptr)
        return makeErrorReply ("No send connection from " + source->getName() + " to " + fx->getName());

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Set send amount");
    sendOut->setGainDb (amountDb);
    finishRenderInteraction (*edit);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Send amount set in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Send amount updated");
    response->setProperty ("source_track_id", source->itemID.toString());
    response->setProperty ("fx_track_id", fx->itemID.toString());
    response->setProperty ("amount_db", sendOut->getGainDb());
    response->setProperty ("send_bus", sendOut->getBusNumber());
    appendTopologySnapshot (*response, *edit, fx);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String SendRouteService::handleDisconnect (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    // Sidechain disconnect: reset the plugin's native sidechain source.
    if (auto* target = object.getProperty ("target").getDynamicObject())
    {
        const auto kind = target->getProperty ("kind").toString().trim();

        if (! kind.equalsIgnoreCase ("sidechain"))
            return makeErrorReply ("track.send.disconnect target.kind must be 'sidechain'");

        const auto pluginItemId = target->getProperty ("plugin_item_id").toString().trim();

        if (pluginItemId.isEmpty())
            return makeErrorReply ("sidechain disconnect requires target.plugin_item_id");

        auto* plugin = findPluginByIdInEdit (*edit, pluginItemId);

        if (plugin == nullptr)
            return makeErrorReply ("Plugin not found for plugin_item_id: " + pluginItemId);

        auto& undo = edit->getUndoManager();
        undo.beginNewTransaction ("Disconnect sidechain send");
        plugin->setSidechainSourceID (te::EditItemID());
        finishRenderInteraction (*edit);

        if (saveProject && ! saveProject())
            return makeErrorReply ("Sidechain source cleared in memory but failed to save project");

        auto response = std::make_unique<juce::DynamicObject>();
        response->setProperty ("status", "ok");
        response->setProperty ("message", "Sidechain send disconnected");
        response->setProperty ("plugin_item_id", plugin->itemID.toString());
        response->setProperty ("sidechain", collectSidechainRows (*edit));
        return juce::JSON::toString (juce::var (response.release()));
    }

    const auto sourceTrackID = object.getProperty ("source_track_id").toString().trim();
    const auto fxTrackID = object.getProperty ("fx_track_id").toString().trim();

    if (sourceTrackID.isEmpty() || fxTrackID.isEmpty())
        return makeErrorReply ("track.send.disconnect requires source_track_id and fx_track_id");

    auto* source = findAudioTrackByID (*edit, sourceTrackID);

    if (source == nullptr)
        return makeErrorReply ("Source track not found for source_track_id: " + sourceTrackID);

    auto* fx = findAudioTrackByID (*edit, fxTrackID);

    if (fx == nullptr)
        return makeErrorReply ("FX track not found for fx_track_id: " + fxTrackID);

    if (! hasFxRole (*fx))
        return makeErrorReply ("Track " + fx->getName() + " has no send_return_fx role");

    const bool hadConnection = findAuxSend (*source, fxSendBusOf (*fx)) != nullptr
                            || findAuxReturn (*source, fxReturnBusOf (*fx)) != nullptr;

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Disconnect send route");
    removeSourceSendPair (*source, *fx);

    // Zero sources left → release the FX side and recycle the bus pair.
    bool dismantled = false;

    if (! anyTrackHasAuxSend (*edit, fxSendBusOf (*fx)))
    {
        dismantleFxTrack (*fx, undo);
        dismantled = true;
    }

    finishRenderInteraction (*edit);

    if (saveProject && ! saveProject())
        return makeErrorReply ("Send route disconnected in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", ! hadConnection ? "Send connection already absent"
                                                      : (dismantled ? "Send disconnected; FX track released (bus pair recycled)"
                                                                    : "Send disconnected"));
    response->setProperty ("source_track_id", source->itemID.toString());
    response->setProperty ("fx_track_id", fx->itemID.toString());
    response->setProperty ("fx_released", dismantled);
    appendTopologySnapshot (*response, *edit, nullptr);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String SendRouteService::handleList (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    te::AudioTrack* filterFx = nullptr;
    const auto fxTrackID = object.getProperty ("fx_track_id").toString().trim();

    if (fxTrackID.isNotEmpty())
    {
        filterFx = findAudioTrackByID (*edit, fxTrackID);

        if (filterFx == nullptr)
            return makeErrorReply ("FX track not found for fx_track_id: " + fxTrackID);
    }

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Send topology read-back");
    appendTopologySnapshot (*response, *edit, filterFx);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String SendRouteService::handleSetFxRole (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto trackID = object.getProperty ("track_id").toString().trim();
    const auto role = object.getProperty ("role").toString().trim();

    if (trackID.isEmpty())
        return makeErrorReply ("track.send.set_fx_role requires track_id");

    if (role != kFxRoleValue && role != "none")
        return makeErrorReply ("track.send.set_fx_role role must be 'send_return_fx' or 'none'");

    auto* track = findAudioTrackByID (*edit, trackID);

    if (track == nullptr)
        return makeErrorReply ("Track not found for track_id: " + trackID);

    auto& undo = edit->getUndoManager();

    if (role == kFxRoleValue)
    {
        for (auto* plugin : track->pluginList.getPlugins())
            if (dynamic_cast<te::AuxSendPlugin*> (plugin) != nullptr && ! hasFxRole (*track))
                return makeErrorReply ("Track " + track->getName() + " is itself a send source and cannot take the FX role");

        if (! hasFxRole (*track))
            undo.beginNewTransaction ("Set FX track role");

        if (const auto error = initialiseFxTrack (*track, undo); error.isNotEmpty())
            return makeErrorReply (error);
    }
    else if (hasFxRole (*track))
    {
        undo.beginNewTransaction ("Clear FX track role");
        dismantleFxTrack (*track, undo);
    }

    finishRenderInteraction (*edit);

    if (saveProject && ! saveProject())
        return makeErrorReply ("FX role changed in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", hasFxRole (*track) ? "FX track role set"
                                                         : (role == "none" ? "FX track role cleared" : "FX track role unchanged"));
    response->setProperty ("track_id", track->itemID.toString());
    response->setProperty ("role", hasFxRole (*track) ? kFxRoleValue : "none");

    if (hasFxRole (*track))
    {
        response->setProperty ("send_bus", fxSendBusOf (*track));
        response->setProperty ("return_bus", fxReturnBusOf (*track));
    }

    appendTopologySnapshot (*response, *edit, nullptr);
    return juce::JSON::toString (juce::var (response.release()));
}

bool SendRouteService::normaliseFxChainsForEdit (te::Edit& edit) const
{
    bool changed = false;

    for (auto* track : te::getAllTracks (edit))
    {
        auto* fx = dynamic_cast<te::AudioTrack*> (track);

        if (fx == nullptr || ! hasFxRole (*fx))
            continue;

        const auto pluginsBefore = fx->pluginList.getPlugins();
        juce::StringArray orderBefore;

        for (auto* plugin : pluginsBefore)
            orderBefore.add (plugin != nullptr ? plugin->getName() : juce::String());

        normaliseFxChain (*fx, edit.getUndoManager());

        const auto pluginsAfter = fx->pluginList.getPlugins();
        bool reordered = pluginsBefore.size() != pluginsAfter.size();

        if (! reordered)
            for (int i = 0; i < pluginsBefore.size(); ++i)
                if (pluginsBefore[i] != pluginsAfter[i])
                    reordered = true;

        if (reordered)
        {
            changed = true;
            juce::Logger::writeToLog ("SendRouteService: re-normalised FX chain on track " + fx->itemID.toString());
        }
    }

    if (changed)
        finishRenderInteraction (edit);

    return changed;
}

} // namespace vit
