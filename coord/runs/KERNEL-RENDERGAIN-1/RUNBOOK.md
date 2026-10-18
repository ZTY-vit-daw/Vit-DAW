# KERNEL-RENDERGAIN-1锛?32锛夋爤鑵?RUNBOOK锛氬唴鏍哥绾挎覆鏌?+6 dB 宸垎瀹為獙

- owner锛欴SH PC 鎵ц娴?exec-rendergain锛泈orktree `D:/Vit_DAW_wt_rendergain1`锛屽垎鏀?`port/kernel-rendergain-1`
- 鍓嶇疆锛氳€冨彜鎶ュ憡 [ARCHAEOLOGY.md](ARCHAEOLOGY.md) 宸茬粰鍑烘牴鍥犲亣璁撅紙master volume fader 浣嶇疆 1.0 = +6.000 dB锛夛紱鏈吙鎶婂亣璁惧崌涓虹湡鏍堝疄娴?- 杈圭晫锛?*闆剁敓浜т唬鐮佹敼鍔?*锛涘彧鏂板鏈洰褰曡瘖鏂剼鏈紱涓嶅惎 Godot UI锛涗笉鍐欏叡浜富宸ヤ綔鏍戯紱涓?push main

## 0. 椹卞姩鐨勪綋绯讳笌鍋忓樊澹版槑锛圠ead 鐭ユ倝椤癸級

- 鏃㈡湁 `scripts/dev_agent_smoke.ps1` 鏄崟浣撹剼鏈€佹棤鍙寕鑷畾涔夎吙鐨勫紑鍏筹紝涓旀湰鍗＄姝㈡敼鏃㈡湁鑴氭湰銆傛湰鑵?*澶嶅埢鍏?berth 绾緥**锛?  1. 鍚屼竴鏃跺埢鍗曟爤 owner锛氬惎鏍堝墠鎷掓枼浠讳綍宸茬洃鍚爤绔彛/宸插瓨鍦ㄦ爤杩涚▼锛圓GENTS 搂9锛夛紱
  2. **run-dir kernel workspace 闅旂**锛堝悓鑴氭湰 journey-berth 濂戠害 dev_agent_smoke.ps1:981-1020锛夛細`Settings/Logs/default_project.xml` 鎷疯繘鏈 run 鐩綍锛宍VIT_PROJECT_XML` 鐜鍙橀噺閲嶅畾鍚戝埌 run 鐩綍鐨勯粯璁ゅ伐绋嬪壇鏈紙VitPaths.h:183 鏀寔锛夛紝鍐呮牳宸ヤ綔鐩綍=run 鐩綍 workspace 鈫?**瀵逛富妫€鍑?`VitApp/Workspace` 闆跺啓鍏?*锛?  3. 鍙惎鍔ㄥ唴鏍革紙**涓嶅惎 agent銆佷笉鍚?UI**锛夛細鏈吙鍏ㄩ儴瀹為獙璧板唴鏍?ZMQ 鍛戒护闈紙5555锛夛紝涓嶉渶瑕?agent HTTP/LLM 闈紱鍚爤鍓嶅悗閮戒翰鏍稿洓绔彛+鍥涚被杩涚▼锛?  4. finally 鎷嗘爤锛氬彧鏉€鏈吙鑷捣 pid锛屽鏍哥鍙?杩涚▼=0銆?- 璇ュ亸宸紙kernel-only 鑰岄潪 kernel+agent锛夊瀹炰笂鎶?Lead锛氳嫢 Lead 瑕佹眰 agent 鍚屽湪鏍堬紝鍙姞 `-StartAgent` 鏀嚎锛堟湰鑵垮疄楠屼笉渚濊禆 agent锛夈€?
## 1. 琚祴涓庣幆澧?
| 椤?| 鍊?|
|---|---|
| 鍐呮牳 exe | `D:\Vit_DAW\Export\staging\runtime\VitApp.exe`锛坰tock锛屽彧璇诲紩鐢紝涓嶅湪涓绘爲鍐欏叆锛?|
| 鍐呮牳 SHA256 | `B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B`锛? K1 璁板綍鐨?stock 鍐呮牳鍝堝笇锛?|
| 绔彛 | 5555 REQ / 5556 SUB / 5557 log锛堝唴鏍稿浐瀹氾級锛?878/8787/4445 浠呭仛鍗犵敤棰勬 |
| workspace | `coord\runs\KERNEL-RENDERGAIN-1\run_<ts>\kernel_workspace\`锛堟瘡娆℃柊鐩綍锛?|
| python | PATH `python`锛?.13锛宲yzmq 27.1.0 / numpy / scipy 鍦級 |
| 搂8 澹版槑锛堣繍琛屽墠锛?| LLM=0 娆¤皟鐢紙绾‘瀹氭€у唴鏍稿懡浠ら潰锛夛紱鎴愬姛鏉′欢=鑴氭湰 exit 0锛堟瘡鑵垮湪棰勬湡瀹瑰樊鍐呬笖鎷嗘爤骞插噣锛夛紱澶辫触鍒嗙被=environment_interrupt锛堟爤璧蜂笉鏉?绔彛鍗犵敤/杩涚▼娈嬬暀锛?assertion_fail锛堝疄娴嬩笌鑰冨彜棰勬湡涓嶇锛夛紱姝㈡崯=鍚岀鍚?assertion_fail脳2 |

## 2. 鍚姩鍛戒护锛圠ead 鏀捐/G袨 鍚庢墽琛岋級

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File D:\Vit_DAW_wt_rendergain1\coord\runs\KERNEL-RENDERGAIN-1\run_render_gain_diag.ps1 `
  -RepoRoot D:\Vit_DAW_wt_rendergain1 `
  -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe
```

