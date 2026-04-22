#include "SharedMemoryTester.h"

namespace vit
{

HANDLE SharedMemoryTester::testMappingHandle = nullptr;

void SharedMemoryTester::createTestMemory()
{
    if (testMappingHandle != nullptr)
        return;

    testMappingHandle = CreateFileMappingA(INVALID_HANDLE_VALUE,
                                           nullptr,
                                           PAGE_READWRITE,
                                           0,
                                           sizeof(float) * 10,
                                           "Vit_Waveform_Test");

    if (testMappingHandle == nullptr)
        return;

    auto* data = static_cast<float*> (MapViewOfFile(testMappingHandle,
                                                    FILE_MAP_ALL_ACCESS,
                                                    0,
                                                    0,
                                                    sizeof(float) * 10));

    if (data == nullptr)
        return;

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

    UnmapViewOfFile(data);
}

void SharedMemoryTester::releaseTestMemory()
{
    if (testMappingHandle != nullptr)
    {
        CloseHandle(testMappingHandle);
        testMappingHandle = nullptr;
    }
}

} // namespace vit
