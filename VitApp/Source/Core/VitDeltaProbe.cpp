#include "VitDeltaProbe.h"

namespace vit
{

VitDeltaRingBuffer::VitDeltaRingBuffer()
    : fifo (capacity)
{
}

bool VitDeltaRingBuffer::tryPush (DeltaEvent&& event) noexcept
{
    int start1 = 0, size1 = 0, start2 = 0, size2 = 0;
    fifo.prepareToWrite (1, start1, size1, start2, size2);

    if (size1 <= 0 && size2 <= 0)
    {
        dropped_count.fetch_add (1, std::memory_order_relaxed);
        return false;
    }

    const int index = size1 > 0 ? start1 : start2;
    event.seq_id = ++next_seq_id;
    storage[(size_t) index] = std::move (event);
    fifo.finishedWrite (1);
    return true;
}

bool VitDeltaRingBuffer::tryPop (DeltaEvent& event) noexcept
{
    int start1 = 0, size1 = 0, start2 = 0, size2 = 0;
    fifo.prepareToRead (1, start1, size1, start2, size2);

    if (size1 <= 0 && size2 <= 0)
        return false;

    const int index = size1 > 0 ? start1 : start2;
    event = std::move (storage[(size_t) index]);
    fifo.finishedRead (1);
    return true;
}

int VitDeltaRingBuffer::getNumReady() const noexcept
{
    return fifo.getNumReady();
}

uint64_t VitDeltaRingBuffer::getDroppedCount() const noexcept
{
    return dropped_count.load (std::memory_order_relaxed);
}

void VitDeltaRingBuffer::reset() noexcept
{
    fifo.reset();
    next_seq_id.store (0, std::memory_order_relaxed);
    dropped_count.store (0, std::memory_order_relaxed);
}

//==============================================================================
VitValueTreeDeltaListener::VitValueTreeDeltaListener (VitDeltaRingBuffer& ringRef) noexcept
    : ring (std::addressof (ringRef))
{
}

void VitValueTreeDeltaListener::attach (const juce::ValueTree& root)
{
    detach();

    if (ring == nullptr || ! root.isValid())
        return;

    trackedRoot = root;
    trackedRoot.addListener (this);
}

void VitValueTreeDeltaListener::detach() noexcept
{
    if (trackedRoot.isValid())
        trackedRoot.removeListener (this);

    trackedRoot = {};
}

juce::String VitValueTreeDeltaListener::extractNodeUid (const juce::ValueTree& tree) noexcept
{
    static const juce::Identifier idProp ("id");
    static const juce::Identifier projectId ("projectID");

    if (tree.hasProperty (idProp))
        return tree[idProp].toString();

    if (tree.hasProperty (projectId))
        return tree[projectId].toString();

    return tree.getType().toString();
}

void VitValueTreeDeltaListener::valueTreePropertyChanged (juce::ValueTree& tree, const juce::Identifier& property)
{
    DeltaEvent ev;
    // action 携带属性名，value 仅为新值（避免在回调内分配复合 var）
    ev.action = juce::String ("property_changed:") + property.toString();
    ev.target_uid = extractNodeUid (tree);
    ev.value = tree[property];
    ev.timestamp = juce::Time::getMillisecondCounterHiRes() / 1000.0;
    juce::ignoreUnused (ring->tryPush (std::move (ev)));
}

void VitValueTreeDeltaListener::valueTreeChildAdded (juce::ValueTree&, juce::ValueTree& child)
{
    DeltaEvent ev;
    ev.action = "node_added";
    ev.target_uid = extractNodeUid (child);
    ev.value = child.getType().toString();
    ev.timestamp = juce::Time::getMillisecondCounterHiRes() / 1000.0;
    juce::ignoreUnused (ring->tryPush (std::move (ev)));
}

void VitValueTreeDeltaListener::valueTreeChildRemoved (juce::ValueTree&, juce::ValueTree& child, int)
{
    DeltaEvent ev;
    ev.action = "node_removed";
    ev.target_uid = extractNodeUid (child);
    ev.value = child.getType().toString();
    ev.timestamp = juce::Time::getMillisecondCounterHiRes() / 1000.0;
    juce::ignoreUnused (ring->tryPush (std::move (ev)));
}

//==============================================================================
VitEditStateDeltaHub::VitEditStateDeltaHub()
    : listener (buffer)
{
}

void VitEditStateDeltaHub::attach (const juce::ValueTree& editStateRoot)
{
    listener.attach (editStateRoot);
}

void VitEditStateDeltaHub::detach() noexcept
{
    listener.detach();
}

} // namespace vit
