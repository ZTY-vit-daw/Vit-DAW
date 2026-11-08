# RACK-PARAM-PERSIST-FIX-1：rack 内插件参数持久化修复（VitApp C++——320 揭出的产品缺陷）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[320 续批取证](../blocked/2026-11-08-PLUGIN-CONTROL-EVAL-PROQ3-FREE-1.md)——A/B 判别铁证：rack 内 Pro-Q 3 Band1 Gain 0.7 写入在靶→save/open→回默认 0.5×三次；track 级 Volume&Pan -18.08dB 过重载原样保留=缺陷限定 rack 内插件；[forensic_persistence_20261108 双报告](../runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/forensic_persistence_20261108/)）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 321；目标仓库=VitApp C++（保存/写入 flush 面）；P1 **产品缺陷**（不只影响实验：一切 rack 内插件参数调整重载即丢）/ 0.5 天；L2 / PC；零栈单测先行+真栈 probe 验收

## 目标

1. **修复**（主管裁定方向：**写时直 flush 为主+保存路径 rack 遍历兜底，两腿都做**——最小干预与全覆盖兼得）：
   - 写时：`PluginRackControlService.cpp handleSetPluginParam` ~2051-2054 写后 flush 目标 rack 插件（`RackType::flushStateToValueTree()` 在位可用）；
   - 保存路径：`VitHeadlessService.cpp ipcSaveEncryptedProjectToPath` ~1316-1320 / `saveProjectToDefaultXml` ~671-675 保存前 rack 遍历 flush（覆盖非写路径 mutation）；
   - 锚点背景（320 亲读）：`Track::flushStateToValueTree` ~49-53 只级联本轨 pluginList（~277-280），rack 内插件挂 RackType 自身轨表不被触达——修复不改动该语义，在两腿显式补 rack 面；
2. **验收**：320 的 forensic probe 复跑=**rack 内参数过 save/open 存活**（0.7 重载后仍 0.7）+track 级回归维持+全量构建 exit 0+既有持久化/工程测试零回退；
3. **解锁 320**：本卡合入后 320 续批（装置全就绪，一次栈窗口完成 T7×3+对账+用户 GUI 终审交付物）。

## 约束

- 文件域=VitApp 保存/写入 flush 面±测试；不触 tracktion_engine（`RackType::flushStateToValueTree` 在位用）；C++ 构建走 301 同款 cmake 配置；真栈 probe 复用 320 取证脚本（只读复用）；
- §8 跑前成文；插桩留痕保留。

## 停止条件

修复发现 rack 状态树结构不允许外部 flush（需 engine 改动）→取证上交转裁定（扩域须授权）。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
