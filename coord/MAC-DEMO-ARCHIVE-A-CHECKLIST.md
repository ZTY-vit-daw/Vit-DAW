# mac 演示存档 A 操作清单（2026-09-28 下午冻结线，9-29 中期检查用）

> 目标：建立一套**自洽只读**的演示环境副本，检查当天从它拉起；mac 主工作区此后照常开发互不影响。

## 前置确认

1. 夜池当前卡（TIM-KERNEL-DISCLOSE）告一段落或暂停（构建跑完再拷，避免拷到半写文件）。
2. 磁盘空间 ≥1.5GB。

## 步骤（约 10 分钟）

```bash
ARCHIVE=~/vit-demo-checkpoint-20260929
mkdir -p $ARCHIVE

# 1. 前端工程副本（纯文件拷贝，不用 git）
cp -R ~/Documents/vit-daw-frontend $ARCHIVE/frontend
# 清掉拷贝里的运行时状态（保留扫描所需）
rm -rf $ARCHIVE/frontend/.vit_agent $ARCHIVE/frontend/.vit_derived

# 2. 内核二进制+运行文件（演示用的那套：mac 构建的含 HYGIENE 修复的 5fb4585b 或更新稳定版）
#    位置按你的演示启动链实际取（VitApp/build/... 或 staging 目录），连同运行所需资源一起拷
cp -R <你的内核运行目录> $ARCHIVE/runtime

# 3. Workspace 快照（扫描表 719+候选面 83 的状态在这里）
cp -R <主仓>/VitApp/Workspace/Settings $ARCHIVE/workspace-settings-snapshot

# 4. 清单（版本锚定）
cd $ARCHIVE && {
  echo "created: $(date '+%F %T')"
  echo "frontend_head: $(git -C ~/Documents/vit-daw-frontend rev-parse HEAD)"
  echo "main_repo_head: $(git -C <主仓> rev-parse HEAD)"
  echo "kernel_sha256: $(shasum -a 256 <runtime 内核 exe> | cut -d' ' -f1)"
} > MANIFEST.txt
```

## 冻结前核对（关键，约 5 分钟）

**从存档 A 拉一次栈**，确认三件事：

1. **进程路径**：Godot/内核/agent 进程的可执行文件路径都在 `$ARCHIVE` 内（`ps aux | grep -i vit` 核对）——不在=拉起路径解析不自洽（前端 start_page 的 exe 路径是绝对路径/环境变量形态时，需在存档内配启动脚本先设 `VIT_DAW_DEV_ROOT=$ARCHIVE/...`）；
2. **扫描表**：内核起来后插件表条数 ≈719（Settings 快照生效，不是重新扫描）；
3. **新建工程可用**：从启动页新建一个测试工程，能进主界面。

三件都过 → 存档 A 冻结（此后不再动它）。有任何一件不过 → 来找我排查路径解析。

## 检查当天

- 从 `$ARCHIVE/frontend` 拉起 Godot → 新建工程 → 演示；
- 主工作区不要同时跑重任务（端口/性能争抢）；
- 万一异常：重拉一次即可（存档只读，坏不了）。

## 检查后

存档 A 保留（12 月答辩彩排可复用或重建）；mac 主工作区恢复全速开发。
