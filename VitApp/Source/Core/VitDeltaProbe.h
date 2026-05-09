#pragma once

#include <JuceHeader.h>
#include <array>
#include <atomic>

namespace vit
{

/** 单条 ValueTree 增量事件（由监听线程入队，消费者线程出队处理）。 */
struct DeltaEvent
{
    uint32_t seq_id = 0;
    juce::String action;
    juce::String target_uid;
    juce::var value;
    double timestamp = 0.0;
};

//==============================================================================
/**
 * 基于 juce::AbstractFifo + 定长环形的 SPSC 无锁队列。
 * 生产者：ValueTree::Listener（本工程中即为消息线程）；消费者：任意单线程批量 drain。
 */
class VitDeltaRingBuffer final
{
public:
    static constexpr int capacity = 4096;

    VitDeltaRingBuffer();

    /** 写入一条事件；队列满时丢弃并返回 false（监听回调内禁止阻塞）。 */
    bool tryPush (DeltaEvent&& event) noexcept;

    /** 读取一条事件；空则返回 false。 */
    bool tryPop (DeltaEvent& event) noexcept;

    int getNumReady() const noexcept;
    uint64_t getDroppedCount() const noexcept;

    void reset() noexcept;

private:
    juce::AbstractFifo fifo;
    std::array<DeltaEvent, (size_t) capacity> storage {};
    std::atomic<uint32_t> next_seq_id { 0 };
    std::atomic<uint64_t> dropped_count { 0 };
};

//==============================================================================
/**
 * 挂载到工程根 ValueTree（如 Edit::state），仅将变动摘要压入环形队列。
 * 禁止在本类回调中做 JSON、网络或重计算。
 */
class VitValueTreeDeltaListener final : private juce::ValueTree::Listener
{
public:
    explicit VitValueTreeDeltaListener (VitDeltaRingBuffer& ringRef) noexcept;

    void attach (const juce::ValueTree& root);
    void detach() noexcept;

    bool isAttached() const noexcept { return trackedRoot.isValid(); }

private:
    void valueTreePropertyChanged (juce::ValueTree& tree, const juce::Identifier& property) override;
    void valueTreeChildAdded (juce::ValueTree& parent, juce::ValueTree& child) override;
    void valueTreeChildRemoved (juce::ValueTree& parent, juce::ValueTree& child, int) override;

    static juce::String extractNodeUid (const juce::ValueTree& tree) noexcept;

    VitDeltaRingBuffer* ring = nullptr;
    juce::ValueTree trackedRoot;
};

//==============================================================================
/** 队列 + 监听器组合，便于在服务对象内嵌套使用。 */
class VitEditStateDeltaHub final
{
public:
    VitEditStateDeltaHub();

    void attach (const juce::ValueTree& editStateRoot);
    void detach() noexcept;

    VitDeltaRingBuffer& getBuffer() noexcept { return buffer; }
    const VitDeltaRingBuffer& getBuffer() const noexcept { return buffer; }

private:
    VitDeltaRingBuffer buffer;
    VitValueTreeDeltaListener listener;
};

} // namespace vit
