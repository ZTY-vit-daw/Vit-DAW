# FIX-PLUGIN-SELECT-1 红绿证据摘要

日期：2026-09-23 · 分支：port/fix-plugin-select-1 · 基线 main@8edd6557（claim commit e3ceb8b8）

## 修前红（实现前记录，同一工作树）

- `red_experimentplugins.txt`：experimentplugins 包对新 v6 套件编译红（undefined: SchemaVersionV5 / StaticEQPlugins / Select* 等，slice 语义缺失）。
- `red_chat_agentloop.txt`：chat 包 undefined: buildFreeStatePluginCandidateDisclosure；agentloop 包 undefined: messageLoopPluginCandidateDirective。
- 红的形态说明：本卡为 schema/API 演进卡，红测试针对新 API 编写，修前红表现为编译红（API 缺失即红）；行为面红样例=TestLoadRejectsUnknownSchemaVersion（v6 文件被旧 loader 整体拒绝）。

## 修后绿

- `go_full_after.txt`：`go test ./... -count=1`（agent 全量）EXIT=0 全绿。
  - 备注 1：全量首次跑时 TestWorkspaceSwitchSettlesInFlightChainExplicitly 一次失败（并发负载时序），隔离复跑 -count=3 与 -count=5 均 ok，文件域（workspace_switch_safety）与本卡 diff 无交集，最终全量复跑绿（§11 记录在案）。
  - 备注 2：contextruntime 无环境失败（先前卡已归置 /var 项）。
- webui：`npm run test` 324/324 绿 + `npm run build`（tsc+vite）成功。
- 新测试文件：
  - agent/internal/experimentplugins/whitelist_v6_test.go（红④ v5 单元素加载+往返、v6 列表、未知版本 fail-closed、族内重复 identifier 拒绝、选择语义全套：单候选无 pin/错 pin、多候选尊重所选/非成员拒/无 pin 拒、PCA 按 selected 条目跑）
  - agent/internal/chat/free_state_d1_plugin_select_test.go（红①绑定=所选条目（路径/名称/identifier/频带参数全部来自所选条目）、红②非成员拒、红③+⑤ v5 单候选行为不变（无 pin=唯一条目、错 pin 历史措辞保留）、多候选无 pin 歧义拒、de_esser v2 族镜像、披露构建器有界性（无 plugin_path 泄漏、单候选 nil））
  - agent/internal/agentloop/ccb_model_prompt_plugin_candidates_test.go（指令渲染披露 identifier、无披露静默、系统提示嵌入门）
