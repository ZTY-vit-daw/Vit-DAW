#pragma once

#include "SharedMemorySegment.h"

#include <memory>

namespace vit
{

class SharedMemoryTester
{
public:
    static void createTestMemory();
    static void releaseTestMemory();

private:
    static std::unique_ptr<ISharedMemorySegment> testSegment;
};

} // namespace vit
