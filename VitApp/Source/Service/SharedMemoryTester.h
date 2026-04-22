#pragma once

#include <windows.h>

namespace vit
{

class SharedMemoryTester
{
public:
    static void createTestMemory();
    static void releaseTestMemory();

private:
    static HANDLE testMappingHandle;
};

} // namespace vit
