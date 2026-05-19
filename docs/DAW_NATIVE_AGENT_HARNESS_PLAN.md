# VitDAW DAW-Native Agent Harness Plan

> Status: architecture plan
> Target: VitAgent v0.2+
> Scope: design the long-term DAW-native agent harness after VitAgent v0.1 has replaced the Python bridge path and Ask Vit can reach the LLM.

## 1. Core Positioning

VitAgent is not a generic chatbot and it is not an XML editor. It should become the DAW-native agent harness for VitDAW: the process that understands the current music project, exposes safe DAW tools to AI, validates every mutation, records agent actions, and keeps Godot UI, VitApp kernel, project state, plugins, assets, memory, and external tools on one controlled execution path.

The long-term data and execution flow is:

```text
VitApp kernel real project
  -> get_project_state / project XML
  -> Agent Project IR
  -> plan / patch
  -> kernel command execution
  -> undo / journal / shadow refresh
```

The VitApp / Tracktion engine remains the source of truth. Agent shadow state, Project IR, XML snapshots, and docs are working layers for reading, reasoning, diffing, validation, and audit.

XML is an observation, analysis, and planning material. It must not become the direct execution layer where the agent edits XML and asks the kernel to accept the file as truth. Reliable execution should always compile back into stable kernel commands, stable target IDs, undo labels, and a shadow refresh.

## 2. Runtime Shape

The target product runtime remains a three-process DAW:

```text
VitApp.exe        kernel / Tracktion-JUCE engine
Godot UI.exe      frontend / interaction layer
VitAgent.exe      bridge + AI agent harness
```

VitAgent keeps compatibility with the existing bridge protocol while growing agent capabilities:

- Godot sends command packets through UDP `4445`.
- Godot receives telemetry through UDP `4444`.
- Agent sends kernel requests through ZMQ `5555`.
- Agent receives kernel telemetry through ZMQ `5556`.
- Ask Vit talks to the local Agent HTTP API.

This preserves the current Godot IPC path. The first agentization goal is not to rewrite the UI communication model, but to place one reliable harness between natural language, DAW commands, tool execution, and kernel state.

## 3. Harness Subsystems

### 3.1 `bridge`

Maintains the existing Godot UDP and VitApp ZMQ bridge behavior. It should remain boring and protocol-compatible: forward commands, forward telemetry, enforce timeouts, and preserve existing DAW operation flows.

### 3.2 `shadow`

Keeps a current project mirror from `get_project_state` and kernel telemetry. Deltas may be used for low-latency updates, but full snapshots remain the reconciliation anchor.

Required behavior:

- Initialize from `get_project_state`.
- Refresh after structural commands and important telemetry such as `recording_stopped`.
- Detect sequence gaps or suspicious drift.
- Provide concise state summaries for Ask Vit.
- Keep stable IDs for tracks, clips, rack nodes, plugins, parameters, and generated assets.

### 3.3 Project IR

Project IR is the agent-readable representation above raw kernel JSON/XML. It should be optimized for LLM reasoning, not for file persistence.

It should include:

- project metadata: path, tempo, time signature, sample rate, transport state;
- tracks: stable IDs, names, types, mute/solo/arm, routing, clips, plugins;
- clips: stable IDs, source files, timing, length, selection, take/generation metadata;
- MIDI: notes, ranges, patterns, quantization state where available;
- rack/control graph: nodes, edges, exposed controls, macro mappings;
- plugins: instances, parameters, aliases, grabber profiles;
- assets: imported audio, generated files, renders, takes;
- current user/UI context: selected track, clip, node, parameter, time range;
- memory references: project goal, style notes, user preferences, previous actions.

Project IR can be produced from `get_project_state`, selected XML/project-document reads, project docs, and memory. It can be patched by the agent, but the patch must be compiled into kernel commands before execution.

### 3.4 Project Reconciler / Shadow Reconciler

The reconciler is not an AI tool. It is a harness subsystem.

Responsibilities:

