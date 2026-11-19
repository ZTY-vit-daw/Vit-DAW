# NEXT-SESSION-BRIEF（下次会话入口；gate 时更新）

- 更新：2026-11-19 18:30（Asia/Shanghai）；上次 gate：`coord/reports/2026-11-18-gate-2215.md`。
- main：`745b6265`（本文件提交后以实际 HEAD 为准）；webui dist 不变 `FA293DB3…304E`。
- 栈：空闲（teardown 后 Vit 系进程 0；TCP 7878/7879/8787/5555/5556/9333=0；UDP 4445=0）；无运行中子代理/后台 job。

## 用户已定事项

- 2026-11-19：**425 真栈腿授权**；本会话由 lead 亲自执行，不用 GPT/Codex 执行流。
- 425 真栈 S0–S7 **PASS**，W01（解析/编译/工程绑定）收口；卡已 done；裁定 `coord/rulings/2026-11-19-G0-LEGAME-BIND-1-realstack-pass.md`。
- LLM 配置：`~/.vit/config.json` 已是 DeepSeek 直连（`https://api.deepseek.com/v1`，key 与桌面 `DS apikey.txt` 一致；`/models` 验证通过）。启动栈时清除 `OPENAI_*`/`VIT_AGENT_LLM_*` 环境覆写即可保持。
- 入口左栏 + 右侧对话窗口 A 方案已验收；左栏内容动效 800ms 定稿；设置唯一入口=左栏 footer；新对话无节点从 HEAD 分叉；加载卡在消息前；契约 R1–R6 冻结。

## 未结/下一首排

1. **422 CONV-CROSS-COMBO-1**（todo）：B2b 跨工作树真栈组合验证；先写 runbook+脚本（零栈），再候栈窗口；421/419 已合入。
2. **W02 最小切片**：设计卡已落 [432 W02-A1](../cards/todo/2026-11-19-W02-A1-LESSON-CTX-CLIP-1.md)——审计发现选中 clip 未消费、pullback 注入不达模型；建议先派 A1a（选中 clip 绑定，零栈可测），A1b 另定 prompt 路径；W18 不在 A1 依赖内。
3. **425 装置债**：berth 启动前复制并还原 `Settings.xml`/`default_project.xml`；425 脚本内建 A/B 创建或 setup 前置隔离；`default_project.xml` 已被内核改为空白默认（预跑为六 stems 测试残留，非用户工程；仅存哈希无法逐字节还原）——**用户 2026-11-19 已裁定接受空白默认**。详见 `coord/runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/DEVICE-NOTES.md`。
4. 用户节点：390 YuE instrumental 真栈生成+人耳；384 M2/点音色演示；材料听核。
5. 旧池未动：doing 8 / blocked 6；POOL-INDEX 397–431 回填留 gate。

## 读入口（按序，不读全史）

`NEXT-SESSION-BRIEF` → `coord/cards/POOL-INDEX.md` → 相关 doing/blocked 卡与 `coord/rulings/2026-11-19-*` → 需要时再读契约/代码。

## 规则

- 先读 `coord/decisions/2026-11-18-agent-economy-rules.md` 与 `coord/agents/ROSTER.md`；同产线优先复用子代理；回执≤15行、证据落 coord/runs；主管不逐行读 diff。
- 真栈腿：berth 前**复制**权威文件（Settings.xml/default_project.xml）与 recents 快照；teardown 还原并哈希亲证；单 owner、单正式轮、失败即停。
