import json
R = r'D:/Vit_DAW/coord/runs/UX-AB-AUDITION-COVERAGE-1/forensics_1'
out = open(R + '/evidence_extracts.md', 'w', encoding='utf-8')
out.write('# 取证证据摘录（原始文件路径+行号锚点，工件只读复制）\n\n')
out.write('## 源文件\n')
out.write('- 主仓 message loop debug: D:/Vit_DAW/VitApp/Workspace/Logs/agent_message_loop_debug.jsonl（28MB, 最后写 2026-10-23 00:36）\n')
out.write('- 内核日志1: D:/Vit_DAW/VitApp/Workspace/Logs/VitHeadlessServer2026-10-22_23-48-54.log（14MB, 23:48:54–00:24:57）\n')
out.write('- 内核日志2: D:/Vit_DAW/VitApp/Workspace/Logs/VitHeadlessServer2026-10-23_00-24-57.log（7.5MB, 00:24:57–00:27）\n')
out.write('- 内核日志3: D:/Vit_DAW/VitApp/Workspace/Logs/VitHeadlessServer2026-10-23_00-34-52.log（7.1MB, 00:34:52–00:38）\n')
out.write('- 用户工程 journal: C:/Users/timoz/AppData/Local/Vit/ProjectHistory/drafts/draft_20261022T162141_61d037c5/.vit_agent/vitproj_871a5e2577a24402a654bc48119079bb/journal/000001.jsonl\n')
out.write('- 用户工程会话态: 同目录 .vit_history/.sessions/.../workspace/state/agent_runtime_state.json\n')
out.write('- 174 遗留 agent 日志: D:/Vit_DAW/coord/runs/UX-AB-AUDITION-IMPL-1/aba174_chat_20261022_233614/logs/agent_last.log\n\n')
out.write('## A. 三轮 final gate 条目（agent_message_loop_debug.jsonl）\n\n')
for line in open(r'D:/Vit_DAW/VitApp/Workspace/Logs/agent_message_loop_debug.jsonl', encoding='utf-8', errors='replace'):
    if 'webui_mvjr' in line:
        d = json.loads(line)
        out.write('### {} {} goal={}\n'.format(d['at'], d['conversation_id'], d.get('goal_id')))
        out.write('```json\n' + json.dumps(d, ensure_ascii=False, indent=1)[:1600] + '\n```\n\n')
out.write('## B. 内核1（轮1 mvjr15zw）agent.main 直发命令（行号锚定）\n\n')
lines = open(r'D:/Vit_DAW/VitApp/Workspace/Logs/VitHeadlessServer2026-10-22_23-48-54.log', encoding='utf-8', errors='replace').read().splitlines()
marker = 'processing queued command on JUCE message thread: '
for i in (260452, 292393, 297383, 380348, 400454):
    l = lines[i]
    out.write('- line {}:\n'.format(i))
    out.write('```json\n' + l[l.find(marker) + len(marker):][:520] + '\n```\n\n')
out.write('## C. 内核2（轮2 mvjr5hcm）agent.main 直发命令（行号锚定）\n\n')
lines2 = open(r'D:/Vit_DAW/VitApp/Workspace/Logs/VitHeadlessServer2026-10-23_00-24-57.log', encoding='utf-8', errors='replace').read().splitlines()
for i in (81781, 86771, 169744, 189850):
    l = lines2[i]
    out.write('- line {}:\n'.format(i))
    out.write('```json\n' + l[l.find(marker) + len(marker):][:520] + '\n```\n\n')
out.write('## D. 用户工程 journal（轮1 全程工具/命令序列，去重）\n\n')
seen = set()
for line in open(r'C:/Users/timoz/AppData/Local/Vit/ProjectHistory/drafts/draft_20261022T162141_61d037c5/.vit_agent/vitproj_871a5e2577a24402a654bc48119079bb/journal/000001.jsonl', encoding='utf-8'):
    d = json.loads(line)
    k = (d.get('updated_at'), d.get('agent_action_id'))
    if k in seen:
        continue
    seen.add(k)
    out.write('- {} | {} | {} | {}\n'.format(d.get('updated_at'), d.get('source'), d.get('tool') or d.get('command_name'), (d.get('args_summary') or '')[:100]))
out.write('\n## E. 轮1 持久会话终态（agent_runtime_state.json, saved_at=2026-10-23T00:23:42）\n\n')
st = json.load(open(r'C:/Users/timoz/AppData/Local/Vit/ProjectHistory/drafts/draft_20261022T162141_61d037c5/.vit_history/.sessions/vitproj_871a5e2577a24402a654bc48119079bb/session_20261022T162141_df08b1b6/workspace/state/agent_runtime_state.json', encoding='utf-8'))
msgs = st['conversations']['webui_mvjr15zw']
out.write('```json\n' + json.dumps(msgs, ensure_ascii=False, indent=1)[:800] + '\n```\n\n')
out.write('## F. 174 遗留 agent 日志 00:23 区段（轮1 完成与 audition 挂载/拒选/重落座）\n\n```text\n')
for l in open(r'D:/Vit_DAW/coord/runs/UX-AB-AUDITION-IMPL-1/aba174_chat_20261022_233614/logs/agent_last.log', encoding='utf-8', errors='replace'):
    if l.startswith('2026-10-23T00:23:2') or l.startswith('2026-10-23T00:23:3') or l.startswith('2026-10-23T00:23:4'):
        if '[DEBUG]' not in l:
            out.write(l)
out.write('```\n')
out.close()
print('OK')
