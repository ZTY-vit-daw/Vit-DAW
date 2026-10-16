# Ruling：GENSHELL-CPP-1 pass（2026-10-16 晚，Mac 决策侧）

- **裁定：pass**。合入 main = cherry-pick `3337bd7e`（→ `7745719a`，基线 `b39d9c73`；干净单 commit，parent=领取基线 `1ec6ccfa`，与主仓零冲突）。远端 `port/genshell-cpp-1` 分支在位且指向原 commit（ls-remote 亲核）。
- **决策侧独立复验（Mac 决策会话亲核，非转述回执）**：
  - 文件域亲核：1187 文件全部落在 `/GenShell/**`（含 vendoring：vstgui4 d646b60f Apache-2.0 + nlohmann/json v3.11.3 MIT，各带 VENDORING.md）+ `scripts/gen_shell_skeleton_smoke_mac.sh`——零越域。
  - 合入树零差异：`git diff 3337bd7e HEAD -- GenShell scripts/...` 为空。
  - **独立构建**：`cmake -S GenShell -B <tmp> -DCMAKE_BUILD_TYPE=Release -DGENSHELL_VST3_SDK_ROOT=<主仓 tracktion JUCE 内嵌 SDK>` → exit 0，产物 GenShell.vst3 + genshell_selftest + libgenshell_vst3sdk.a。
  - **壳自测亲跑**：genshell_selftest 对新构建 bundle → SELFTEST GREEN 0 failures，exit 0。
  - 真栈工件亲读：`~/Documents/vit-genshell-artifacts/skeleton/` 四 run 与卡面对得上（两次调试期 RED + **两次 all_green** `genshell_skel_mac_20261013-185035`/`-185107`，两份 skeleton_report.json 均 14 ok / 0 red，六腿 S+K1-K5 齐全）；关键 report/run_meta/prereq 补档入 `coord/runs/GENSHELL-CPP-1/`。
- **诚实边界（与卡面一致，采信）**：编辑器视觉面零断言（进程存活为数据级门）——视觉目检留用户顺手项，非本卡验收门；Windows 端构建未做（规格允许执行端先一端，随 PC 首卡补）；T2 计数读数走单测进程/未来宿主服务直读（JUCE 不暴露非 automatable debug 参数面）。
- **瑕疵记录（不阻塞）**：回执构建命令简写省略 `-DCMAKE_BUILD_TYPE`——裸 Makefile 生成器无配置宏会触发 SDK `fdebug.h` 的 `#error`（决策侧首跑复现）；权威完整命令在 `GenShell/README.md`（带 Release）。后续卡（HOST 烟测等）引用构建命令时以 README 为准。
- **卡 2/4 锚点五条采信**（VSTGUI init 时机 / JUCE 双 IConnectionPoint 占用 / 恢复顺序 / `genshell_state_sync` 内部消息按未知 messageID 丢弃 / debug 参数不可自动化面）——GENSHELL-HOST-1 领取时直接消费。
- **解锁**：池序 96（GENSHELL-HOST-1，P1 关键路径）与 99（GENSHELL-PCA-1，二进制可指纹即领）依赖满足；池序 97（AGENT-1）待 HOST。生成集成面首张落地达成（GEN-API-ADAPTER-1 ruling 的解锁复核点）。
- 验收人：Mac 决策侧（用户 2026-10-16 晚授权验收+继续发卡；PC 主管复核留档待）。
