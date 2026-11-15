# UI-MSG-SHADOWBOX-1：用户消息阴影框标识——两面对齐实施+SPEC-V5 修订

- 发卡：GLM 主管决策流 / 2026-11-15（依据=[用户裁定：用户消息阴影框](../../decisions/2026-11-15-user-message-shadowbox-ruling.md)）
- 派发确认：待用户触发领取；**排 UI-SPRITE-MOUNT-1（361）合入后领取**（渲染烟测断言文件域不重叠）
- 验收负责人：主管决策流 + 用户目检（用户消息来源一眼可辨）
- 池序 362；P1 / 0.25 天 / 依赖=361；PC；零栈（浏览器渲染面）

## 目标

1. **SPEC-V5 修订**：§6 对话消息「用户消息右对齐无色块」→「右对齐+阴影框标识」；S5 消息状态板补标记态；修订理由引用裁定档案。
2. **launcher surface 实施**：`.v5-launcher-msg--user`（launcher.css:227）加阴影框——复用 v5.1 空间修订「1px 灰边+柔影」令牌（窗口/宏卡同款，不新造体系），可选极浅底；`--assistant` 维持无框。
3. **project surface 对齐**：工程面用户消息同款标记（锚定 App.tsx 消息渲染+对应样式位，与 launcher 一致）。
4. **渲染断言**：test:render 断言用户消息元素带标记样式类/ computed 边框+阴影非空，两面各一；防回归。

## 非目标

- agent 消息样式变化（维持现状）。
- 磁带 icon 三态、图标体系统一（361 域/后续裁定）。

## 文件域

`agent/webui/src/styles/`（launcher.css/v5.css 或等价样式位）+ `agent/webui/src/App.tsx`（工程面消息样式类，如需）+ `agent/webui/design/main-window-v5/SPEC-V5.md` + 渲染烟测断言文件 + coord 工件；领取时回填精确清单。

## 验收标准

三门全绿（vitest/build/test:render）+两面渲染断言在卷+SPEC-V5 修订段落引用裁定档案+用户目检通过。

## 停止条件

阴影框令牌与现有消息排版冲突（行高/换行/滚动条抖动）且简改不可收敛→取证上交裁定。

## 领取/回执/验收：待回填
