# melody-drum-v1：课程固定健康音源模板（SRC-TEMPLATE-1 / 池序 373）

- 形态：`template.v1.xml`（clear-XML Tracktion EDIT，280205 字节）+ `template_manifest.json`（vit.course_template.v1）。
- 内容：2/4、80 BPM、C 调；5 轨对齐 design.v1.yaml workspace.tracks（参考与示范/我的旋律/辅助律动（鼓）/我的八小节主题/我的成曲版本）；旋律/主题/鼓三轨 Surge XT（factory init patch state 逐字固化，sha256 在 manifest）。
- 鼓预设：`groove-2-4-basic`——2/4 四小节辅助律动一套（GM 打击键位 36/38/42 记谱，channel 10；音色=Surge XT chromatic 合成，记谱/音色分离声明见 manifest）。
- 占位 scaffold：`rhythm-lab`（旋律轨 8 拍占位音高 60）+ `eight-bar-theme`（主题轨 16 拍占位音高 60）——学生「占位音高→赋音高」编辑即 DG1-B B4 面。
- 血统与纪律：
  - Surge state blob=309 E4 fixture 逐字复制（非静音对照 -12.6 dBFS 离线/-10.97 dB 实时在卷：`coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/`）；Waves 全系不采纳（RT-SILENT）。
  - 本目录是模板资产不是工程状态——禁止在真实栈打开后保存回写本文件；烟测/预演一律复制到隔离 run 目录（沿 `coord/fixtures/legame/FIXTURE.md` place-copy 纪律）。
  - 模板漂移以 manifest `template_sha256` 为准核对；再生成用 `python scripts/src_template_build.py`（确定性：同输入→同输出）。
- W01 衔接：创建乐局时 timing 字段（time_signature/key_signature/tempo_bpm）如何落到本模板的命令缝与固化位，见 manifest `w01_seam.field_map`（契约=`docs/LEGAME_DESIGN_CONTRACT_V0.md`；绑定卡=W01-C）。
- 过关面：DG1-B（`docs/STACKGATE_DG1_DG3_V1.md` §3.2，B1-B7）；装置=`scripts/src_template_smoke.py`（`dev_agent_smoke.ps1 -SrcTemplateSmoke` 挂腿，berth 形态）。
