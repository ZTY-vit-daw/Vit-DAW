import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchLauncherRecentProjects } from "./launcher";

afterEach(() => vi.unstubAllGlobals());

describe("401 read-only recent projects", () => {
  it("uses only GET and preserves existence and governed directive", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      status: "ok", recent_count: 1, project_result_cards: [{
        id: "fixture", name: "fixture", file_path: "X:/fixture.vit", exists: false,
        launcher_directive: { action: "open_project", file_path: "X:/fixture.vit", requires_governance: true }
      }]
    })));
    vi.stubGlobal("fetch", fetchMock);
    const cards = await fetchLauncherRecentProjects();
    expect(fetchMock).toHaveBeenCalledExactlyOnceWith("/agent/launcher/recent-projects", { method: "GET" });
    expect(cards[0].exists).toBe(false);
    expect(cards[0].directive?.requires_governance).toBe(true);
  });

  it("accepts an explicit successful empty array", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"status":"ok","project_result_cards":[]}')));
    await expect(fetchLauncherRecentProjects()).resolves.toEqual([]);
  });

  it.each([
    [503, '{"status":"error","kernel_unavailable":true,"error":"offline"}'],
    [200, '{"status":"ok","project_result_cards":null}'],
    [200, '{"status":"error","project_result_cards":[]}'],
    [200, "not json"],
    [200, ""]
  ])("rejects error/malformed response instead of reporting empty (%s)", async (status, body) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(body, { status })));
    await expect(fetchLauncherRecentProjects()).rejects.toThrow();
  });
});
