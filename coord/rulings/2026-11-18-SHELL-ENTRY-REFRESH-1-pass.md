# SHELL-ENTRY-REFRESH-1 验收：PASS（产物刷新完成，零真栈）

- 卡：`2026-11-18-SHELL-ENTRY-REFRESH-1`（池序 413）；主管=lead（PC 决策流）。
- 执行：`lead/entry413`（DeepSeek-V4.1-Flash，PC），主检出执行、零源码改动、零 git 写、零真栈/零 GUI。
- 前置：前端 main `cf33195b`（410+411 已验收合入）、主仓 main `dfa86cac`（412 已验收合入），均已推 origin/main。

## 结果（主管已核工件与哈希）

| 产物 | 新 SHA256 | 备注 |
|---|---|---|
| VitShell.exe | `5D420DEDB482D6CA2FADA050477C3319722B1AEE6A7CF7DFE922485A1F3FCBA0` | 311,808 B，含 410 producer 的 Release 重建 |
| extension debug DLL | `B270362D1E3380D27A709747611FED149A8A0F54B11BA00C1229834510CD2EB7` | 520,192 B；旧 tracked `E23872FA…` 已就地替换（未提交） |
| VitAgent.exe | `27D09FA09D0CD6F710F9BFAD7E06F44242159987178241953431B13C435C3768` | go build |
| webui dist/index.html | `0886058EF839F421FBA09FDEAE381583FE5E8911D561DD9C4E5866161EFCFB9F` | 含 412 呈现 |
| 启动VitShell.cmd | `293E9B48F6974E5A8CA436B35AADDFDBC1C245FDE18BB548ED80119822BE7CD9` | 未变（路径参数逐项有效） |

- 真实工程 headless 导入：`--import --check-only` exit 0（19.8s）、`--import` exit 0（18.6s），无 crash/GDExtension 错误；部署 DLL 无窗探针 `EXTENSION_REGISTERED/METHODS_OK/FEATURE_ENABLED` 全 true exit 0。
- 收尾无 godot/VitShell/VitAgent/VitApp/VspHub 残留。

## 裁定与边界

- 通过：实际手测入口指向的 VitShell.exe / extension DLL / agent / webui dist 均为 410/411/412 后新产物；入口 cmd 无需改。
- tracked DLL 替换保留为未提交工作树改动（构建产物不入库）；旧 DLL 备份在 temp。后续同步 git 若被该文件阻挡，先记录哈希再处理，不得丢弃新 DLL 证据。
- 边界：`template_release` DLL 仍为 11-14 旧产物（导出/发行路径不在本批）；最小隔离工程的 headless import 崩溃未在真实工程复现（非 409 回归、非产品 `--path` 路径），暂不立卡；真实栈呈现由 414 验证。
- 工件：`coord/runs/SHELL-ENTRY-REFRESH-1/{PLAN,PRESTAGE,FINAL}.md`。
