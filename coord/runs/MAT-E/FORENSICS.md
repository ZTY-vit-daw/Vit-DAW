# MAT-E 真栈烟测取证与运行索引

## 最终通过 run（门②证据）

`materialize_shadow_smoke_20260927_223003` — **PASS，exit 0**（HEAD=60651ed4 工作树，
未用 -SkipBuild，agent 二进制含本卡改动；内核=pcverify1 增量构建 sha256=8CC661F5...）。

断言面：
- S1 mode=on 启动行在案（kinds=tom,acp,dom store=memory）；
- S2-on miss 轮：响应 actual_cost_class=compile + recomputed=1，on_round 行
  `dom=miss ... backfilled=1`（回填发生）；
- S2-on 命中轮：响应 index/0，on_round 行 `dom=hit ... backfilled=0`（命中装配
  发生——dom finalize 被跳过，服务的是回填世代实例）；
- S2-on 变更轮：set_volume 失效后响应退回 compile/1（stale 行不供读）；
- S3 脏传播：3 条 receipt 行+shadow_round inv>0；
- S5 回退演练（on→shadow→off，agent -RestartAgent 重启、内核/UI 不动）：
  shadow 腿=mode=shadow 启动行+observe 响应无物化字段+shadow_round 照常；
  off 腿=零 [materialize] 行+observe 正常+响应无物化字段（现状恢复）。

## 红运行取证（两个脚本侧缺陷，Go 侧零缺陷）

- `materialize_shadow_smoke_20260927_221627` — 首次 on 态运行崩于 S2-on：
  PS5.1 下 `[ordered]`（OrderedDictionary）条目经 `@()` 包装入数组再点赋值进
  `[ordered]` report 会抛 ArgumentException"参数类型不匹配"（同毒组合亦使
  ConvertTo-Json 崩溃）。隔离复现与修复：采集条目改 `[pscustomobject]`、report
  赋值改 `.ToArray()`。**Go 侧运行时行为在该 run 已被证实正确**（on_round 行
  dom=miss backfilled=1 → dom=hit 序列完整在案）。
- `materialize_shadow_smoke_20260927_222737` — S3 假红：hit-probe 轮产生的
  shadow_round 行未入收集集，change 轮 Wait-NextShadowRoundLine 把陈旧 probe 行
  当作变更轮行（无 inv>0）。修复：probe 循环同步收集 shadow_round 行。

其余中间红运行为同缺陷重复触发与一次二进制锁环境中断（agent 进程未退净），已
从工件目录清除；保留上列三个 run 为完整诚实记录。
