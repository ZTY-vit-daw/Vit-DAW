# M2-TIMEFX-1：reverb/delay 语义控制面（时域效果器控制·语义侧）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=用户三线同步：M2 时域=混响/延迟类效果器；MIX-CAP-RECON §2.B/§3 C3 行）
- 派发确认：已确认（用户 2026-10-11 晚窗同步）
- 验收负责人：GLM 主管决策流
- 池序 71；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / **MIXLAYER-M1-3 合入后领取**（C3 观察侧与控制侧同域，先观察后控制）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
- **定义钉死（用户裁定）**：时域效果器=**混响、延迟**类；动态族（compressor/de-esser/transient/limiter）属 C2 已完成，不入本卡。

## 目标

reverb/delay 两族按 C2 动态族现役模式接入语义控制面：

1. **域表扩展**：d1s1_domains.go 四族模式（SemanticIntentCoverage 轴）复制出 reverb/delay 两族（语义意图→插件参数映射，如 reverb=decay/pre-dry/wet、delay=time/feedback——词表设计执行侧定，风格参照 attestation coverage 现有轴）。
2. **参数抓取/inspector 面**：MIXLAYER 勘察结论"plugin_grabber 无 reverb/delay inspector"——领取时 grep 实际落点（勘察名词可能非代码符号名），把 reverb/delay 参数快照纳入抓取面。
3. **写回链接入**：既有 set_plugin_param / plugin.set_params_batch（readback+base_revision）通道直接复用，零新协议。
4. **自由态可达**：话术/意图路由进自由态循环（fastpath 注册清单完整性契约模式），非编排独占。

## 文件域

chat/capability seam + 域表文件（d1s1_domains.go 同级）+ 对应单测；与 M1-3 领取记录核对文件域。

## 验收标准

- 单测：域表两族映射+越界 fail-closed；「reverb/delay 语义控制在自由态循环内可达」冒测断言。
- 真栈烟测（AGENTS §5，扩展现有 ps1 体系）：reverb 或 delay 一族语义话术→确认→写参→回读一致，exit 0。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

域表/写回链锚点与卡面不符（上游已变）→ 锚点上交。

- 领取：2026-10-12 PC 晚窗 / origin/main=813cbf5850225d3edc0f70ba609a6f137d54a27e / owner=GLM-5.3 执行流（ZCode PC 会话，M2 时域线）/ 分支=port/m2-timefx-1 / worktree=D:/Vit_DAW_wt_m2_timefx1 / 领取提交=（本提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
