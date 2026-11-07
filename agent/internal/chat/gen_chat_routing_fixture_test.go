package chat

// genChatRoutingRun2Message is the VERBATIM chat message from 278 run2
// (coord/runs/GEN-YUE-DIALECT-1/run_20261107_193711/leg1_chat_response.json,
// original_intent). Pre-fix it was hijacked by the pan_layout canary: the ABC
// score tail "|b32|Z|" contains the substring "b3" and the full-message
// substring match routed the turn into static_mix.pan_layout.v0
// (workflow=capability_runtime_v1, canary_stage=readiness_blocked,
// goal_status=failed). run3 (run_20261107_194419) sends the same body
// behind "/goal " and is the defect-2 fixture.
const genChatRoutingRunMessage = `请用 YuE2 把下面这段谱唱出来。调用 gen.generate 工具，参数：provider_id="yue-local"，abc=下面这段 ABC 谱（必须逐字原样传入，不要改写、不要截断），lyrics=下面的歌词，style_tags="Mandarin city pop, female vocal, warm electric piano, bass and drums, 108 BPM"，duration_seconds=60，不要传 track_id。

ABC 谱：
X:1
T:
M:4/4
L:1/32
Q:1/4=106
V: Vocal clef=treble name="Vocal Melody" snm="Vocal"
V: Ins clef=treble name="Ins Melody" snm="Inst."
K:C
% intro
V: Vocal
Z|z4e4e4e4d4c4A4c4-|
V: Ins
Z2|
% verse
V: Vocal
"Dm7"c4A4z12A4c4e4-|"G7"e4f8d12z8|"Em7"b8g8g4g8a4|"A7"g2a2z16g4a4c'4-|
V: Ins
Z4|
V: Vocal
"Dm7"c'4a8z8g4a4c'4-|"G7"c'4a8z8e4g4b4-|"Cmaj7"b12z8g4a4b4-|"A7"b12c'12z8|
V: Ins
Z4|
% chorus
V: Vocal
"Dm7"e'8c'8a6g6e'4|"G7sus4"z4c'8a8g8z4|"Em7"d'8b8g6e6d'4|"A7"d'4^c'8d'8e'8z4|
V: Ins
Z4|
V: Vocal
"Dm7"e'8c'8a6g6e'4-|"G7sus4"e'4c'8a8g4a4b4-|"Cmaj7"b12z12a4b4-|"A7"b4c'8d'8e'12|
V: Ins
Z4|
% outro
V: Vocal
"Dm7"z32|"G7sus4"z32|"Cmaj7"z32|"A7"z32|
V: Ins
e'8c'8a6g6e'4-|e'4c'8a8g12|d'8b8g6e6d'4-|d'4^c'8d'8e'12|
V: Vocal
"Dm7"z32|"G7sus4"z32|"Cmaj7"z32|"Cmaj7"z32|
V: Ins
e'8c'8a6g6e'4-|e'4c'8a8g12|b32|Z|


歌词：
[Verse]
夜色落在窗台
城市在呼吸
[Chorus]
把这一刻写成歌`
