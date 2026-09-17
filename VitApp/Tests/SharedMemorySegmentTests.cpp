// PORT-A1: publisher-side shared-memory segment interface tests — the
// two-platform published-name policy and the create/write/unmap/release
// lifecycle, exercised through ISharedMemorySegment on Windows and POSIX.

#include "../Source/Service/SharedMemorySegment.h"

#include <cassert>
#include <cstddef>
#include <string>

namespace
{

bool isLowerHex (const std::string& text)
{
    if (text.empty())
        return false;
    for (const auto c : text)
    {
        const bool digit = c >= '0' && c <= '9';
        const bool lower = c >= 'a' && c <= 'f';
        if (! digit && ! lower)
            return false;
    }
    return true;
}

void exerciseSegmentLifecycle (const std::string& descriptiveName, size_t floatCount)
{
    std::string errorDetail;
    const auto byteCount = floatCount * sizeof (float);
    auto segment = vit::ISharedMemorySegment::createAndMap (descriptiveName, byteCount, errorDetail);
    assert (segment != nullptr);
    assert (errorDetail.empty());
    assert (segment->publishedName()
            == vit::ISharedMemorySegment::platformPublishedName (descriptiveName));

    auto* data = static_cast<float*> (segment->writableData());
    assert (data != nullptr);
    for (size_t i = 0; i < floatCount; ++i)
        data[i] = (float) i + 0.5f;
    for (size_t i = 0; i < floatCount; ++i)
        assert (data[i] == (float) i + 0.5f);

    segment->unmapView();
    assert (segment->writableData() == nullptr);

    segment.reset();
}

} // namespace

int main()
{
    // Short descriptive bodies publish as-is on both platforms (the smoke
    // tester segment name).
    assert (vit::ISharedMemorySegment::platformPublishedName ("Vit_Waveform_Test")
            == "Vit_Waveform_Test");

    const std::string longWaveformName = "Vit_AudioFeature_waveform_track-07_g3_12";
    const std::string longSpectralName = "Vit_Waveform_track-07_g3_12";

#if defined(_WIN32)
    // Windows publishes descriptive names verbatim (pre-A1 semantics).
    assert (vit::ISharedMemorySegment::platformPublishedName (longWaveformName) == longWaveformName);
    assert (vit::ISharedMemorySegment::platformPublishedName (longSpectralName) == longSpectralName);
#else
    // POSIX collapses over-budget names onto "VAF_" + 16-char lowercase hex
    // digest: deterministic, distinct per descriptive name, body within the
    // 30-char PSHMNAMLEN budget.
    const auto digestWaveform = vit::ISharedMemorySegment::platformPublishedName (longWaveformName);
    const auto digestSpectral = vit::ISharedMemorySegment::platformPublishedName (longSpectralName);
    assert (digestWaveform.size() == 20);
    assert (digestWaveform.rfind ("VAF_", 0) == 0);
    assert (isLowerHex (digestWaveform.substr (4)));
    assert (digestWaveform != digestSpectral);
    assert (digestWaveform
            == vit::ISharedMemorySegment::platformPublishedName (longWaveformName));
#endif

    // Lifecycle through the interface: a sub-16KiB payload (POSIX storage
    // rounds up; the logical size stays the mapping size) and a multi-16KiB
    // payload (waveform-tile scale). Re-running the same name afterwards
    // verifies that release really ended the previous segment.
    exerciseSegmentLifecycle ("Vit_SegmentTest_Small", 10);
    exerciseSegmentLifecycle ("Vit_SegmentTest_Large_g1_0", 6 * 1024);
    exerciseSegmentLifecycle ("Vit_SegmentTest_Small", 10);

    return 0;
}
