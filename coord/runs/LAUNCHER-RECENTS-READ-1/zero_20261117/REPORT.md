# 401 LAUNCHER-RECENTS-READ-1 零栈回执

- 分支：codex/converge-entry-388；领取HEAD 779477ea；领取时388的5文件修改保留，401独立暂存。
- GET /agent/launcher/recent-projects 复用kernel只读recent helper及既有卡片schema；不依赖chat流门。正常含exists/治理directive；成功空数组；核不可达503，不伪装空态；其他method405。
- 新测试覆盖startup/entered-project、去重、exists、authority/工程identity/launch事实/goal maps不变、fixture内容不变、空态/错误/method。无agent/LLM装配的Server可完成handler，kernel只收到get_recent_projects一次。
- 命令（agent/）：`go test ./internal/chat -run 'Test(Launcher|ProjectFlow)' -count=1` exit0；`go test ./internal/chat -count=1` exit0（164.057s）；`go build ./...` exit0。
- 命令（agent/webui/）：`npx vitest run src/api/launcher.test.ts` exit0，7tests；`npm run test` exit0，59files/713tests（含388未提交UI骨架）。API错误/无效payload不被转换为空列表。
- 原始日志：本目录converge401-go-targeted.log、converge401-go-chat.log、converge401-go-build.log、converge401-api.log；前端全量日志在388零栈工件。
- 未启动VitApp/agent/Godot/VitShell，未占用真实栈。真实端点与telemetry零调用、回访列表/磁带打开/异常态仍须388真栈腿；本回执不作最终验收。
