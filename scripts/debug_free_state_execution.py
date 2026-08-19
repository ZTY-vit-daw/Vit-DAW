#!/usr/bin/env python3
"""Debug script for free-state reasoning loop execution breakpoint.

This script helps diagnose why LLM can converge in observation but cannot
enter the execution layer after outputting FreeStateNeedsAction.
"""

import json
import sys
from pathlib import Path

def check_free_state_context(context_file):
    """Check if free_state context has all required fields for execution."""

    if not Path(context_file).exists():
        print(f"❌ Context file not found: {context_file}")
        return False

    with open(context_file, 'r', encoding='utf-8') as f:
        data = json.load(f)

    print("=== Free State Reasoning Loop Execution Checkpoint ===\n")

    # Check free_state_reasoning_loop
    loop = data.get('free_state_reasoning_loop', {})
    if not loop:
        print("❌ free_state_reasoning_loop missing in context")
        return False

    print(f"✅ Loop ID: {loop.get('loop_id')}")
    print(f"   Status: {loop.get('status')}")
    print(f"   Decision Phase: {loop.get('decision_phase')}")

    # Check if awaiting_action
    if loop.get('status') != 'awaiting_action':
        print(f"⚠️  Status is '{loop.get('status')}', not 'awaiting_action'")
        print("    LLM may not have reached the execution decision yet.")
        return False

    print(f"✅ Status is 'awaiting_action'")

    # Check latest_decision
    decision = loop.get('latest_decision', {})
    if not decision:
        print("❌ latest_decision missing")
        return False

    print(f"\n=== Latest Decision ===")
    print(f"   Status: {decision.get('status')}")
    print(f"   Processor Type: {decision.get('processor_type')}")
    print(f"   Evidence Status: {decision.get('evidence_status')}")

    # Check semantic_processor_intent
    intent = decision.get('semantic_processor_intent', {})
    if not intent:
        print("❌ semantic_processor_intent missing in decision")
        print("   This is REQUIRED for execution!")
        return False

    print(f"\n✅ Semantic Processor Intent present:")
    print(f"   Schema: {intent.get('schema_version')}")
    print(f"   Status: {intent.get('status')}")
    print(f"   Family: {intent.get('family')}")
    print(f"   Control Mode: {intent.get('control_mode')}")
    print(f"   Required Coverage: {intent.get('required_coverage')}")

    if intent.get('status') != 'resolved':
        print(f"❌ Intent status is '{intent.get('status')}', not 'resolved'")
        return False

    # Check semantic_entry_decision
    entry = data.get('semantic_entry_decision', {})
    if not entry:
        print("\n⚠️  semantic_entry_decision missing")
        print("    This may block freeStateRouteAuthorized check")
    else:
        print(f"\n✅ Semantic Entry Decision:")
        print(f"   Route: {entry.get('route')}")
        print(f("   Control Mode: {entry.get('control_mode')}")
        print(f"   User Authorization: {entry.get('user_authorization')}")
        print(f"   Target Scope: {entry.get('target_scope')}")

        if entry.get('route') != 'open_semantic':
            print(f"❌ Route is '{entry.get('route')}', not 'open_semantic'")
            return False

        if entry.get('control_mode') != 'semantic_loop':
            print(f"❌ Control mode is '{entry.get('control_mode')}', not 'semantic_loop'")
            return False

        if entry.get('user_authorization') != 'action':
            print(f"❌ User authorization is '{entry.get('user_authorization')}', not 'action'")
            return False

    # Check target binding
    print(f"\n=== Target Binding ===")
    track_id = data.get('selected_track_id') or data.get('selected_plugin_track_id')
    if not track_id:
        print("❌ selected_track_id missing!")
        print("   This is REQUIRED if target_scope is 'current_selection'")

        target_scope = entry.get('target_scope', '')
        if target_scope == 'current_selection':
            print(f"   AND target_scope IS 'current_selection'")
            print("   → EXECUTION WILL FAIL")
            return False
        elif target_scope == 'project_context':
            print("   But target_scope is 'project_context', may still work")
        else:
            print(f"   Target scope is '{target_scope}' (unknown)")
    else:
        print(f"✅ Track ID: {track_id}")

    # Check if free_state_route_authorized flag is set
    if not data.get('free_state_route_authorized'):
        print("\n⚠️  free_state_route_authorized flag not set")
        print("    This should be set by goalrunner_chat.go:334")
    else:
        print("\n✅ free_state_route_authorized flag set")

    # Check free_state_processor_type
    processor_type = data.get('free_state_processor_type')
    if not processor_type:
        print("⚠️  free_state_processor_type not set")
    else:
        print(f"✅ Processor type: {processor_type}")

    # Check free_state_semantic_processor_intent in context
    context_intent = data.get('free_state_semantic_processor_intent')
    if not context_intent:
        print("⚠️  free_state_semantic_processor_intent not in context")
        print("    This should be set by goalrunner_chat.go:339-341")
    else:
        print("✅ free_state_semantic_processor_intent in context")

    print("\n=== Summary ===")
    print("All critical fields present for execution entry.")
    print("If execution still doesn't happen, check:")
    print("1. Agent logs for routeOrdinaryAgentTreatmentStrategy return value")
    print("2. Whether handleSemanticTreatmentForFreeState is being called")
    print("3. PCA (Plugin Capability Attestation) availability")

    return True


if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: python debug_free_state_execution.py <context_json_file>")
        print("\nYou can also provide context as stdin:")
        print("  cat agent_context.json | python debug_free_state_execution.py -")
        sys.exit(1)

    context_file = sys.argv[1]

    if context_file == '-':
        # Read from stdin
        import tempfile
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            f.write(sys.stdin.read())
            context_file = f.name

    try:
        success = check_free_state_context(context_file)
        sys.exit(0 if success else 1)
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(2)
