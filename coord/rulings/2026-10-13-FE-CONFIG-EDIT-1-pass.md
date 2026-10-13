# Ruling：FE-CONFIG-EDIT-1 验收 pass（手测后置）

- 卡：FE-CONFIG-EDIT-1（池序 105，设置页 LLM 配置输入体验修复四件）
- 实现：ea8df384 @ port/fe-config-edit-1（worktree Vit-DAW-feconfig1）
- 验收：Mac 决策侧 2026-10-13（决策流验收 + 独立执行流复验分工，依用户当日裁定：决策流不承担执行性复验）
- 裁定：**pass**（手测后置边界见下）

## 证据链

1. 决策侧 diff 审查：四件全部落实——retainSecretInputs 沿用输入态（正确处理服务端 RedactSecrets 不回明文语义，清空语义保留给重载/重开）；占位符乱码修复；autoComplete 主/路由 key=new-password、其余=off；settingsSavedStatus 两情形文案在场。新增 export 仅为测试可达性，无行为影响。
2. 复验 run [20261013-1015-reverify](../runs/FE-CONFIG-EDIT-1/20261013-1015-reverify/)（独立执行流，样板=20261012-2315-browser）四门：
   - npm run test → exit 0（38 files / 454 tests，与回执一致）
   - npm run build → exit 0
   - 乱码口径 A（[渚涘簲鍟嗘ā鍨] 排除 *.test.*）设置页 0 命中
   - 浏览器级 16/16 PASS：真键盘 cua.type 两轮 41 字符无丢失、保存后主/路由 key 输入态保持（旧行为此处清空）、两情形状态行文案、重载清空语义保留、7/7 路由卡 new-password、落盘 PreserveBlankSecrets（留空保存后旧 key 保留、种子路由 key 原样未动）
   - 环境：隔离 HOME（用户真实 ~/.vit 未触碰，mtime 佐证）；vitagent sha256 与昨日 run 一致（同源码确定性构建）；进程收尾核实
3. 用户容器旁证（非验收门）：用户 2026-10-13 09:59 在**未修复版**（9-19 dist）设置页成功写入真实 key 并实测 LLM 连通——后端保存链路一直正常，与取证结论一致。

## 复验修正记录

回执口径"真键盘 34 字符"与两串实际长度（20+21=41）不符，复验按实际记录，采信复验口径（证据量只多不少）。

## 手测后置边界（不追改本裁定）

Safari/WKWebView（Godot webview）容器与中文 IME 组合场景留用户实机手测（卡面 [等待用户] 五条），前置=dist 重建部署完成。若手测复现"间歇性无法输入"→ 开返工卡，取证方向=WKWebView 自动填充/IME composition 深层干预（可能超本卡文件域，按 §11 另立卡）。

## 域外发现裁量（§11）

复验广谱乱码复核发现 3 处卡前存量 GBK→UTF8 双重编码文案（App.tsx 内核错误/显式步进/节点，origin/main 与 ea8df384~1 均在场=非本卡引入，设置页域外）：不阻塞本卡，立卫生卡 FE-MOJIBAKE-CLEANUP-1（池序 107）。

## 合入与后续

- merge 合入 main：见本提交前序 merge commit（--no-ff，含昨日 run 工件）
- dist 重建部署（9-19 旧构建→本次修复）：执行活，派发独立执行流（用户 2026-10-13 裁定：决策流不承担执行）
- 手测清单：dist 就位后用户执行
