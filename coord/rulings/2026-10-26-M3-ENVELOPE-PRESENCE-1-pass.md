# 裁定：M3-ENVELOPE-PRESENCE-1（190）整卡 pass（2026-10-26）

- 验收人：GLM 主管决策流（ZCode 决策会话）；实现=GLM-5.3 PC 执行会话（72a4f0ea @ port/m3-envelope-presence-1）；合入=87b032e2（main）。
- 依据（三项亲核）：
  1. **diff 亲读**：单 helper（appendEnvelopePointCountsToParameterDescriptors，resolve 失败如实回 0）+handleGetPluginParameters 单调用点，插入位置在 parameterSurfaceSignature 之后（增列不污染签名）——与回执"双保险"声明一致；ps1 +47 行仅新增 -EnvelopePresenceSmoke 模式；文件域未越卡。
  2. **判定面亲核**：summary.json 12 检全绿+console PASS+探针断言实现抽查（strip_envelope_column 基线比对/diff_replies 全回执逐键 diff+漂移披露不静默/assert_column int 型校验）——真栈 ZMQ 5555 命令面，装载 ValhallaSupermassive 实插件，基线双查全等→三点目标=3 未动=0→既有字段零改动（唯一披露差异 0→3）→clear 回基线全等。
  3. **栈记录亲核**：登记 14dee1d2→18:55:36 即时复核（距启动 <60s）→无 already listening→登记先于启动；VIT_PROJECT_XML 隔离亲证（副本被写 18:56:41/worktree 工程运行前后同哈希 947e4d00/Settings 零触碰）；拆栈复核全零。
- 采信披露：内置轨级参数（volume/pan）不在 get_plugin_parameters 回执面=内核既有形态（189 L1 逐字在卷），本卡覆盖面=外部 rack VST 插件参数——如实登记，符合卡面停止条件主管补注。
- 主管裁定（遗留项处置）：轨级参数在场性数据源**另裁**（候选=轨级 automation.get_curve 探测/内核轨参数面小扩），不阻塞消费面——189 二段自动展开腿与 191 徽章首版覆盖面=外部插件参数，轨级缺口如实呈现不造假，数据源定案后补覆盖。
- 记录性 nit（不影响判定）：回执/栈记录行文"20 参数行"，summary.json 实为 21 行——以 JSON 为准，断言面"全行=0"不受影响。
- 端测边界声明（四项）采信：a) 裸 ZMQ 命令面全链（与 Godot 宿主同面）；b) 零改动=diff 级+同构建双查；c) agent/webui/Godot 零改动；d) 消费端（自动展开）归 189 二段。
