# 405：真实JSONL解码链无窗补验

主管审前一提交后指出原25例只直接调用geometryTrace。本补段只改原probe.cpp的显式--geometry-selftest函数，前端提交24c6664；readGeometryTrace/geometryTrace运行判据、观察器、唯一候选顺序和主脚本均未改。原25例保持原样。未启Godot、DLL或测试窗口。

## 新增文件链检查

原无窗入口在输出文件同级创建PID专属全新decoder目录；已存在则不覆盖并失败。所有临时fixtures位于本次jsonl_decoder_static工件下，不写源码树或旧工件。以observer write_record的完整嵌套schema序列化native记录和lifecycle drain_end，随后**真实调用readGeometryTrace**（同runtime解析函数），而非只验证内存向量。

新增16例：完整schema最终往返、完整schema前缀往返、drain后partial尾在prefix接受而final拒绝、drain外完整未提交记录在prefix丢弃而final拒绝、缺boolean、缺DPI、缺awareness、错nonce、seq缺口、最终drain边界不符、缺drain、嵌套windowpos尺寸、嵌套flags的NOSENDCHANGING、嵌套flags+normal_rect的恢复前提解码。几何错误必须返回原有具体失败类型，不能因parser失读转成假PASS。最终partial文件是有意无效JSONL反例，原字节留存。

geometry_selftest.jsonl中原25例+新增16例共41例，0failure/window_created0，exit0。新增schema测试只在显式selftest入口执行，不改变正常probe路径。

## Schema实物静态交叉核对

另一个run内Python审计脚本静态读取**当前observer.cpp实际write_record snprintf字符串**，展开占位符解析为JSON，再和生成fixture首个native_message逐层比较字段路径：51路径完全一致，missing[]/extra[]。原formatter及DLL均未执行，也未手写不全schema冒原生输出。observer_schema_check.json保存具体字段路径。

初版该审计helper只展开%u/%llu漏了实际%lu PID占位，JSON解析失败exit1；原helper、错误说明、commands退出记录保留。修正的final helper只增加%lu替换，exit0；它不是native parser/policy修正，也没有覆盖旧测试输出。

## 命令、哈希与边界

本段新build目录configure0/build0，旧probe保留已知C4244 nonce字符宽窄转换warning，无新增编译错误。唯一执行命令是构建后的probe --geometry-selftest <本次工件输出>；未执行--godot/--godot-observer/--godot-geometry。完整命令exit在commands.txt，原始fixtures和selftest输出均入库；final_hashes.json包含当前源码/新binarySHA，build/binary排除提交。git diff --check0。

可证：实际JSONL读入链与当前observer schema在这些正反例按预期工作，prefix/final语义与原判据一致。不能证真实Godot加载、placement/DPI坐标、首ordinary geometry/z或2s运行结果；前一405候选仍须主管另批runtime。没有修改生产或旧工件、没有自动补跑窗口。
