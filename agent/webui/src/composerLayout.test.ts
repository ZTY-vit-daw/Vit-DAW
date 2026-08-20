import { describe, expect, it } from "vitest";
import { composerControlOrder, composerLayout } from "./composerLayout";

describe("Composer visual layout contract", () => {
  it("uses explicit input and control rows", () => {
    expect(composerLayout.inputRow).toBe("composer-input-row");
    expect(composerLayout.controlsRow).toBe("composer-controls-row");
    expect(composerLayout.controlsLeft).toBe("composer-controls-left");
    expect(composerLayout.controlsRight).toBe("composer-controls-right");
    expect(composerControlOrder).toEqual(["attachment", "authority", "send_or_stop"]);
  });

  it("keeps authority text accessible without making it a visible heading", () => {
    expect(composerLayout.authority).toBe("authority-mode-control");
    expect(composerLayout.srOnly).toBe("sr-only");
  });

  it("keeps the main Composer as one compact surface", () => {
    expect(composerLayout.main).toBe("composer-main");
    expect(composerControlOrder).not.toContain("textarea" as never);
  });
});
