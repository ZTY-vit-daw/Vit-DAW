package semanticeffect

// StaticEQActionPromptExample is the single LLM-facing JSON example for an
// ordinary generic static-EQ leaf. B4 wraps the same leaf in a project batch;
// it does not own or redefine the horizontal EQ language.
const StaticEQActionPromptExample = `{"schema_version":"semantic_effect_action.v1","action_type":"eq_edit","payload_schema":"semantic_effect.eq_plan.v1","target":{"track_id":"exact supplied id","plugin_id":"exact supplied id"},"user_goal":"acoustic listening goal","negative_constraints":[],"evidence_decision":{"choice":"derive","basis":"observation","reason":"why this evidence supports the move","observation_id":"supplied observation id","evidence_refs":[]},"limitations":[],"eq_plan":{"schema_version":"semantic_effect.eq_plan.v1","atomic":true,"atoms":[{"atom_id":"stable semantic id","action":"upsert","shape":"bell","frequency_hz":180,"gain_db":-1.5,"q":1.1,"purpose":"distinct acoustic purpose","field_origins":{"frequency_hz":"llm_selected","gain_db":"llm_selected","q":"llm_selected"},"evidence_refs":[],"confidence":"medium"}]}}`

// StaticEQAtomPromptRules mirrors EQAtom.Validate for the fields the LLM must
// decide. Deterministic code must reject missing musical semantics rather than
// inventing defaults such as purpose or confidence.
const StaticEQAtomPromptRules = `Every EQ atom must include atom_id, action="upsert", shape, frequency_hz, purpose, confidence, and field_origins for every numeric field it supplies. confidence is low, medium, or high. bell/low_shelf/high_shelf also require gain_db. low_cut/high_cut forbid gain_db. q and slope_db_per_oct are optional only when the supplied topology proves them writable. purpose must state that atom's distinct acoustic job; it may not be empty or synthesized by deterministic code.`
