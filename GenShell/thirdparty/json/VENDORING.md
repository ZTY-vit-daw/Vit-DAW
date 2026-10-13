# nlohmann/json vendoring 记录

- 来源：https://github.com/nlohmann/json.git（tag `v3.11.3`）
- 获取方式：浅克隆取 `single_include/nlohmann/json.hpp` 单头文件 + LICENSE.MIT 入库（非 submodule，理由同 thirdparty/vstgui4/VENDORING.md）
- 上游 tag：`v3.11.3`（2026-10-13 克隆）
- 许可：MIT（见 LICENSE.MIT）；文件改名为 `nlohmann_json.hpp`（include 形态 `#include "nlohmann_json.hpp"`，避免与其自身 `#include <nlohmann/json.hpp>` 内部引用混淆——单头文件自身无内部 include，改名安全）
- 用途：GenShell T3 chunk（vit.genshell.state.v1）与 T2 消息信封（vit.genshell.msg.v1）的 JSON 编解码（GENSHELL-CPP-1）
