# GENAPI-REALVERIFY-1 运行工件（genapi 真源端到端验证）

- 会话：GLM-5.3 L2 Mac 执行侧 ZCode 会话 / 2026-10-16 晚
- 基线：5ba87c42（= origin/main 领取点）／实现分支 port/genapi-realverify-1 @ worktree Vit-DAW-genapi-rv1
- 受测二进制：内核 sha256 `099327af134451825b408efd549a353fe18a778cf98b2ee8e6abf6428d01d508`（本卡 worktree 从基线源码新建构建，tracktion pin b439749 与主仓 checkout 一致）；vitagent sha256 `93fd00b28f5e27af0af0a74d5101217db8a295e052c1a046e96c233f5081ea0a`（同树 go build）

## 结论（如实）

**真源生成调用未执行——生成源尚未选定**。用户 2026-10-16 21:49 在执行会话中确认："目前的确还没有明确的音乐生成源，我们还没定"。卡面发卡依据"key 已配好"被该陈述推翻：`~/.vit/genapi_providers.json` 不存在、无 `VIT_GENAPI_*` 环境变量（`~/.vit/config.json` 为 agent 的 deepseek LLM 配置，非音乐生成源）。

§8 失败分类：**no_key 环境阻塞**（非 provider 真实错误；无任何外呼、零 quota 消耗；"同类失败两次即停"不适用——没有生成调用轮次）。卡面按 PROTOCOL §2 移入 blocked/，等决策流完成生成源选型后凭配置重派/续跑。

## 本卡实际交付（真源选型定后即可续跑）

1. **脚本最小扩展**（`scripts/gen_api_adapter_smoke_mac.sh`，本卡唯一代码面改动）：新增 `--real-source PROVIDER_ID` 模式——跳过模拟器、读用户级配置（默认 `~/.vit/genapi_providers.json`，`--provider-config`/`VIT_GENAPI_CONFIG` 可覆写）、单次真源 `gen.generate` 调用（脚本内零重试，quota 不烧第二次）、诚实失败分类（`provider:<error_class>:<http_status>` 入 summary.failure_kind）、key 全程不打印 + 运行文件就地脱敏 + 终扫零出现门（`realsource_secretfree_artifacts`）。真源门：providers 列表 SecretFree / host 轨 / 生成 ok / 产物 RIFF 魔数 / ingest 落轨（clip_id/asset_ref/active_take_id）/ provider 任务终态 done 类（status_seen 末值或 sync 单发说明）/ project.state 可寻址 / 命令面存活。模拟器 21 门路径零语义改动（仅条件包裹）。
2. **回归轮 run1**（本目录 run1-sim-regression-20261016-2153/）：扩展后脚本模拟器模式 **exit 0，21/21 门全绿**——重构未破坏 GEN-API-ADAPTER-1 链路；内核为当前 main 树新构建（含 tempo-persist/automation/timefx 等基线后合入，非旧二进制复用）。
3. **no_key 实证轮 run2**（run2-nokey-20261016-2154/）：真源模式在无凭据条件下 **exit 1 + `env:no_key` 分类**，未启栈未外呼——卡面"区分 no_key 环境阻塞 vs provider 真实错误"的分类机制实证。
4. **用户级模板**：`~/.vit/genapi_providers.json.template`（不入仓；三种 dialect 占位条目 + 选型指引 `_read_me`；复制为 `genapi_providers.json` 并填值后生效）。

## 真源就绪后的续跑命令（记录备查）

```bash
# ~/.vit/genapi_providers.json 填好后（只需 provider id）：
./scripts/gen_api_adapter_smoke_mac.sh --real-source <provider_id> \
  --kernel-bin <VitApp/build/VitApp_artefacts/Debug/VitApp>
# 验收：exit 0（providers SecretFree+生成+RIFF 产物+ingest 落轨+终态 done+可寻址+存活+零 key）；
# provider 失败：exit 1 + summary.failure_kind=provider:<class>[:<http>]，同类两次即停上交。
```

## 证据与边界

- 端测边界：agent HTTP 工具面数据断言（/agent/invoke + /health）；无 webui/前端改动；真源音频质量不评估（留后续 AUDITION 卡）。
- 零生产代码：`agent/` 对基线 5ba87c42 **零 diff**（git diff --stat 实证）；全量 `go test ./...` 以此声明豁免（基线合入时 95 包 0 FAIL）；本卡实际跑 `go test ./internal/genapi/ -count=1` = ok。
- 栈纪律：占用登记 48a54f72（占用前 5555/5556/5557/7878 监听=0、进程=0 亲核）；run1 脚本自 teardown，run2 未启栈；21:54 复核四端口 free、VitApp/vitagent 进程=0。
- 工件零 key 材料：本目录全部文件不包含任何凭据（本卡从未接触真实凭据）。
