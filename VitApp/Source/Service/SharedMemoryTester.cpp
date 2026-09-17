#include "SharedMemoryTester.h"

namespace vit
{

std::unique_ptr<ISharedMemorySegment> SharedMemoryTester::testSegment;

void SharedMemoryTester::createTestMemory()
{
    if (testSegment != nullptr)
        return;

    std::string errorDetail;
    testSegment = ISharedMemorySegment::createAndMap (std::string ("Vit_Waveform_Test"),
                                                      sizeof (float) * 10,
                                                      errorDetail);

    if (testSegment == nullptr)
        return;

    auto* data = static_cast<float*> (testSegment->writableData());

    if (data == nullptr)
    {
        testSegment.reset();
        return;
    }

    data[0] = 1.1f;
    data[1] = 2.2f;
    data[2] = 3.3f;
    data[3] = 4.4f;
    data[4] = 5.5f;
    data[5] = 6.6f;
    data[6] = 7.7f;
    data[7] = 8.8f;
    data[8] = 9.9f;
    data[9] = 10.0f;

    testSegment->unmapView();
}

void SharedMemoryTester::releaseTestMemory()
{
    testSegment.reset();
}

} // namespace vit