- compare kernel snapshots, shadow state, Project IR, and planned patches;
- decide when a full refresh is required;
- verify that command results match expected structural changes;
- reject or pause execution if stable target IDs disappear or drift;
- generate human-readable diffs for preview/confirmation;
- feed journal entries with before/after summaries.

### 3.5 Tool Registry

The tool registry is the local catalog of tools the agent may call. It includes DAW tools, plugin grabber tools, traditional agent tools, memory tools, and safety/undo tools.

Each tool entry should define:

- tool name and namespace;
- description for model/tool routing;
- input schema;
- output schema;
- risk level;
- whether it mutates the DAW project;
- whether it supports undo;
- required stable target IDs;
- confirmation policy;
- kernel command mapping or local implementation;
- refresh policy after execution.

### 3.6 `command_catalog`

`command_catalog` should be generated from, or at least synchronized with, kernel-supported commands. This avoids forcing the agent code to manually duplicate every DAW capability forever.

The catalog should expose:

- command name;
- argument schema;
- command category;
- mutation type;
- risk level;
- undo support;
- busy/rendering allowlist status;
- required target ID types;
- result shape;
- expected refresh behavior.

Long term, DAW features should enter the kernel command catalog first, then become available to UI, shortcuts, mouse capsule hints, and agent tools from the same command identity.

### 3.7 `daw.invoke`

`daw.invoke(command_name, args)` is the stable execution gateway for DAW mutations and low-level DAW queries.

Execution path:

```text
agent plan
  -> daw.invoke
  -> command_catalog validation
  -> policy risk check
  -> optional user confirmation
  -> kernel IPC command
  -> result validation
  -> shadow refresh / reconciler check
  -> journal entry
  -> Ask Vit/UI response
```

Every modifying action must carry:

- `agent_action_id`
- `undo_label`
- `risk_level`
- `requires_confirmation`
- stable target IDs, never only UI row indexes

### 3.8 `policy`

Policy decides whether a planned action can run directly, needs preview confirmation, should ask the user a clarifying question, or must be rejected.

Policy should be deterministic and inspectable. The LLM may propose a plan, but policy owns execution permission.

### 3.9 `journal`

Journal records agent intent, tool calls, kernel commands, user confirmations, results, before/after summaries, and undo labels.

Journal is required for:

- "undo the last thing you did";
- debugging agent mistakes;
- building user trust;
- future project memory;
- reproducible A/B takes and generated assets.

### 3.10 `memory`

Memory is queryable project/user knowledge, not raw chat history.

It should remember:

- user preferred styles, BPM ranges, instruments, and workflow habits;
- commonly used plugins;
- plugin parameter aliases and grabber profiles;
- common sample folders;
- current project goal;
- mix preferences;
- executed agent actions;
- generated asset provenance;
- user corrections and confirmations.

Memory must stay separate from kernel truth. It can guide planning, but execution still goes through tools and kernel commands.

## 4. Tool System

VitAgent needs both DAW-native tools and traditional agent tools. The final system should not choose between "full hand-written DAW tools" and "edit XML directly". It should use a layered approach:

```text
high-level semantic tools
  -> daw.invoke / command_catalog
  -> kernel commands
  -> shadow refresh / reconciler

Project IR / XML / docs
  -> reading, planning, diffing, fallback reasoning
  -> compiled commands
  -> kernel execution
```

If a requested operation cannot yet be expressed by a high-level tool, the agent may inspect Project IR, XML, docs, and command catalog to propose a patch. That patch still needs a compiler step into supported kernel commands or a clearly marked "missing kernel/tool capability" result.

### 4.1 DAW 专用工具

These tools are the core musical operation layer.

#### `project`

- `project.state`
- `project.save`
- `project.open`
- `project.undo`
- `project.redo`
- `project.health`

#### `transport`

- `transport.play`
- `transport.stop`
- `transport.record`
- `transport.set_tempo`
- `transport.set_loop`
- `transport.clear_loop`

#### `track`

- `track.add`
- `track.delete`
- `track.rename`
- `track.mute`
- `track.solo`
- `track.arm`
- `track.route`

#### `clip`

