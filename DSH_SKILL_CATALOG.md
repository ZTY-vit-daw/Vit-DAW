# DSH 可用开源 Skill / 插件清单（2026 整理）

> 用途：为 Vit-DAW 开发（GUI 前端）、论文写作、PPT 制作储备的现成开源技能。
> 需要时按本清单安装，无需自己编写 skill。
> 最后整理：2026-08（本会话调研核实，仓库链接均已验证可访问）。

---

## 0. 安装方式速览（DSH skill 机制）

| 方式 | 做法 | 适用 |
|---|---|---|
| ① 手动复制（最简单） | 把仓库里 `skills/<名称>/` 目录（含 SKILL.md）复制到 `C:\Users\timoz\.dsh\skills\`（用户级，不存在则创建）或 `D:/Vit_DAW/.dsh/skills/`（项目级） | 任何标准 SKILL.md 技能 |
| ② 管理插件 | npm 装 `dsh-plugin-capabilities`，在 DSH 设置页「技能与 MCP」里管理（见 §F1） | 推荐先装这个 |
| ③ 市场/仓库导入 | 用 §F1/F2 插件的市场或仓库导入功能，从 GitHub 拉取 | 不想手工复制时 |

- Claude Code / Codex 的 SKILL.md 技能与 DSH 格式兼容（标准 frontmatter + Markdown 正文），**可互换使用**。
- 部分技能依赖本机工具（python-docx / python-pptx / playwright / poppler 等），装技能时同步补依赖。

---

## 1. 前端 / GUI 设计类

| 技能 | 来源 | 许可 | 干什么 | 适合你的场景 |
|---|---|---|---|---|
| **frontend-design** | anthropics/skills（官方） | Apache-2.0 | 前端 UI 设计：布局、视觉层次、可访问性评审与实现指引 | Vit WebUI 改版设计规范、新面板 UI 评审 |
| **webapp-testing** | anthropics/skills（官方） | Apache-2.0 | Web 应用测试方法论 | WebUI 测试补充 |
| **canvas-design / theme-factory / brand-guidelines** | anthropics/skills（官方） | Apache-2.0 | 画布/海报设计、主题系统生成、品牌指南 | 视觉资产、答辩海报 |
| **web-artifacts-builder** | anthropics/skills（官方） | Apache-2.0 | 生成 Web artifact（HTML 原型） | 新功能原型先行确认 |
| **open-design**（设计工作台） | [nexu-io/open-design](https://github.com/nexu-io/open-design) + DSH 插件 `@w3kits/plugin-opendesign`（npm） | Apache-2.0 | Claude Design 开源替代：31 个设计 skill + 72 套品牌 Design System（Linear/Stripe/Vercel…）+ 5 套视觉方向（OKLch 色板+字体栈）+ 4 个 PPT deck 模式；从 prompt 生成设计/原型 | **视觉改版方向探索、精美 PPT**；不用于日常改码 |
| **huashu-design**（画术） | [alchaincyf/huashu-design](https://github.com/alchaincyf/huashu-design) | MIT | 设计哲学：Junior-Designer 工作流、5 步品牌资产协议、anti-AI-slop checklist、五维自评审 | open-design 的设计方法论内核，可单独用 |
| **playwright**（已有） | 你的 `~/.codex/skills/playwright` | — | 真实浏览器自动化：导航、截图、UI 流程调试（playwright-cli） | 装上 §F1 后 DSH 直接可用，接上 WebUI 截图验收闭环 |

## 2. PPT 制作类

| 技能 | 来源 | 许可 | 干什么 | 备注 |
|---|---|---|---|---|
| **pptx** | anthropics/skills（官方） | Apache-2.0 | 用 python-pptx 生成标准 .pptx（含模板遵循、布局审计） | **最稳的主力**；需 `pip install python-pptx` |
| **guizang-ppt-skill**（归藏） | [op7418/guizang-ppt-skill](https://github.com/op7418/guizang-ppt-skill) | **AGPL-3.0 ⚠️** | 杂志风 PPT：单文件 HTML → PDF，WebGL hero，P0/P1/P2 checklist | open-design 内置的就是它；答辩视觉效果最强，注意 AGPL（内部/答辩用无碍，分发需开源） |
| open-design 的 deck 模式 | 见 §1 | Apache-2.0 | simple-deck / replit-deck / weekly-update 等 4 种 deck 模式 | 需要整个工作台 |

## 3. 论文 / 学术写作类

| 技能 | 来源 | 许可 | 干什么 | 备注 |
|---|---|---|---|---|
| **docx** | anthropics/skills（官方） | Apache-2.0 | Word 文档生成/编辑（标准 .docx） | 论文排版主力；需 `pip install python-docx` |
| **academic-research-skills**（ARS v3.9.2） | [Dubaoxu/distillation-skills](https://github.com/Dubaoxu/distillation-skills) 内含 | **CC BY-NC 4.0 ⚠️** | 4 技能：deep-research（深度调研+三索引交叉验证 Semantic Scholar/OpenAlex/Crossref）、academic-paper（10 模式论文写作）、academic-paper-reviewer（5 审稿人评审）、academic-pipeline（全流程编排+完整性门禁） | 学术研究最完整套件；**非商用许可**（答辩/毕设 OK，发表带署名，商用需授权） |
| **paper-writing-agent** | 同上仓库 | 见文件头部 | 通用论文写作协调 Agent：8 阶段、自动领域识别 | 轻量替代 |
| **academic-research-skills**（另一套） | [cleardry/academic-research-skills](https://github.com/cleardry/academic-research-skills) | 待确认 | research → write → review → revise → finalize 全流程 | 英文环境友好 |
| **doc-coauthoring / academy-guide** | anthropics/skills（官方） | Apache-2.0 | 文档协同写作、课程/教程指南 | 辅助 |

## 4. PDF / 办公文档类

| 技能 | 来源 | 许可 | 干什么 | 备注 |
|---|---|---|---|---|
| **pdf** | anthropics/skills（官方） | Apache-2.0 | PDF 生成/编辑/提取 | 需 python 库（reportlab/pypdf 等） |
| **xlsx** | anthropics/skills（官方） | Apache-2.0 | Excel 表格 | 数据整理 |
| **pdf**（已有） | 你的 `~/.codex/skills/pdf` | — | Poppler 渲染视觉检查 + reportlab/pdfplumber/pypdf | 装上 §F1 后 DSH 可用；需装 Poppler |

## 5. 工程方法论 / 开发流程类

| 技能 | 来源 | 许可 | 干什么 | 备注 |
|---|---|---|---|---|
| **superpowers** | [obra/superpowers](https://github.com/obra/superpowers) | MIT | 完整软件开发方法论：spec 先行、分块确认、可组合 skill 集（支持 14 种 agent CLI） | 偏重流程约束，与你们已有 AGENTS.md 纪律可互补；可选 |
| **skill-creator** | anthropics/skills（官方） | Apache-2.0 | 创建/迭代你自己的 skill | 想自定义时用 |
| **mcp-builder** | anthropics/skills（官方） | Apache-2.0 | 构建 MCP server | 将来接 MCP 工具时用 |
| **find-skills** | [vercel-labs/skills](https://github.com/vercel-labs/skills) | MIT | 发现并安装社区技能 | §F1 插件自带 |

## 6. 管理工具 / 市场（装技能前先看这里）

| 工具 | 来源 | 许可 | 干什么 | 推荐度 |
|---|---|---|---|---|
| **dsh-plugin-capabilities** | npm `dsh-plugin-capabilities`，[qinyre/dsh-plugin-capabilities](https://github.com/qinyre/dsh-plugin-capabilities) | MIT | DSH 设置页新增「技能与 MCP」分区：技能目录可视化（新建/编辑/删除/开关）、GitHub 仓库/本地目录技能导入、MCP 服务器管理、市场精选一键安装；**自动扫描 `~/.claude/skills` 与 `~/.codex/skills`（你 Codex 的 5 个技能装完即见）** | ⭐ **建议第一个装** |
| **dsh-skills-marketplace** | npm `dsh-skills-marketplace`，[cxdyun/dsh-skills-marketplace](https://github.com/cxdyun/dsh-skills-marketplace) | 待确认 | 按「仓库地址+分支+稀疏路径」拉取技能，装进 `~/.dsh/skills`，设置页按插件维度管理 | 备选 |
| **awesome 精选集** | [seb1n/awesome-ai-agent-skills](https://github.com/seb1n/awesome-ai-agent-skills) · [YangsonHung/awesome-agent-skills](https://github.com/YangsonHung/awesome-agent-skills) · [charlieviettq/awesome-agent-skill](https://github.com/charlieviettq/awesome-agent-skill) | 各自条目不同 | 海量技能索引（各自条目不同） | 找灵感用 |

---

## 7. 推荐安装顺序（等你需要时）

1. **现在就能做（0 成本）**：装 `dsh-plugin-capabilities` → 你 Codex 里已有的 `playwright`、`pdf`、`hatch-pet` 等 5 个技能立即进入 DSH，无需迁移。
2. **答辩 PPT 时**：anthropics `pptx`（主力，Apache-2.0）+ 需要视觉冲击就上 `guizang-ppt`（AGPL，内部使用无碍）。
3. **写论文时**：anthropics `docx` + `academic-research-skills`（注意 CC BY-NC 非商用）。
4. **WebUI 改版时**：anthropics `frontend-design` + `web-artifacts-builder`；要大改视觉方向再装 `@w3kits/plugin-opendesign`（工作台，较重，装前审查）。
5. **想自定义 skill 时**：anthropics `skill-creator`。

## 8. 注意事项

- **许可证**：guizang-ppt 是 AGPL-3.0（强 copyleft，衍生分发需开源）；academic-research-skills 是 CC BY-NC（禁止商用）。答辩/毕设/内部使用均无碍，对外发布需注意。
- **兼容性**：所有清单项都是标准 SKILL.md 格式，DSH 直接兼容；但依赖特定 CLI 的技能（playwright、poppler、python 库）需先装对应工具。
- **第三方插件风险**：`@w3kits/plugin-opendesign` 是第三方打包（版本 0.1.30 很新，上游仓库迁移中），含完整 Web 应用 + webcontainer 运行时，体积大；安装前建议审查，先在测试会话体验。
- **你的 Codex 材料现状**：自建技能在 `~/.codex/skills/`（5 个）；OpenAI 官方插件（presentations/documents/pdf）在 `~/.codex/plugins/cache/`，**依赖 Codex 专属 API 不可直接迁移**，但其资产（26 个 Grid 布局模板、design_tokens.json、ooxml 脚本）可手工复用。
