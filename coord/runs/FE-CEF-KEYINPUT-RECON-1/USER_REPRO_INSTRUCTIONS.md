# 用户单轮复现采集指引（FE-CEF-KEYINPUT-RECON-1 [等待用户] 项）

目标：在 CEF 容器内按底账画像复现一次输入失效，三层时间线自动留档；随后做产品设置页
复现与 Safari 直开对照。全程约 10 分钟。

## 0. 前置确认（30 秒）

```bash
# 探针服务是否在（期望 200）：
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7879/
# 若非 200，重启探针服务：
cd /Users/timozty/Documents/Vit-DAW-fecef1/agent/webui/input_probe
nohup ../node_modules/.bin/vite preview --host 127.0.0.1 --port 7879 --strictPort \
  > /tmp/vit_input_probe_preview.log 2>&1 &
```

## 1. R1 轮：CEF 容器内探针页（核心采集）

1. **退出当前打开的 Godot 编辑器**（环境变量必须经启动 shell 继承，见 AGENTS §5）。
2. 在**同一个终端**执行（整段粘贴）：

```bash
export VIT_CEF_DEBUG=1 VIT_CEF_FOCUS_LOG=1 VIT_AGENT_WEBUI_URL=http://127.0.0.1:7879/
cd /Users/timozty/Documents/vit-daw-frontend
/Users/timozty/Applications/Godot.app/Contents/MacOS/Godot --path . \
  2>&1 | tee /Users/timozty/Documents/Vit-DAW-fecef1/coord/runs/FE-CEF-KEYINPUT-RECON-1/godot_user_R1.log
```

3. 在 Godot 编辑器里**运行工程**（F5，按平时手测途径进入目标工程，让 agent 正常拉起）。
4. 打开 **Ask Vit**（webui 窗口）——这次它会加载**探针页**（右上角有黑底时间线浮层即对）。
5. 按探针页面的四步序列操作（每步之间点一下对应的「标记」按钮）：
   - ① 点击 **A 框**（原生输入框），用键盘输入 `abc`
   - ② 点击 **B 框**（React 受控框），输入 `hello`
   - ③ 点回 **A 框**，再输入 `xy`
   - ④ 点击 **B 框**，按 **⌘V** 粘贴（粘贴内容随意，可先在别处复制一段文字）
   - 观察并记住：哪一步开始打不出字/粘贴无效，浮层里 keydown/input 是否还在增长。
6. 结束后点「下载 JSON」保存到本 run 目录（若 CEF 内下载无效，点「复制 JSON」粘贴到
   `user_R1_timeline.json`；两都不行也没关系——完整时间线已通过 `[:INPROBE]` 前缀镜像进
   Godot 日志 `godot_user_R1.log`，不丢）。
7. 关掉 Godot（终端 Ctrl-C / 编辑器退出）。

## 2. R2 轮：CEF 容器内产品设置页（真实缺陷面）

同一终端重新启动（**去掉 URL 覆盖**，其余保留）：

```bash
unset VIT_AGENT_WEBUI_URL
cd /Users/timozty/Documents/vit-daw-frontend
/Users/timozty/Applications/Godot.app/Contents/MacOS/Godot --path . \
  2>&1 | tee /Users/timozty/Documents/Vit-DAW-fecef1/coord/runs/FE-CEF-KEYINPUT-RECON-1/godot_user_R2.log
```

进入设置页，按底账画像操作一次（第一次点击的输入框能输入→之后键入与 ⌘V 全失效），
记录失效开始点。Godot 侧焦点链日志（`[VitCefFocus]` 行）与 console 镜像会留档。

## 3. R3 轮：Safari 直开对照（真实键盘，补执行侧 IAB 基线的能力边界）

- 探针对照：Safari 直接打开 `http://127.0.0.1:7879/`，同样四步（真键盘+真 ⌘V）——
  预期全正常；点「下载 JSON」存为 `safari_direct_timeline.json`（或截图浮层）。
- 产品页对照（可选，R2 栈运行中时）：Safari 打开 `http://127.0.0.1:7878/app/` 进设置页
  同样操作——正常/异常如实记录。

## 4. 交回

把以下文件留在 `coord/runs/FE-CEF-KEYINPUT-RECON-1/`：`godot_user_R1.log`、
`godot_user_R2.log`、`user_R1_timeline.json`（如导出成功）、`safari_direct_timeline.json`
（或截图）。执行侧收取后汇总结论并回填卡面回执。

## 备注

- 探针页时间线同时持久化在页面 localStorage，刷新不丢；点「清空时间线」可重置。
- `VIT_CEF_FOCUS_LOG` 也可不重启经 `app/config/debug_flags.json` 热开
  （`"cef_focus_log_enabled": true`，0.8s 内生效）。
- 若 R1 中探针页四步全部正常但 R2 产品页仍失效 → 这本身就是强结论（React 层嫌疑升级），
  原样交回数据即可，不要自行改产品代码。
