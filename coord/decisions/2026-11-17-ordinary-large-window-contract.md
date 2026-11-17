# 普通大窗契约纠正：保留windowed与“在对话窗后”

主管/root，2026-11-17。405 R2已实测首次ordinary为[0,0,2560,1440]、无FG/focus，但Godot mode4；原生immediate predecessor非guard，未记录完整z链。本轮失败保持，不改原判据追认PASS。

原产品目标是普通大窗、对话窗在前、不抢焦点。405将“大窗”具体化为outerrect=workarea、将“在后”具体化为immediate neighbor，这两项是试验假设，不是用户要求：work==monitor时Godot源码WM_WINDOWPOSCHANGED~5869将其识别fullscreen；neighbor不同也不能推出guard不在上方。证据推翻这两试验具体化，不能为了守住它们破坏windowed或误判层级。

后续407目标为workarea内四边各留16 DIP（按已证同systemDPI/monitor域换算、取整并冻结）的普通大窗；native rect/engine mode0/非zoomed/nonTOPMOST，无postshow resize/z纠正。层级门为guard确实在child上方，使用目标自线程有界完整前驱链证明，不再要求紧邻。自有reference与parent PID/nonce绑定；链断/loop/截断而无法找到reference=coverage不足；任何已有普通可见状态guard不在上方即FAIL。原无FG/首ordinary因果/native全链/2s/生命周期门不变。

这保留用户体验目标，修正执行假设；不是放宽失败run或回避mode0。405候选本轮关闭，407新冻结实现/测试/运行；成功后才改产品并跑完整矩阵。
