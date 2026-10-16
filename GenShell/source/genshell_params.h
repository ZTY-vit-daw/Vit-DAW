//------------------------------------------------------------------------
// GenShell — T1 参数面注册（processor 与 controller 各注册同一集合；件一 §3）
// processor 侧为纯值缓存（音频线程同步用），controller 侧为完整 Parameter 对象。
//------------------------------------------------------------------------
#pragma once

#include "pluginterfaces/vst/vsttypes.h"

namespace Steinberg {
namespace Vst {
class ParameterContainer;
} // namespace Vst
} // namespace Steinberg

namespace GenShell {

// 在 controller 的 ParameterContainer 上注册完整 T1 参数面 + debug 计数参数。
// 返回值：无（异常时 std::bad_alloc 自然传播——注册发生在构造期）。
void registerControllerParameters (Steinberg::Vst::ParameterContainer& parameters);

} // namespace GenShell
