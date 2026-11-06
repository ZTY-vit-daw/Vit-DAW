# SMOKE-ENC-HYGIENE-2 构造验证工件（probe_270）

- 执行：GLM 主会话 freeflash 车道子代理（PC，worktree D:\Vit_DAW_wt_270impl，分支 port/smoke-enc-hygiene-2）
- 日期：2026-11-07（attempt3 00:19:30，exit 0，5/5 ALL PASS）
- 口径：零真栈零 LLM；模式参 268 probe（TEMP 隔离桩 × 每腿独立 × pre/post-fix 对照）；只覆盖本卡两读取点
- 被测行来源：**pre-fix 行=领取基线 `git show 8732bd21:scripts/run_free_state_d1_smoke.ps1`（construct/pre_baseline_run_free_state_d1_smoke.ps1）中提取的完整源语句；post-fix 行=工作树快照（construct/postfix_run_free_state_d1_smoke.ps1）同一语句**——probe 对两快照 Select-String 提取（锚定赋值目标，排除 :665 同变量带编码读取的歧义），Invoke-Expression 逐字执行，保证被测=脚本实际存在的行
- 桩面：UTF-8 无 BOM 字节级构造（probe 内 byte[] 硬编码，probe 文件本身纯 ASCII，无脚本文件编码自坑）：
  - manifest 桩（:445 面）=`{"cases":[{"public_case_id":"case_zh_enc_270","project_path":"D:/工程桩/样例_混音_测试.json"}]}`
  - report 桩（:630 面）=`{"branch":"","reason":"预算上限已到：第 2 轮未通过"}`

## 一、腿表（attempt3，机器可读于 attempt3_20261107_001930/probe_270_summary.json）

| 腿 | 被测行 | 桩 | 期望 | exit 断言 | 结果 |
|---|---|---|---|---|---|
| leg1_manifest_prefix | pre :445 | manifest，UTF-8 无 BOM 含中文路径 | 缺陷存在（值≠源） | got=`D:/\u5bb8\u30e7\u25bc...`（GBK 误读乱码）≠ expected=`D:/\u5de5\u7a0b\u6841/\u6837\u4f8b_\u6df7\u97f3_\u6d4b\u8bd5.json`（工程桩/样例_混音_测试.json），form=mojibake_or_wrong_value | PASS |
| leg2_manifest_postfix | post :445 | 同字节桩 | clean parse 且值==源 | got==expected 逐字节一致，form=clean_parse | PASS |
| leg3_reason_prefix | pre :630（try/catch{} 同脚本 :629-:633 形制） | report，UTF-8 无 BOM 含中文 reason | 缺陷存在（reason≠源） | got=`\u68f0\u52ed...`（乱码）≠ expected=`\u9884\u7b97\u4e0a\u9650\u5df2\u5230\uff1a\u7b2c 2 \u8f6e\u672a\u901a\u8fc7`（预算上限已到：第 2 轮未通过），form=mojibake_or_wrong_value | PASS |
| leg4_reason_postfix | post :630（同 try/catch 形制） | 同字节桩 | clean parse 且 reason==源 | got==expected 逐字节一致，form=clean_parse | PASS |
| leg5_static_encoding_coverage | 两快照全文件静态扫描 | — | 268 四处+本卡两处零回退 | post 快照 Get-Content 共 7 行、无 -Encoding 的 0 行；pre 基线无编码残留恰 2 行（即本卡 :445/:630） | PASS |

probe 退出码=0（5/5 ALL PASS）。console 明细=`attempt3_20261107_001930/console.txt`（detail 用 \uXXXX 码点转义，无控制台代码页歧义）；summary JSON 另存原始字符串。

## 二、证据链要点

- **:445 缺陷形态**（leg1）：pre 行 GBK（CP936）误读 UTF-8 无 BOM 桩→JSON 结构字符（引号/括号）幸存→ConvertFrom-Json 成功但 project_path 为乱码字符串→真实后果：materialize 路径错→LW watch 配置失败（config_error 面）；post 行（-Encoding UTF8）同桩逐字节正确读回（leg2）。
- **:630 缺陷形态**（leg3）：pre 行乱码读出 reason→真实后果：NOT_EXERCISED 分支向 console 打印乱码 reason（MRREG1 打印面失真）；post 行正确读回（leg4）。两腿均套用脚本 :629-:633 的 try/catch{} 原形制。
- **零回退**（leg5）：post 快照全部 7 处 Get-Content 均带 -Encoding（含 268 修复的 :292/:665/:952/:1123/:1459 与本卡 :445/:630）；pre 基线恰 2 处残留=本卡目标。268 leg1 形制（post-fix 全绿腿）由本 probe leg2/leg4 同构覆盖，未重做 268 桩面。

## 三、attempt1/attempt2 留档（probe/桩面问题，非被测脚本）

- attempt1（probe_270.ps1 初版）：line extraction 的 pattern 未锚定赋值目标，`$report` 同变量 :665 行（带编码）造成 2 行匹配歧义→启动即 throw。工件未建目录，无工件。
- attempt2（attempt2_20261107_001612）：manifest 桩漏最外层 `}`（`"}]` 应为 `"}]}`）→leg1/leg2 均 ConvertFrom-Json ArgumentException（JSON 残缺，与编码无关，leg1 为假阳性）；leg3/4/5 已有效 PASS。另桩首字"工"字节笔误（0xE5,0x9B,0x8A=固）于 attempt3 修正。诊断脚本 `diag_270.ps1`+attempt2 工件留档。
- attempt3：桩补全+字节修正+console 码点转义后 5/5 ALL PASS。

## 四、诚实边界（§7 口径）

- 本验证=脚本层两读取面的构造证明（离线桩文件）；**不证明**真实栈上 manifest/report 实际含中文的频率与后果（268 裁定已确认同族缺陷真实触发过，本卡为挂账清偿）；
- 桩形制按脚本读取面钉定：:445 读 `$PublicManifest`（fixture manifest，cases[].project_path 面）；:630 读 `$report`（runner d1_smoke_report.json，.reason 面）——字段面忠实，非完整桩脚本（268 式全脚本桩面不在本卡范围）；
- 断言语义零改动：git diff 仅两行各加 `-Encoding UTF8`，无其他改动；
- leg1/leg3 的 mojibake 字符串中 `?` 为 GBK 解码器对奇数尾字节的替换符，属缺陷形态的一部分，原始字节以 `stub_bytes_copy.json`+summary JSON 留证。