- 闀胯窇锛堝缓璁級锛氫互 `run_in_background` 璧凤紝浣滀笟鏃ュ織鐣欏瓨锛涜剼鏈嚜韬啓 `run_meta.json`/`diag_console.txt`/`diag_summary.json`/`teardown.json`/`kernel_stdout.log`锛?- 鑴氭湰鑷姩鍋氾細绔彛/杩涚▼棰勬 鈫?run-dir workspace 鍑嗗 鈫?鍚唴鏍?鈫?绛?5555 鈫?璺?python 宸垎 鈫?finally 鎷嗘爤澶嶆牳銆?
## 3. 宸垎瀹為獙鐭╅樀锛堣剼鏈?`render_gain_diag.py` 瀹炵幇锛屽叏閮ㄨ褰曡繘 diag_summary.json锛?
| 鑵?| 鍐呭 | 棰勬湡锛堣€冨彜鍋囪锛歮aster pos 1.0=+6 dB锛?|
|---|---|---|
| E0 | 鍚姩鎬佽鍥烇細`get_project_state.snapshot_xml` 鐨?MASTERVOLUME volume 鍊?+ list_tracks master 琛?db | volume="1.0"锛沵aster track db=0.0锛堣瘉鏄?master volume 涓嶅湪杞ㄩ亾璇诲洖闈級 |
| E1 | K1 鏈€灏忓鐜帮細clear_project鈫抋dd_track鈫抜mport(48k 绔嬩綋澹?0.5 440Hz)鈫抮ender 16bit(榛樿 use_master_plugins=true)鈫掗€愬０閬撳箙搴?| amp ratio=1.9953锛?6.0 dB锛夛紱peak鈮?.9976 |
| E2 | 鍚岀紪杈戜粎 `start_render.use_master_plugins=false` | ratio=1.0000 鈫?鍧愬疄娉ㄥ叆鐐?master 鑺傜偣 |
| E3 | `load_project` 5 浠戒粎 MASTERVOLUME 涓嶅悓鐨?fixture锛?.0 / 0.7408182 / 0.6376282 / 鏃?MASTERVOLUME / 鏃?volume 灞炴€э級 | ratio 渚濇 1.9953 / 1.0000 / 0.7079 / 0.7079 / 1.0000 |
| E4 | `new_project`锛坆lank 妯℃澘锛夆啋 add/import/render | ratio=1.9953锛泂napshot volume="1.0" |
| E5 | 鍚?fixture/source 16/24/32-bit 娓叉煋 | ratio 涓嶅彉锛堚啋 鍐欏叆鍣ㄤ笉鏄鐩婃潵婧愶級锛涘彟鍔犵儹婧?0.9 鍓婃尝鑵匡紙+6 dB 鍚?1.795 杩囨弧鍒诲害锛?6/24bit PCM 鍧?clamp锛?|
| E6 | 婧愬箙搴?0.05/0.1/0.25锛?0.5 宸插湪 E1锛?| ratio 鎭掑畾锛堢嚎鎬э紝鏈墛娉級 |
| E7 | 44.1k 婧愶紙閲嶉噰鏍疯矾寰勶級/ 鍗曞０閬撴簮 / 宸︾嫭鍗犳簮 | ratio 鎭掑畾锛涘乏鐙崰 R鈮?锛堥€愬０閬撴爣閲忥級 |
| E8 | 璇诲洖闈細master volume 涓嶅湪浠讳綍杞ㄩ亾琛屻€佹棤鍛戒护闈?| 璁板綍"涓嶅彲瑙佽瘉鎹? |

瀹瑰樊锛歳atio 鐩稿瀹瑰樊 1%锛?6bit 閲忓寲 + writer rounding锛夛紱E7 宸︾嫭鍗?R 閫氶亾 鈮?e-6銆?
## 4. 棰勬湡缁撹钀界偣锛堜笌 ARCHAEOLOGY.md 瀵瑰簲锛?
- 鎴愮珛 鈫?娉ㄥ叆鐐?master volume 鐘舵€侊紙妯℃澘鍊?1.0/+6 dB锛夛紝娓叉煋/鍐欏叆鍣ㄦ棤鍏筹紱K1 鐨?~2x 澧炵泭"= +6.0 dB 绮剧‘鍊硷紱
- 涓嶆垚绔?鈫?閫愯吙澶辫触绛惧悕璁板綍锛堝摢涓€鑵裤€佸疄娴?vs 棰勬湡锛夛紝鎸?AGENTS 搂8 鍚岀鍚?2 娆℃鎹熷苟涓婁氦銆?
## 5. 宸ヤ欢涓庡洖鎵?
- 杩愯 dir锛歚coord/runs/KERNEL-RENDERGAIN-1/run_<yyyyMMdd_HHmmss>/`锛坄run_meta.json`銆乣diag_summary.json`銆乣diag_console.txt`銆乣renders/*.wav`銆乣sources/*.wav`銆乣fixtures/*.vit`銆乣kernel_workspace/`銆乣teardown.json`锛夛紱
- 鍥炴墽缁?Lead锛氭爤璇佹嵁锛堝懡浠?exit code/run id/kernel sha256/绔彛涓庤繘绋嬫媶鍑€澶嶆牳锛? 涓夐棶缁堢瓟 + 鍒嗘敮 commit锛汸C-RUNTIME-STACK 鍗犵敤/閲婃斁璁板綍鐢?Lead 鍗曞啓銆?