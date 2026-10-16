# Mac 决策侧收工 gate（2026-10-16，CEF 输入线专场）

- 本流角色：Mac 决策侧（用户 10-13 晨授权）；下一条决策流由用户新开，本报告为其入口底账之一。

## 一、本流战果（全部已推 main）

| 项 | 结果 |
|---|---|
| FE-CONFIG-EDIT-1（105） | pass 合入+dist 重建（设置页四件修复全闭环） |
| API key 接线 | 用户实配完成（hejuapi/gpt-6.1-sol 实测 200；CF 1010 教训=测试须带浏览器 UA） |
| FE-CEF-KEYINPUT-RECON-1（111） | pass：断点层终判=gdcef 焦点/IME 桥乒乓（三层探针取证） |
| FE-CEF-FOCUSFIX-1（114→116 前身） | pass（带边界）：上游定位 dsh0416/godot-cef #228/#236 + 宿主补偿机制 |
| FE-CEF-UPGRADE-1（116） | 交付采信硬门 FAIL 关账：v1.15.4→v1.16.2 升级+七项回归（⑤ipc 后证伪=探针假阴性） |
| FE-CEF-V1162-GAPS-1（121） | 终局 pass：⑤假故障结案+⌘V 垫片+用户终验（粘贴腿后翻案见 SHIM-2） |
| FE-CEF-PASTE-SHIM-2（122） | 终局 pass：捕获点 Window _input 升级，用户 10-16 终验「API 已配置正常」 |
| 卫生卡 FE-MOJIBAKE-CLEANUP-1（108） | 在池未派（App.tsx 三处存量乱码，L1 小卡） |

CEF 输入失灵缺陷链五卡全闭环；前端= v1.16.2+粘贴垫片默认开+补偿默认关（回滚 v1.15.4 须关垫片防双插——已入账）。

## 二、待新决策流处理（生成能力开发相关排序建议）

1. **GENSHELL-CPP-1（95）done 待验收**（10-13 起，真栈烟测两次 all_green+壳自测 46 PASS）——生成线首件，验收后解锁 HOST→AGENT→GUI→PCA→AUDITION 五卡序列（规格=coord/runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md）。
2. genapi 真源验证轮（用户待办解锁：key 已配好；填 ~/.vit/genapi_providers.json 零代码+首跑 gen.generate 真源端到端）。
3. 生成线池内：AIGC-DRUM-GEN-1、GEN-VOCAL-SYNTH-1（SVS）、ARRANGE-RECON-1 等。
4. 挂账不阻塞：上游 godot-cef issue（只报 ⌘V 翻译层一项）发布权待用户；SHIM-2 run 工件未入库（场景在前端仓可复跑）。

## 三、环境事实（新流直接可用）

- 前端仓 /Users/timozty/Documents/vit-daw-frontend @ port/fe-cef-v1162-gaps-1（255fa69，含垫片）；GH001 推送限制（>100MB 历史二进制）照旧——补丁存档主仓 run 目录模式。
- gdcef v1.16.2 在位；v1.15.4 归档 ~/Documents/vit-daw-cef-backups/（BIN_ARCHIVE_V1154.sha256）。
- 探针基建常驻可复用：input_probe 7879（重启=KEYINPUT run SERVICE.md）；诊断场景（autodrive/paste_diag/input_hook/click_matrix）在前端 vit_dock/diagnostics/。
- 用户手测路径=Godot 编辑器 Cmd+B 运行→前端激活 agent；干净终端（勿继承取证 export）。
- 网络间歇：git 直连死时走本机 7890 代理（反向兜底，10-16 实证）。
- 工作树：主仓 port/rlm-profile-2（历史不动）+运行时状态文件；前端仓 port/fe-cef-v1162-gaps-1。

## 四、纪律备忘（本流教训固化）

- 用户终验以**明确反馈**为准，不完整日志+口头模糊确认不落 pass（SHIM-2 翻案教训）。
- 自动化验证路径与真实用户路径的分叉点必须在卡面显式列边界（eval-focus vs 真实点击聚焦）。
- 大文件下载（>百 MB）一律用户手动执行，卡面写明。
- 池序推送竞争频繁（PC 并行流）：立卡前 grep 确认+被拒即 fetch 合流重推（后发者改号）。
