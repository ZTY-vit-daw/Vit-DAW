# PORT-A1：ISharedMemorySegment 接口抽象 + POSIX mmap 实现（内核 shm 发布侧）

- 优先级 / 预估 / 依赖：P1 / 2-3 天（PORT_AUDIT 层 A）/ **依赖 A3**（POSIX 实现需 mac 编译验证）
- 模型分级：L3 / GLM-5.3；接口方案（命名/世代/平台映射）有疑义先上交参谋讨论，不自行定契
- 目标：内核共享内存发布抽象为 `ISharedMemorySegment` 接口；Windows 实现 = 现 OpenFileMapping/CreateFileMapping 语义搬迁；新增 POSIX 实现（shm_open + ftruncate + mmap + munmap/close）；3 使用点改写：`VitApp/Source/Service/WaveformEnvelopeBaker.cpp`、`TiledSpectrogramBaker.cpp`、`SharedMemoryTester.cpp`
- **硬约束（A2 验收采纳，违反 = 返工，见 rulings/2026-09-17-A2-pass.md）**：
  1. macOS shm 段名**含前导 `/` 上限 31 字符**（PSHMNAMLEN）——现命名 `Vit_AudioFeature_waveform_<bakekey>_g<gen>_<tile>` 光前缀即 27 字符必然超限；mac 发布名须短前缀或哈希，**≤30 正文字符**
  2. shm 存储按 16KiB 粒度向上取整——`ftruncate` 一次到位即可，尾部空间浪费可接受，勿反复截断（第二次返回 EINVAL）
  3. Go 侧读取契约已冻结（agent 已验收，`shm_darwin.go` posixShmName 仅补前导斜杠）：Windows 用原名、mac 用规约后短名，**两侧发布名映射关系必须显式**（接口内注释 + 回执说明）
- 文件域：`VitApp/Source/Service/SharedMemory*`（新接口 + 双平台实现 + 3 使用点）；涉 `VitApp/Tests/**` 列明；不改 `agent/**`
- 验收标准：① Windows 行为逐项对照零回退（段创建/写入/世代更替/清理路径）；② mac 编译过（A3 就位）；③ SharedMemoryTester 在 mac 真机跑通一次发布 → Go 侧 `shm_darwin` 读回的**跨进程真实验证**（run ID、命令、工件目录入回执）；④ 段名方案与映射关系记录在案
- 停止条件：接口设计需要改 Go 侧读取契约 → 停（契约冻结）；对照中发现 Windows 侧语义本身缺陷 → blocked 上交
- 领取：
- 回执：
- 验收：