- `clip.import`
- `clip.move`
- `clip.resize`
- `clip.split`
- `clip.delete`
- `clip.select`

#### `midi`

- `midi.read_notes`
- `midi.write_notes`
- `midi.quantize`
- `midi.generate_pattern`

#### `rack`

- `rack.add_node`
- `rack.connect`
- `rack.remove`
- `rack.scope`
- `rack.read_graph`

#### `plugin`

- `plugin.scan`
- `plugin.search`
- `plugin.instantiate`
- `plugin.open`
- `plugin.get_params`
- `plugin.set_params`

#### `render/assets`

- `render.start`
- `render.cancel`
- `assets.ingest_generated_asset`
- `assets.switch_take`

### 4.2 插件抓手工具

Plugin grabber is a core VitDAW agent feature. The goal is not merely to expose a raw parameter list. The goal is to convert chaotic plugin parameters into musical controls that an AI and a musician can both understand.

Required tools:

- `plugin_get_parameters`
- `plugin_set_parameter`
- `plugin_set_aliases`
- `plugin_make_grabber_profile`
- `plugin_explain_controls`
- `plugin_map_macro_to_params`
- `plugin_create_control_graph_node`
- `plugin_learn_common_roles`

Semantic roles should include common musical meanings such as:

- `cutoff`
- `resonance`
- `drive`
- `mix`
- `attack`
- `release`
- `threshold`
- `ratio`
- `gain`
- `pan`
- `width`
- `rate`
- `depth`
- `feedback`
- `decay`
- `sustain`

The plugin grabber should support:

- raw parameter snapshot from the kernel;
- normalized value range `0..1`;
- display name and unit where available;
- alias mapping;
- confidence score for learned roles;
- user-confirmed profile;
- macro controls mapped to multiple parameters;
- control graph node creation for AI-friendly rack automation.

### 4.3 传统 Agent 工具

These tools are required because the DAW agent must work like a real project assistant, not only a DAW command runner.

First principle: ordinary file tools should be read-first. Write operations should go to agent scratch/workspace first, then enter the DAW through DAW tools such as asset ingest or MIDI import.

Required tools:

- `web_fetch`: read plugin manuals, model docs, audio terminology, API docs, and external references.
- `read_file`: read project documents, user lyrics, reference notes, plugin preset text, and exported metadata.
- `glob`: find project files, sample packs, lyrics, MIDI, configs, and generated assets.
- `grep`: search text inside project docs, presets, lyrics, notes, and config files.
- `read_project_docs`: read VitDAW project notes and current project instructions.
- `read_audio_metadata`: inspect duration, sample rate, channels, BPM tags, loudness or format metadata where available.
- `read_midi_file`: inspect MIDI tracks, channels, tempo, note ranges, note density, and structure.
- `write_scratch_file`: write drafts, analysis reports, generated MIDI/JSON, temporary manifests, and proposed patches without overwriting the project.
- `ask_user`: ask clarifying questions when the musical goal or target object is ambiguous.
- `todo`: maintain an agent execution checklist.
- `journal`: record plans, executed actions, confirmations, undo labels, errors, and observations.

Later tools may include `shell`, `write_file`, and `replace_file`, but only with sandboxing, approval, path restrictions, and clear separation from direct DAW project mutation.

### 4.4 记忆与知识工具

Memory tools should expose structured, queryable knowledge.

Required memory domains:

- user style preferences;
- preferred BPM ranges;
- common plugins and instruments;
- plugin parameter aliases;
- plugin grabber profiles;
- sample folders;
- current project goals;
- mix preferences;
- agent action history;
- generated asset provenance;
- user approvals, rejections, and corrections.

Memory operations should include:

- `memory.search`
- `memory.get`
- `memory.upsert`
- `memory.link_to_project`
- `memory.link_to_plugin`
- `memory.link_to_action`

Memory writes should usually be low risk but visible. Sensitive external secrets such as API keys do not belong in project memory.

### 4.5 安全与 Undo 工具

Safety is not just confirmation prompts. It is stable IDs, validation, undo, journal, preview, and refresh.

Required tools/capabilities:

