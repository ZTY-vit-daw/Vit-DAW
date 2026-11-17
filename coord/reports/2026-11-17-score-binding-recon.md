# 392 只读勘察与拆卡裁定（2026-11-17）

- 证据来源：`/root/score392_recon`（Sol/high），代码基线116f2215；零文件改动、零测试、零真实栈。主管采纳拆分为397/398/399/400，父392不重复领取。
- 现成数据：`Workbench.tsx:134/141`有桥工厂/面板挂点；`ScoreWorkbenchPanel.tsx:153-178`读取ui/state轨/clip但混列所有类型、只读一次；`shadow.go:690`保留track_id/name/type/clips；`CommandDispatcher.cpp:943`披露clip_type/start/end/length/revision；`server.go:1277/1310/2350`返回tracks+ui_context+selected_track。
- GUI→agent已接通：前端`Track_Row_Controller.gd:2182`→`timeline_dock_adapter.gd:1656`→`project_command_service.gd:87`；`vit_dock_root.gd:294/707/747`每0.75s向`/agent/ui/context`披露选择，agent `server.go:1398/2552`缓存允许selected_track_id/selected_clip_id等。反向`track.select`在`catalog.go:1197`、`harness.go:2830`，GUI mutation消费者`vit_dock_root.gd:3605`；反向端到端未实测，不许凭注册就宣告同步。
- 绑定风险（代码审阅，非实测复现）：面板507切clip直接setClipID，206 reload覆盖buffer，失败保留旧buffer却activeKernel已换；463 apply使用新activeKernel，480异步回包可覆盖后续选择。首卡必须绑定成功加载目标身份并处理dirty/busy/load failure/generation；不改记谱model/render/derive/edit核心。
- `kernel.ts:74/99`已支持clipID+trackID，162 reload、233 confirmed apply+246 readback可复用。
- 草稿库前提未成立：`VitMediaPoolManager.h:11/cpp:26/84/146`是生成音频/进程内map/从已落轨clip重建；`GeneratedAssetService.cpp:159/187/219`要求轨、AudioFile、立即落位。`artifacts/extract.go:547`支持mid路径记录、`artifacts/store.go:30`清单在.vit_agent目录，但不是工程包可携带未落轨MIDI库；`projectpackage/project_package.go:101/254`未见clip库专用prepare/restore。400先定工程包生命周期与兼容，禁止直接补成大卡。
- 转正末端可用`midi.import_file`（catalog1239 RiskConfirm；MidiService.cpp1784 track/start_time_beats，1903创建clip），但1884长度max(lastNoteEnd,4拍)，1880拒全休止；尾部空白/显式时长需契约先行。
- 验证：复用score_workbench_smoke.mjs J2:932、J1/J3:956；旧data-score-clip-select断言981需迁移，不能删旅程。GUI实际选择装配不能用POST/ui/context替代。跨工程缓存无freshness信息，绑定至少校验当前工程身份+轨/clip存在性。新库若落地须旧工程无库=空、未知schema策略、save-as/fork/export/移目录/重开回读测试。
