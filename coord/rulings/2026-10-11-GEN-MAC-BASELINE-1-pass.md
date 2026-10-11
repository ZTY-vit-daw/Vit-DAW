# Ruling：GEN-MAC-BASELINE-1 pass（2026-10-11，决策侧）

- **裁定：pass**。合入 main = cherry-pick f95f4760/bd079a09/d2d8070e/9560deba/dfcb2947（impl+三次探针修正+工件）；卡片状态链（领取 67a904c9/占用 de3a3b6d/done 23ad4606/回执补遗 4b363393）执行侧已按 §3 直推 main；port/gen-mac-baseline-1 在远端（f90c23f0）留原哈希。
- **决策侧独立复验（Mac 决策会话亲核，非转述）**：
  - 零生产源码：`git diff --name-only origin/main..port/gen-mac-baseline-1 -- VitApp/Source agent/` 为空，75 文件全在 scripts/+coord/——卡面红线兑现。
  - 双二进制哈希亲复：/tmp/vit-genbase1-build 内核 `9a70f064…` 与 vitagent `d3dc1662…` 逐字节复现（shasum 亲算=回执一致）；K1（79b8ecdc）/K2（9f53bd4e）祖先声明与 79b8ecdc..HEAD 对 VitApp/Source 零改动声明采信（合并自 main 的实现链，我方已核同域零改动）。
  - run4 工件亲读：summary.json 30 门全 true+failure_kind=none+gate_failure_details 空；legB 同步拒绝回执亲读（stage=rejected、reason=no_renderable_audio_content、无 job_id——K1 语义 Mac 首证属实）；49 份逐探针回执含 17 确认卡双形态。
  - 栈记录闭环：MAC-RUNTIME-STACK.md 占用（登记前核对空闲）→释放（12:34 复核端口/进程=0）闭环；验收期端口/进程亲核空闲。
  - **独立复跑（最强腿）**：决策侧亲自重跑 `scripts/gen_chain_survey_smoke_mac.sh --kernel-bin … --agent-bin …` → **exit 0、SURVEY COMPLETE all gates green、30 门逐行 gate ok**（run workdir 独立临时目录，18:20 末轮），脚本自 teardown 后 lsof/pgrep 亲核零残留。
- **缺口清单与上交采信**（report.md 四条开放项）：A1 instrument 通道维持开放（归 A1-MAC-FORM-1，PC 2026-10-06 刹车裁定不变）；**mutate argspec 与内核校验漂移**（内核要求每 note 非空 id vs catalog.go:540 id 可选——立后续小卡由决策侧定对齐方向）；note start 坐标系=clip 相对 beats（消费方约定记录）；SMF chunk id 静默容忍（行为注记）。探针迭代三轮均为脚本缺陷、栈本体零功能红的失败分类采信（probe-iteration-history 归档+逐 commit 排障叙事与工件吻合）。
- **端测边界**：两件套（内核+agent）命令/回执面，Godot 前端/webui/旅程面不在本卡——与卡面范围一致，免真栈三件套验收（无前端相关改动）。
- 执行侧评价：Mac 线首张真栈卡达标——探针缺陷三轮如实归档不硬凑、栈占用登记/释放闭环、FetchContent 代理坑解法留档（GIT_CONFIG_* 空覆盖直连）、二进制哈希全记录。流程偏差一处不计过：done 卡先于实现合入直推 main 致 main 短暂引用缺失工件，本裁定 cherry-pick 补齐闭合。
- 后续解锁：A1-MAC-FORM-1（池序 61，依赖本卡合入——现已满足）；AIGC-DRUM-GEN-1 写入+回读腿 Mac 具备开跑条件（report §三）。
