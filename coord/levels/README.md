# coord/levels/ —— 乐局内容包目录

- 语义：一个子目录 = 一局乐局的内容包（乐局卡 `level.yaml` + 素材 `assets/` + 人格块 + 工具 `tools/`）。内容工件属协作面，随 `coord/` 卡池走（发卡/验收见 `coord/cards/`）。
- 格式：乐局卡 schema = `vit.level_card.v0`；权威定义 = [first-run-twinkle/DESIGN-DRAFT-2026-11-06.md](first-run-twinkle/DESIGN-DRAFT-2026-11-06.md) §1 + 首个定稿实例 [first-run-twinkle/level.yaml](first-run-twinkle/level.yaml)。素材清单 schema = `vit.level_assets.manifest.v0`（各包 `assets/MANIFEST.yaml`）。
- 判定器枚举（首版 3，闭集）：`note_sequence_match` / `pitch_set_match` / `rhythm_match`。交互事件与结构谓词枚举、校验门拒绝路径 12 条见设计稿 §1.2/§1.9（280 EDU-LEVEL-RUNTIME-A 实现校验门，与 279 回执互引）。
- 索引：`first-run-twinkle`（卡 EDU-LEVEL-CONTENT-1 / 池序 279；先行稿 2026-11-06，定稿 2026-11-07）。
- 工具：各包 `tools/` 为该包素材的可复现生成/校验脚本（Python stdlib only）；命令与设计决策见各包 `assets/MANIFEST.yaml` produced_by。

## 课程设计与教材资料（尚非运行包）

- [高中《音乐编创》第二单元「旋律写作」](../lesson-designs/music-creation-unit2/README.md)：用户要求持久整理的教材定向文本、原页图像、题面/谱例索引、精选节奏数据，以及乐局课程第一稿和设计 YAML。后续会话先读该目录，不重新遍历整本 PDF。设计格式/材料/控件接线尚未完成，不作为已安装乐局列入上面的运行包索引。