- `policy.evaluate`
- `plan.preview`
- `agent.confirm`
- `agent.reject`
- `project.undo_agent_action`
- `journal.get_recent_actions`
- `journal.get_action`
- `journal.mark_result`
- `reconciler.refresh`
- `reconciler.diff`

Every agent mutation should be recoverable when the kernel supports undo. The user should be able to say "撤回刚才那个操作", and the harness should map that to the latest relevant `agent_action_id` and `undo_label`.

## 5. Safety Model

The first production policy should be mixed and DAW-aware.

### Direct Execution

The following can execute directly:

- query project state;
- explain current project;
- read tracks/clips/plugins;
- play;
- stop;
- go to start;
- inspect parameters;
- read docs/files;
- search project assets;
- open harmless views where supported.

### Direct But Undoable

Small project edits can execute directly when all targets are unambiguous and undo is available:

- rename one track;
- mute/solo/arm one track;
- add one empty track;
- set tempo when user clearly asked;
- move one selected clip by an explicit amount;
- set one plugin parameter when target and parameter are explicit;
- create a scratch file outside the real project file.

The UI/Ask Vit response should show: "已执行，可撤销", plus the undo label.

### Requires Confirmation

The following require preview confirmation:

- delete tracks or clips;
- overwrite existing files;
- batch edits;
- importing large files or many files;
- rendering/exporting;
- external writes outside scratch/workspace;
- modifying rack topology;
- changing MIDI notes in existing clips;
- destructive plugin/rack changes;
- operations with missing or low-confidence stable target IDs;
- operations where the user intent is broad and the result is subjective.

### Ask First

The agent should ask the user rather than guess when:

- target track/clip/plugin is ambiguous;
- "make it better" has no current style/goal context;
- multiple selected objects are plausible;
- the action may overwrite creative work;
- the agent cannot explain what command sequence will produce the requested result.

## 6. Public Agent Interfaces

Existing interfaces remain:

- `GET /health`
- `GET /agent/state`
- `POST /agent/chat`
- `POST /agent/confirm`

Planned interfaces:

### `GET /agent/tools`

Returns the tool registry exposed to Ask Vit and future UI surfaces.

Each tool record should include:

- `name`
- `namespace`
- `description`
- `input_schema`
- `output_schema`
- `risk_level`
- `mutates_project`
- `supports_undo`
- `requires_confirmation`
- `required_target_ids`

### `POST /agent/invoke`

Debug/development endpoint for invoking one tool or DAW command through the same path the LLM uses.

This endpoint should not bypass policy.

Request should include:

- `tool`
- `args`
- `source`
- optional `user_confirmation_token`

Response should include:

- `status`
- `agent_action_id`
- `requires_confirmation`
- `preview`
- `result`
- `undo_label`
- `error`

### `GET /agent/actions`

Returns recent journaled agent actions.

Each action should include:

- `agent_action_id`
- `time`
- `source`
- `summary`
- `commands`
- `risk_level`
- `confirmation_status`
- `status`
- `undo_label`

### Internal `daw.invoke`

Internal tool gateway used by chat, planners, task runners, and future multi-agent execution. It should be the preferred implementation path even if `/agent/invoke` is not exposed in release builds.

## 7. Role Of Vit lage

`D:\Vit lage\Vit lage` should not become the main DAW agent codebase.

It may be used as reference for:

- AI config format;
- LLM client/provider calling style;
- approval/confirmation ideas;
- traditional agent tool registry patterns;
- later multi-agent/DAG design lessons.

Do not directly migrate:

- Wails UI;
- unfinished multi-agent executor;
- complex DAG system;
- unrelated app-shell assumptions.

VitDAW needs a DAW-native harness first. General multi-agent orchestration can be layered in later after tool execution, safety, memory, and undo are stable.

## 8. Version Roadmap

### v0.2 Tool Foundation

Goal: make VitAgent a real tool harness, not only bridge + chat.

Implement:

- tool registry;
- `command_catalog`;
- internal `daw.invoke`;
- `agent_action_id`;
- undo labels;
- journal;
- `GET /agent/tools`;
- `GET /agent/actions`;
- stronger `/agent/state` summary;
- policy metadata per tool.

