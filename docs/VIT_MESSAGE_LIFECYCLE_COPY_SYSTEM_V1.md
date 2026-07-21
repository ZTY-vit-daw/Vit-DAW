# Vit Message Lifecycle & Copy System v1

Status: implementation contract  
Scope: Ask Vit WebUI, Agent Events, Project History, local conversation cache

## 1. Outcome

Vit exposes one durable conversation and one transient activity lane. A message may never be both. Proposal, Receipt, Verification, and user/assistant dialogue are durable. Observation, context assembly, execution progress, and verification progress are transient.

The UI must never render a transient duplicate of a durable Proposal. A completed turn must be understandable after every transient activity has disappeared.

The default conversation mode is labelled `协作`, not `即时`. `即时` is reserved for lifecycle language and is not shown as a per-message mode badge.

## 2. Message envelope

Every new message or event uses these fields:

| Field | Values | Contract |
| --- | --- | --- |
| `lifecycle` | `transient`, `durable` | Controls whether the message survives the active turn. |
| `persistence` | `none`, `local`, `project_history` | `none` is forbidden for durable history; transient messages always use `none`. |
| `message_kind` | `activity`, `user`, `assistant`, `proposal`, `execution_receipt`, `verification`, `warning`, `error`, `system` | Controls copy and presentation. |
| `turn_id` | stable run/goal identity | Groups activity cleanup without becoming a message identity. |
| `logical_message_id` | stable message identity | Deduplicates the HTTP response, local cache, and Project History representation of one message. |
| `supersedes` | logical message IDs | Optional explicit replacement relationship. |

`turn_id` and `logical_message_id` are deliberately different. A Proposal and its later Receipt may share a turn but must not merge into one message.

## 3. Lifecycle invariants

1. Agent Events are `transient` / `none` / `activity`.
2. Transient activity is held outside the durable `messages` collection.
3. Transient activity is never written to localStorage or Project History.
4. `item.completed` closes its matching activity.
5. `approval.requested` closes execution activity; the durable Proposal is the only confirmation surface.
6. `turn.completed` and `turn.failed` close all activity for that turn.
7. HTTP responses and Project History rows for the same durable message share `logical_message_id` or `source_id`.
8. A Proposal has one durable carrier. Its action state may resolve, but its analysis text remains in history.
9. A Receipt or Verification is appended as a new durable message and never replaces its Proposal.
10. Legacy cached `agent_event_*` and `*_processing_*` rows are discarded during restore.

## 4. Copy system

### Activity — blue, transient

Pattern: `正在 + 动作 + 对象`.

- `正在读取 61 条轨道`
- `正在组装 B2 能力上下文`
- `正在验证 12 条推子`

Do not repeat evidence, risk, confirmation instructions, or final conclusions here.

### Proposal — yellow, durable

Order:

1. outcome/title;
2. evidence summary;
3. proposed changes;
4. limits and risk;
5. one natural-language confirmation sentence and contextual actions.

Proposal copy must stand on its own. Generic text such as “这个操作需要确认” is not a second message.

When a structured Proposal card is available, it is the message body. The same Proposal is not rendered again as an adjacent plain-text paragraph.

### Receipt / Verification — blue or red, durable

Lead with the outcome. State what changed, what was read back, verification status, and rollback/recovery information. Never claim success when verification is inconclusive.

### Error — red, durable

Pattern: `结果 + failed boundary + next safe action`.

- `B2 未写入工程：角色关系证据不足。补齐轨道角色后可重新生成方案。`

## 5. Compatibility

Old Project History nodes without protocol fields are interpreted as durable project-history messages. Old local cache rows with Agent Event or processing prefixes are treated as transient residue and are not restored. Existing history data is not deleted or rewritten.

## 6. Acceptance sequence

1. Submit a natural-language B2/B3 request.
2. Observe transient reading/planning activity in the activity lane.
3. Confirm that exactly one durable Proposal appears.
4. Confirm via button or natural language.
5. Observe transient execution/verification activity.
6. Confirm that one durable Receipt/Verification appears.
7. Refresh and switch history trees.
8. Confirm that only user messages, Proposal, confirmation utterance, and Receipt/Verification remain.
