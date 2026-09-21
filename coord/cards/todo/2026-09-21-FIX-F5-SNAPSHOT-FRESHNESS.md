# FIX-F5-SNAPSHOT-FRESHNESS：声学桥快照新鲜度标注（前台铃机制）+ 原子发布

- 优先级 / 预估 / 依赖：P1 / 1 天 / 取证定责 FORENSIC-45-FAMILIES（两次实证+自收敛证据在案）
- 模型分级：L2 / GLM-5.3
- **背景**：⑤ 声学桥断言两次撞上 band_energy_summary.request_id=mixboard_时间戳 vs latest_request=kernel_prepared_* 的分叉（统计 ⑤R4 + 修复会话 ⑤R2）；事后快照自收敛（kernel_prepared/ready）证明是瞬态竞态窗。两条写路径（kernel L3 备谱 / mixboard 戳记）非原子更新同一快照。用户裁定方向：**双层各自在写入时自写新鲜度标注（前台铃机制——按铃即刷新，不按即未刷新）**；永久单侧缺口有先例（投影清单 §9 masking unavailable / mixed-tap withheld，按显式 missing+理由处理）。
- 目标：
  1. 代码定位：mixboard 戳记与 kernel_prepared 两条写路径及快照装配点（预期 agent/internal/mixboard + bridge/内核装配面，定位后收窄文件域）
  2. 写侧①（标注）：每个特征行携带所属 request_id + 新鲜度标注——当新 latest_request 发布时旧行必须被标 stale/superseded（标注由写入层自己写，不做读取侧推断）；贯彻 CCB"stale 原样保留不升级 readiness"纪律
  3. 写侧②（原子）：快照整体 copy-on-write 发布（表头+行同瞬替换），消除瞬态分叉窗
  4. 读/断言侧：分叉时一次有界重读区分瞬态/永久；断言对象从"不存在分叉"改为"不存在**未标注**的分叉"；永久分叉以"行 X 属旧请求 Y 且未标注"披露
  5. 红测试：构造双写竞态序列，修前断言红（未标注分叉）/修后绿（标注或收敛）
- §8：断言语义变更=本卡目标之一（非削弱：从"无分叉"到"无未披露分叉"，更诚实不是更宽松）；真栈 ⑤ 1 轮该段验证
- 文件域：定位后收窄（预期 agent/internal/mixboard、装配层，可能涉 VitApp 桥面——跨层则拆子卡）；scripts/run_vit_product_path_smoke.ps1 断言段
- 验收：①双写路径定位报告；②红测试；③回归绿；④⑤ 真栈该段过
- 停止条件：装配点在内核 C++ 深处且改动影响渲染主链 → 上交拆卡
- 领取：
- 回执：
- 验收：
