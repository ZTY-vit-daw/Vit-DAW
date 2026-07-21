# Mix Benchmark Manifest v0

日期：2026-06-29  
用途：Agent Action Workflow v1 与自动混音闭环的本地真实工程素材登记。

## 使用纪律

- 不把音频素材加入 git。
- 不在代码、prompt、规则里硬编码封存测试集轨道名。
- 训练/开发集可以反复用于调试通用逻辑。
- 封存测试集只在关键阶段验收时运行。
- 封存测试失败时，只记录失败类型；修复必须回到通用逻辑或训练/开发集，不直接为封存测试集打补丁。

## 训练/开发集

名称：`sattelites`  
路径：`E:\BaiduNetdiskDownload\yingge - sattelites tracks out`

已盘点元数据：

- 文件数：61
- 扩展名：`.wav`
- 总大小：约 3.74GB
- 音频格式：48kHz / 24bit / stereo
- 时长：约 228.571 秒
- 用途：日常开发、smoke、规则迭代、Project Context Pack 与 Action Workflow v1 闭环调试。

## 封存测试集

名称：`Weekend Lover`  
路径：`E:\BaiduNetdiskDownload\yingge - Weekend Lover tracks out`

已盘点元数据：

- 文件数：104
- 扩展名：`.wav`
- 总大小：约 7.22GB
- 音频格式：48kHz / 24bit / stereo
- 时长：约 258.641 秒
- 用途：held-out test set，只用于阶段验收，验证系统是否能泛化到更复杂真实工程。

## Phase 1 Smoke 语句

训练/开发集优先使用：

1. `先只观察这首歌的人声和伴唱关系，不要修改工程。`
2. `帮我让主唱更靠前一点，但先给我确认。`
3. `可以，执行。`
4. `给我 AB 结果。`

封存测试集只在阶段验收时使用同类语句，不根据失败结果写测试集特例。
