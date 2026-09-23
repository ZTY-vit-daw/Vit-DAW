# FIX-PLUGIN-SELECT-1：恢复自由态 LLM 插件自选——白名单 v6 候选列表 + 提案期选择接入

- 优先级 / 预估 / 依赖：P1 / 1 天 / **FIX-D1-PLUGIDENT-1 合入**（绑定层 identifier 就位是本卡前提）；**用户裁定 2026-09-23「那这个得修啊」**——恢复用户设计的 LLM 自选逻辑于自由态路径
- 模型分级：**L2 / PC 侧 GLM-5.3 亲自开发**（准入核心+G 门邻域，A 类）；mac 侧验证回归
- **背景（决策侧 2026-09-23 核实）**：用户设计的自选逻辑两半——意图层 `processorintent.Intent`（model-owned 族/意图/覆盖，**故意不含身份**）在自由态生效中；身份层候选选择工作流 `plugin_recommendation_selection`（候选带完整身份+交互续接面 server.go:4195）**代码活着但只接老路径**（C2/semantic 系列），自由态 D1 路径为 PCA 单点纪律改成了每族单对象钉定（v5 schema 结构上不支持多候选）。两端同机制非移植问题。mac 现状：EQ 已有 **4 个 PCA 认证主体**（Q10+API-550A+API-560+EMO-F2）但白名单只能装一个——本卡落地后 mac 立即可演示"模型在四个认证 EQ 中自选"。
- **设计冻结（执行侧不得改接口，要改走 blocked/）**：
  1. **schema v6**：`vit.free_state_experiment_plugins.v6`——每族值从单对象演进为**条目数组**（条目形状与 v5 逐字段相同）。**兼容阀**：loader 同时收 v5（单对象→单元素数组）与 v6；旧 v5 文件行为**逐字节不变**（单候选族=今天的行为，演示安全阀）。
  2. **选择流**：族内候选 **>1** 时——提案构造期向模型有界披露候选集（结构性字段：name/manufacturer/identifier/能力面，bounded disclosure 纪律）；模型提案携带所选 `plugin_identifier`；准入从"必须等于唯一条目"改为"**必须是列表成员**"（非成员照拒 fail-closed）；D1 绑定解析到所选条目的 path/name/identifier/参数面。候选 **=1** 时：现行为原样（无 pin 即唯一条目，pin 错照拒）——单候选机器/族零扰动。
  3. **红线**：G1-G8 零改动（选择发生在提案/准入层，门保持 phase-blind/content-blind）；防滥用不破（选择≠绕过 PCA：列表成员=该机已认证主体）；`plugin_recommendation` 候选面复用为主，不新建交互机制。
  4. **持久化兼容（§11）**：v5→v6 旧文件反序列化+往返测试必须有；未知版本 fail-closed 语义明确。
- 目标：
  1. v6 schema+loader 兼容+白名单读取链（含 wl 加载单测：v5/v6/坏文件三态）
  2. 准入 membership 校验替换 exact-equality（free_state_d1_plan_table.go admission 段）+ D1 绑定到所选条目
  3. 提案期候选披露+选择解析（improvement_proposal_workflow 侧，结构性字段）
  4. 红测试：①多候选族模型选择被尊重（绑定=所选条目）②非成员 pin 拒绝 ③v5 单候选行为不变 ④v5 文件加载为单元素列表+往返 ⑤候选=1 时无 pin 照常
  5. 回归：agent 全量+webui 绿；PC 真栈插件写 spot 1 轮 exit 0（现白名单单候选=行为不变证明）
  6. mac 验证腿（用户转交）：EQ 多候选选择腿——4 认证 EQ 披露→模型自选→装载/应用/回读走完 exit 0（自选复活实证）
- 文件域：`agent/internal/chat/`（白名单加载/free_state_d1_plan_table admission+绑定/improvement_proposal_workflow）+ `agent/internal/experimentplugins/`（schema）+ 测试；零内核/零 G 门/零 webui 预期（如披露需 webui 面调整，列明后最小化）
- 验收：①红测试修前红/修后绿；②agent 全量+webui 绿；③PC spot exit 0；④mac 多候选腿 exit 0；⑤回执记两端 HEAD+两端白名单版本
- 停止条件：发现选择流必须动 G 门或 PCA 认证面才能绿 → 域外上交；webui 披露面超出最小改动 → 拆子卡上交
- 领取：
- 回执：
- 验收：
