# Ruling：GENSHELL-PCA-1 pass（修订版·方案 B，2026-10-17 晚，Mac 决策侧——双签完成）

- **裁定：pass**（Mac 决策侧先行裁定 + PC 主管复核留档 [BATCH1 §三](2026-10-17-BATCH1-rulings.md)——双签齐）。合入 main = cherry-pick `4d779236`（前腿词表）+`a198e1c3`（修订腿）→ `0ab9d416`+`6fed5f64`，基线 `25531b40`，干净零冲突。
- **Mac 决策侧独立复验（亲核，非转述）**：
  - diff 亲读：前腿 6 文件（三包词表+测试）+修订腿（native_seed.go 幂等 seed+native_seed_test 5 测+registry NativeSeed 声明与校验+chat/server seedNativeProcessorAdmissions+scheduler Start() 接线+smoke 脚本+run 工件）——全部落修订卡域内；**三道现行门零触碰**（processor_certification_entry.go/harness.go token 段/local_processor_certification.go/receipts_v2.go 均不在 diff，亲核）。
  - seed 语义亲读：同 subject+family+当前 bundle 指纹的 promoted 记录短路（幂等零写入）；未命中→零覆盖签发+PromoteCurrent；typed 家族 fail-closed 拒——与裁定落地设计逐条一致。
  - 烟测工件亲读：`coord/runs/GENSHELL-PCA-1/genshell_pca_mac_20261016-231931/` **12 ok / 0 red**，五腿齐全（A seed 三匹配/B agent 路径放行+action 留痕/C 运行时 revoke 同进程拒——错误串 `pca_load_gate: … no_promoted_current_pca_admission_for_identifier` 在案/D UI 路径 revoke 态回归 ok/E 重启 re-seed 新记录+装载恢复）。
  - **本机亲跑（含本卡改动的 cherry-pick 树，Mac 侧）**：processorattestation/processorregistry/processorintent 三包 +chat 定向全 ok。全量 95 包 0 FAIL 采信 PC 复核独立亲跑（双端各跑一遍）。
- **边界三条采信**：PC 未部署 GenShell.vst3 时 seed WARN 跳过（Mac 部署事实声明，PC 部署后自动生效）；revoke=会话级语义（重启 re-seed 恢复）为裁定既定；identifier 由 GenShell 固定 FUID 派生，FUID 变更须同步 registry 声明行。
- **复验过程瑕疵记档（不阻塞，流程教训）**：Mac 侧首次定向测试误在主工作树（未含本卡改动）上执行——发现后改在 cherry-pick 验收树上重跑方为有效；教训=决策侧复跑必须确认测试树包含被验收提交。
- **解锁与影响**：本卡为独立治理腿，无下游依赖卡；生效后 agent 路径装载 GenShell 走 native 永久准入（promoted 记录+audit 留痕），UI 路径回归不受影响。生成线剩余：AGENT-1（97，doing）→GUI/AUDITION；131（Suno 边车设计，等用户订阅）。
- 验收人：Mac 决策侧（用户 2026-10-17「检查」口令授权）。
