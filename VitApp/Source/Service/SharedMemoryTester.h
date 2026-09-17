#pragma once

#if defined(_WIN32)
#include <windows.h>
#endif

namespace vit
{

class SharedMemoryTester
{
public:
    static void createTestMemory();
    static void releaseTestMemory();

private:
#if defined(_WIN32)
    static HANDLE testMappingHandle;
#endif
};

} // namespace vit