Acceptance:

- Ask Vit can see available tools.
- Low-risk commands run through `daw.invoke`.
- Dangerous commands return preview/confirmation.
- Every mutation writes journal.

### v0.3 Basic DAW Tools

Goal: cover the most common low-risk and medium-risk DAW operations.

Implement:

- project state/save/undo/redo/health;
- transport play/stop/record/tempo/loop;
- track add/delete/rename/mute/solo/arm/route;
- clip select/import/move/resize/split/delete in safe subsets;
- refresh and reconciliation after structural edits.

Acceptance:

- User can ask Ask Vit to inspect, play, stop, rename, mute, add tracks, and perform simple clip operations.
- Deletes and batch edits require confirmation.
- Undo can target the latest agent action.

### v0.4 Plugin Grabber

Goal: make plugins AI-addressable by musical meaning.

Implement:

- parameter snapshot tools;
- alias mapping;
- grabber profile creation;
- explain controls;
- macro-to-parameter mapping;
- common role learning;
- control graph node creation.

Acceptance:

- Agent can explain a plugin in musical terms.
- User can map "brightness" or "filter cutoff" style controls to real plugin parameters.
- Saved profiles are reused across sessions/projects.

### v0.5 Project IR + Patch Planner

Goal: bring the XML/IR idea back in a safe form.

Implement:

- Project IR builder from `get_project_state`, XML, project docs, and memory;
- IR diff and patch format;
- patch-to-command compiler;
- reconciler verification;
- preview UI payloads for structured plans.

Acceptance:

- Agent can propose multi-step project edits as readable patches.
- Patches compile into kernel commands.
- Execution updates shadow and journal.
- XML is used for reading/diff/audit, not direct writeback.

## 9. Testing And Acceptance

Documentation acceptance:

- All five tool groups are listed: DAW tools, plugin grabber tools, traditional agent tools, memory/knowledge tools, safety/undo tools.
- The document clearly states that XML is not the direct execution layer.
- The document preserves the hybrid architecture: kernel truth, Agent IR/planning, kernel command execution.

Harness acceptance for later implementation:

- `VitAgent.exe` continues replacing the Python bridge without breaking UDP `4444/4445` and ZMQ `5555/5556`.
- `GET /agent/tools` lists tools with risk and undo metadata.
- `daw.invoke` validates against `command_catalog`.
- Low-risk commands can execute directly.
- Dangerous commands require confirmation.
- Each mutation records `agent_action_id` and `undo_label`.
- Shadow refresh occurs after mutations and important telemetry.
- User can undo the latest agent action.

DAW connection acceptance:

- UDP `4445` `ping` reaches kernel and returns `pong`.
- `get_project_state` initializes shadow and Project IR input.
- Kernel telemetry still forwards to Godot UDP `4444`.
- `recording_stopped` triggers agent project refresh.
- Ask Vit can call AI and receive either answer text, direct execution result, or confirmation preview.

## 10. Non-Goals For The First Agent Harness Stage

- Do not build the full multi-agent/DAG runtime yet.
- Do not replace Godot's existing ordinary DAW IPC path yet.
- Do not make XML direct writeback the main execution path.
- Do not expose broad shell/write filesystem tools without sandboxing and approval.
- Do not require every DAW feature to have a handcrafted high-level semantic tool before `command_catalog` and `daw.invoke` exist.

## 11. Design Principle

The target is a DAW-native agent harness:

- native to VitDAW's kernel, project model, plugins, clips, rack, MIDI, render, and undo;
- readable by AI through state, IR, docs, XML, memory, and tool schemas;
- executable through stable kernel commands, not fragile UI indexes or direct XML overwrite;
- safe through policy, confirmation, stable IDs, journal, undo, and reconciliation;
- extensible through command catalog, plugin grabber profiles, traditional agent tools, and later multi-agent orchestration.

This keeps the original "read the whole project and reason over it" dream, but removes the risky shortcut of asking the model to edit the project file as the source of truth.
