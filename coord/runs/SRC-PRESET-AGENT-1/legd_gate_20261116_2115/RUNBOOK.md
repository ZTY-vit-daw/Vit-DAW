# RUNBOOK：384 腿 D 闸门（10 分钟级，主管 2026-11-16 授权）

## 跑前口径（AGENTS §8，跑前成文）

- 运行次数：**正式 1 次**，单 run 无重跑；装置域修复后重跑须分记 attempt。
- 成功条件：kernel_runtime XML-boot → add_track → instantiate Surge XT → save_project{} → 从 boot XML 提取 state 样本 → 三层解码归层判定落 `state_analysis.json`（verdict ∈ {xml_wrapper, binary_309_identical, binary_309_family, raw_save_patch, new_form}）。
- 失败分类：`env_interrupt`（内核起不来/端口冲突/ZMQ 超时）/`face_defect`（内核命令面拒绝：add_track/instantiate/save 任一 status≠ok）/`probe_defect`（driver/解析器缺陷）。环境中断只有留原始日志才可排除有效轮次。
- 止损线：主管令——30 分钟内未取得样本或环境故障两次→放弃占栈、记录分类、交还栈位（382 排队等栈）。同断点连败 2 次禁原样重跑。
- LLM 计费上界：**0**（全确定性 ZMQ 面，无 chat/无 LLM 调用）。

## 被测物 provenance（§9）

- 内核=复用 374/377 同一二进制 `VitApp.exe` sha256 `a59473614152882b…8348e`（主仓 04:33 Nov-16 构建；源=现 main C++ 减 378-D3 的 7 行披露改动（CommandDispatcher effective_project_path 披露+VspKernelReference，不触 state 序列化路径）——本闸门零内核改动，行为等价论证在卷；`runtime_provenance.txt`）。
- 插件库 Settings sha256 `5abf7d95…dae076`（staging 复制，含 Surge XT VST3 isInstrument=1 亲核）。
- boot 种子=staging default_project.xml sha256 `00651bc3…8320`（XML-boot→save_project{} 回写明文=309 同款 clear-XML 分支）。
- 栈位：kernel-only（VitApp.exe 单进程，ZMQ 5555/5556+UDP 4445；无 agent/UI/Godot）。
- S10 recents：`VIT_DAW_USER_CONFIG_DIR` 内核 grep 亲证不支持（351 未在基线）→ 手工快照纪律：before.json 落 run 目录→teardown 还原+哈希亲证。

## 步骤

1. 登记（占栈行推 main 亲核）→ ≤60s 复核进程/端口净空；
2. `VIT_PROJECT_XML=<run>/kernel_runtime/Workspace/default_project.xml` 起 VitApp.exe（Hidden）→ ping；
3. `python legd_gate_driver.py`（add_track→instantiate Surge→settle 3s→get_plugin_parameters 摘要→save_project{}）；
4. `python analyze_state_sample.py <boot xml> state_analysis.json`；
5. teardown：亲停 VitApp pid → TCP 5555/5556+UDP 4445 监听=0 亲证 → recents 还原+哈希亲证 → 释放行推 main。
