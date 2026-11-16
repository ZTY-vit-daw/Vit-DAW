# RUNBOOK：384 腿 D 真栈腿（一次占用=两段 boot + 中途离线装配）

## 跑前口径（AGENTS §8，跑前成文）

- 运行次数：**正式各 1 次**（boot A 采集=1；boot B 验证=1），单 run 无重跑；装置域修复后重跑分记 attempt。中途离线步骤（builder/manifest/三门）不占栈。
- 成功条件：
  - boot A：`capture_craft_states.py` exit 0——两组 patch 内参数全部写读回核对（|actual-requested|<1e-3）+save sidecar+两 state 提取+互异+异于 init；
  - 离线：builder 再生终版模板+manifest 同步+`go build ./...` exit 0+`go test ./...` 全包 exit 0；
  - boot B：`verify_final_template.py` exit 0——A1 五轨/A2 三 clip length_seconds=6.0/6.0/12.0（=8/8/16 拍@80bpm，382 修正判据）/A3 音符 8/21/16（鼓键位⊆{36,38,42,46}；旋律/主题=60）/A4 三 Surge state sha=两 craft 钉值/B 三轨离线探针非静音（ready+nonzero+max_abs≥0.01+nan0，373 DG1-B B2/B3 同判据）。
- 失败分类：`env_interrupt`（内核起不来/端口冲突/超时）/`face_defect`（内核命令面拒绝或回读不符：instantiate/set_plugin_param/save/probe 任一）/`probe_defect`（driver 缺陷）/`assertion_failure`（B 探针静音或 A 系回读不符）。环境中断只有原始日志在卷才可排除。
- 止损线：同断点连败 2 次禁原样重跑；占栈预计 ≤40 分钟；环境故障两次→释放栈位上交。
- LLM 计费上界：**0**（全确定性 ZMQ 面）。

## 被测物 provenance（§9）

- 内核=闸门同款 `VitApp.exe` sha256 `a59473614152882b…`（374/377/384-闸门同二进制；本腿零内核改动）；插件库 Settings `5abf7d95…`（staging 复制含 Surge）；boot A 种子=staging default_project.xml；boot B 被测=**再生后终版 template.v1.xml**（builder 产物）。
- craft 参数集（patch 内，避开通证不入序列化的 Global Volume）：soft-keys 10 参（单振荡/低通 0.38/共振 0.12/延音 0.55/释音 0.35/scene 音量 0.75…）；percussion 11 参（attack 0/decay 0.18/sustain 0/release 0.08/噪声 0.4/低通 0.5/共振 0.2…）——全清单在 `capture_craft_states.py`。
- 波形依据：Surge 唯一会响音源（309 Waves 全系 RT-SILENT -120dBFS）；GR 钢琴路线因此排除——「会响的钢琴」本机不存在，旋律/主题=柔化合成键近似+manifest 诚实声明（M2 用户复听节点裁定）。
- S10 recents：before=`807226F4AF3F…`（本 run 目录 before.json）；VIT_DAW_USER_CONFIG_DIR 内核不支持（闸门亲证）→手工快照纪律。

## 步骤

1. 登记（占栈行推 main 亲核）→ ≤60s 复核净空；
2. boot A（start_kernel_bootA.ps1）→ `python capture_craft_states.py` → state 文件落模板 states/；
3. 停内核 → 离线：`python scripts/src_template_build.py` → manifest 同步（template_sha256/state sha/声明字段）→ `go build`+`go test ./...` 全量；
4. boot B（start_kernel_bootB.ps1，VIT_PROJECT_XML=终版模板）→ `python verify_final_template.py`；
5. teardown：亲停 pid → TEARDOWN_ALL_ZERO 亲证 → recents 还原+哈希亲证 → 释放行推 main。
