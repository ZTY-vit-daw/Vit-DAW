# 388 CONVERGE-ENTRY-1 零栈实现回执

- 基线779477ea；依赖401独立提交b91a8ad9；分支codex/converge-entry-388。未修改主检出/协调状态。
- 生产文件仅App.tsx、ConvergedLauncher.tsx、converged.css、ProjectSurface.tsx、project.css（5个）。新壳原recents/new/open桥保留；列表改读401 GET，两态不再自动发chat。
- 主管裁定的“查看启动页”=工程管理视图，DAW/内核/绑定保持打开。当前工程名+返回工程显式在场；工程实例隐藏保留输入/对话，隐藏时不消费管理页治理。宿主vit_surface仍权威，宿主launcher清除工程标记。内部往返用既有vit_surface_ack登记，不新增launcher五type。
- 管理视图关闭自由composer/建议词，标注“继续当前工程的对话，请返回工程”；工程切换仍经原桥治理，不将管理卷伪称工程卷。
- 两态左下设置复用现有SettingsPage真实config读写；本卡自动化仅GET读取，未保存配置。原生dialog圈定焦点、Escape关闭并归还焦点。运行状态与工程状态读取真实既有端点。
- 自验：`npm ci` exit0；`npm run test` exit0（59files/713tests）；`npm run build` exit0；`$env:VIT_RENDER_SMOKE_OUTPUT_ROOT='<本工件>/render2'; npm run test:render` exit0（全部原有段+新增388段PASS）。无GUI/真实栈进程。
- 新渲染旅程：真实App入口+HTTP/宿主mock；两态设置左下/Escape焦点归还、工程→管理→工程输入保留、GET真schema列表/磁带治理directive、GET503错误不冒充空态、宿主launcher清理、双向ack、零chat与零config写、420/1440px设置可达。管理截图在render2/CONVERGE-ENTRY-1/。
- 首轮render exit1：管理页mode出现后未等待异步recents ready，断言过早；保留render.log与首轮report。补显式ready等待后第二轮exit0，未弱化产品判据。其他原有段两轮均PASS。
- 测试噪声：vitest原有VexFlow/jsdom无canvas stderr（原基线706tests同样exit0）；原始完整输出以vitest.zip保存。构建大chunk提示为已有边界。
- 端测边界：headless只证明DOM与请求/桥契约；尚未证明真实宿主往返PID/绑定不变、GET端点实际数据、真实治理取消和telemetry零调用。待独立验收装置commit及主管放行真栈；不作为最终验收。
