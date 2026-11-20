/*
 * TUTOR-SIM-2（池序 436）：live 服务接缝测试。
 *
 * 覆盖：/agent/tutor/start 请求形状、ok 字段映射、unavailable/异常降级（不抛进 UI）、
 * composer 发送在导师会话存在时用服务端 conversation_id 寻址 /agent/chat。
 * 网络层用 vi.stubGlobal("fetch") 替身，不触真实 agent。
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({
  apiPath: (path: string) => path,
  invokeAgent: vi.fn(),
  fetchUIState: vi.fn(),
  sendChat: vi.fn(),
  stopTurn: vi.fn(),
  startTutorSim: vi.fn()
}));

vi.mock("../lib/api", () => api);

import {
  createProjectLiveService,
  projectContextFromUIState,
  tutorMaterialLoadOutcomeFromResponse,
  tutorStartOutcomeFromResponse
} from "./projectService";

afterEach(() => {
  vi.unstubAllGlobals();
});

beforeEach(() => {
  vi.clearAllMocks();
});

describe("TUTOR-SIM-2：工程路径进入上下文（自动开场触发器）", () => {
  it("projectContextFromUIState 暴露 project_path，工程名与轨道行不变", () => {
    const view = projectContextFromUIState({
      status: "ok",
      project: { project_path: "D:/projects/tutor-436/tutor.vit" },
      tracks: [{}, {}]
    });
    expect(view).toEqual({
      name: "tutor.vit",
      source: "agent",
      detail: "2 条轨道",
      path: "D:/projects/tutor-436/tutor.vit"
    });
  });
});

describe("TUTOR-SIM-2：startTutorSim 回执映射", () => {
  it("ok：携带 project_path 请求，回执映射为导师会话 + 开场 + 步骤", async () => {
    api.startTutorSim.mockResolvedValueOnce({
      status: "ok",
      conversation_id: "tutor_live_436",
      reply: "  第 1/3 步：先听。  ",
      step_index: 1,
      step_total: 3,
      step_id: "step-1"
    });
    const service = createProjectLiveService();
    await expect(service.startTutorSim("D:/projects/tutor-436/tutor.vit")).resolves.toEqual({
      status: "ok",
      conversationID: "tutor_live_436",
      reply: "第 1/3 步：先听。",
      stepIndex: 1,
      stepTotal: 3,
      stepID: "step-1"
    });
    expect(api.startTutorSim).toHaveBeenCalledWith({ project_path: "D:/projects/tutor-436/tutor.vit" });
  });

  it("unavailable / 网络异常 / 缺会话 id：一律降级 unavailable，不抛进 UI", async () => {
    const service = createProjectLiveService();
    api.startTutorSim.mockResolvedValueOnce({ status: "unavailable", reason: "sidecar_missing" });
    await expect(service.startTutorSim("D:/p.vit")).resolves.toEqual({
      status: "unavailable",
      reason: "sidecar_missing"
    });

    api.startTutorSim.mockRejectedValueOnce(new Error("network down"));
    await expect(service.startTutorSim("D:/p.vit")).resolves.toEqual({
      status: "unavailable",
      reason: "network down"
    });

    api.startTutorSim.mockResolvedValueOnce({ status: "ok", reply: "缺少会话 id" });
    await expect(service.startTutorSim("D:/p.vit")).resolves.toEqual({
      status: "unavailable",
      reason: "missing_conversation_id"
    });
  });

  it("HTTP 错误回执（status=error）也按 unavailable 处理", async () => {
    api.startTutorSim.mockResolvedValueOnce({ status: "error", error: "invalid tutor_sim.yaml" });
    const service = createProjectLiveService();
    await expect(service.startTutorSim("D:/p.vit")).resolves.toEqual({
      status: "unavailable",
      reason: "invalid tutor_sim.yaml"
    });
  });

  it("tutorStartOutcomeFromResponse：纯函数映射", () => {
    expect(tutorStartOutcomeFromResponse({ status: "unavailable", reason: "no_project" })).toEqual({
      status: "unavailable",
      reason: "no_project"
    });
    expect(tutorStartOutcomeFromResponse({ status: "ok" })).toEqual({
      status: "unavailable",
      reason: "missing_conversation_id"
    });
    expect(
      tutorStartOutcomeFromResponse({
        status: "ok",
        conversation_id: "tutor_x",
        reply: " 你好 ",
        step_index: 2,
        step_total: 3,
        step_id: "step-2"
      })
    ).toEqual({ status: "ok", conversationID: "tutor_x", reply: "你好", stepIndex: 2, stepTotal: 3, stepID: "step-2" });
  });
});

describe("TUTOR-SIM-2：composer 会话寻址", () => {
  it("有导师 conversation_id：用它 POST /agent/chat（tutor 分支）", async () => {
    api.sendChat.mockResolvedValueOnce({ reply: "通过", workflow: "tutor_sim" });
    const service = createProjectLiveService();
    const outcome = await service.sendMessage("7", "tutor_live_436");
    expect(outcome.text).toBe("通过");
    expect(api.sendChat).toHaveBeenCalledWith({ conversation_id: "tutor_live_436", message: "7" });
  });

  it("透传 workflow/workflow_data 供 tutor 进度卡消费", async () => {
    api.sendChat.mockResolvedValueOnce({
      reply: "进入下一步。",
      workflow: "tutor_sim",
      workflow_data: { step_index: 2, step_total: 3, completed: false }
    });
    const service = createProjectLiveService();
    const outcome = await service.sendMessage("7", "tutor_live_436");
    expect(outcome.workflow).toBe("tutor_sim");
    expect(outcome.workflowData).toEqual({ step_index: 2, step_total: 3, completed: false });
  });

  it("无导师会话：维持本 surface 既有普通会话", async () => {
    api.sendChat.mockResolvedValueOnce({ reply: "收到" });
    const service = createProjectLiveService();
    await service.sendMessage("你好");
    const payload = api.sendChat.mock.calls[0]?.[0] as { conversation_id: string; message: string };
    expect(payload.message).toBe("你好");
    expect(payload.conversation_id).toMatch(/^webui_project_/);
  });
});

describe("TUTOR-SIM-2：lib/api.startTutorSim 真实请求形状", () => {
  it("POST /agent/tutor/start（可选字段序列化）", async () => {
    vi.resetModules();
    const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ status: "ok", conversation_id: "tutor_http_436", reply: "开场" }), { status: 200 })
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ status: "error", error: "llm_config_incomplete" }), { status: 503 })
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(actual.startTutorSim({ project_path: "D:/projects/tutor-436/tutor.vit" })).resolves.toMatchObject({
      status: "ok",
      conversation_id: "tutor_http_436"
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/agent/tutor/start",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ project_path: "D:/projects/tutor-436/tutor.vit" })
      })
    );
    const headers = (fetchMock.mock.calls[0]?.[1] as RequestInit).headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");

    // 两字段均可选：空载荷也要是合法 JSON。
    await expect(actual.startTutorSim()).rejects.toThrow("llm_config_incomplete");
    expect(fetchMock.mock.calls[1]?.[1]?.body).toBe("{}");
  });
});

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（438）：材料装载接缝                                      */
/* ------------------------------------------------------------------ */

describe("TUTOR-SIM-4：loadTutorMaterial 请求形状与回执映射", () => {
  it("装载请求 POST /agent/tutor/load_material，成功回执映射 track/clip/音源", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          status: "ok",
          conversation_id: "tutor_live_438",
          load: { loaded: true, track_id: "trk-1", clip_id: "clip-1", plugin_id: "pl-1", instrument_attached: true }
        }),
        { status: 200 }
      )
    );
    vi.stubGlobal("fetch", fetchMock);
    const actual = createProjectLiveService();
    const outcome = await actual.loadTutorMaterial("tutor_live_438", "主题材料");
    expect(outcome).toEqual({
      ok: true,
      alreadyLoaded: false,
      trackID: "trk-1",
      clipID: "clip-1",
      pluginID: "pl-1",
      instrumentAttached: true
    });
    const [path, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/agent/tutor/load_material");
    expect(init.method).toBe("POST");
    expect(JSON.parse(String(init.body))).toEqual({ conversation_id: "tutor_live_438", material_title: "主题材料" });
  });

  it("already_loaded 与音源被拒都算 ok（best-effort 如实带回）；失败映射 ok:false", () => {
    expect(
      tutorMaterialLoadOutcomeFromResponse({ status: "ok", already_loaded: true, load: { loaded: true, track_id: "t", clip_id: "c" } })
    ).toMatchObject({ ok: true, alreadyLoaded: true, instrumentAttached: false });
    expect(
      tutorMaterialLoadOutcomeFromResponse({
        status: "ok",
        load: { loaded: true, track_id: "t", clip_id: "c", instrument_attached: false, instrument_error: "plugin not found" }
      })
    ).toEqual({
      ok: true,
      alreadyLoaded: false,
      trackID: "t",
      clipID: "c",
      pluginID: "",
      instrumentAttached: false,
      instrumentError: "plugin not found"
    });
    expect(tutorMaterialLoadOutcomeFromResponse({ status: "error", error: "unknown_material" })).toEqual({
      ok: false,
      reason: "unknown_material"
    });
    expect(tutorMaterialLoadOutcomeFromResponse({ status: "error", failure_class: "kernel_error" })).toEqual({
      ok: false,
      reason: "kernel_error"
    });
    expect(tutorMaterialLoadOutcomeFromResponse({})).toEqual({ ok: false, reason: "load_failed" });
  });

  it("装载网络失败降级 ok:false，不抛进 UI", async () => {
    const fetchMock = vi.fn().mockRejectedValueOnce(new Error("socket closed"));
    vi.stubGlobal("fetch", fetchMock);
    const actual = createProjectLiveService();
    await expect(actual.loadTutorMaterial("c", "t")).resolves.toEqual({ ok: false, reason: "socket closed" });
  });
});

describe("TUTOR-SIM-4：start 回执携带 brief/materials（workflow_data 局部扩展）", () => {
  it("v1 开场投影 brief/materials；v0（无 workflow_data）两字段缺省空", () => {
    const v1 = tutorStartOutcomeFromResponse({
      status: "ok",
      conversation_id: "tutor_v1",
      reply: "开场",
      step_index: 1,
      step_total: 4,
      step_id: "step-hear-theme",
      workflow_data: {
        step_index: 1,
        step_total: 4,
        step_id: "step-hear-theme",
        brief: "教学简报",
        materials: [{ kind: "midi_clip", title: "主题", path: "assets/theme.mid", note_count: 42, loaded: false }]
      }
    } as never);
    expect(v1.status).toBe("ok");
    if (v1.status === "ok") {
      expect(v1.brief).toBe("教学简报");
      expect(v1.materials?.[0]).toMatchObject({ kind: "midi_clip", title: "主题", noteCount: 42, loaded: false });
    }
    const v0 = tutorStartOutcomeFromResponse({
      status: "ok",
      conversation_id: "tutor_v0",
      reply: "开场",
      step_index: 1,
      step_total: 3,
      step_id: "step-1"
    });
    if (v0.status === "ok") {
      // v0 回执无 workflow_data：输出不含 brief/materials 键（436 严格映射面）。
      expect(v0.brief).toBeUndefined();
      expect(v0.materials).toBeUndefined();
      expect(Object.keys(v0).sort()).toEqual(["conversationID", "reply", "status", "stepID", "stepIndex", "stepTotal"]);
    }
  });
});

describe("TUTOR-INPUT-TYPE-1：start 回执投影 answerKind", () => {
  it("workflow_data.answer_kind=text → text；number → number；v0（无 workflow_data）不含键", () => {
    const text = tutorStartOutcomeFromResponse({
      status: "ok",
      conversation_id: "tutor_kind_text",
      reply: "开场",
      step_index: 1,
      step_total: 4,
      step_id: "step-confirm",
      workflow_data: { step_index: 1, step_total: 4, step_id: "step-confirm", answer_kind: "text" }
    } as never);
    if (text.status === "ok") {
      expect(text.answerKind).toBe("text");
    }
    const number = tutorStartOutcomeFromResponse({
      status: "ok",
      conversation_id: "tutor_kind_number",
      reply: "开场",
      step_index: 1,
      step_total: 4,
      step_id: "step-listen",
      workflow_data: { step_index: 1, step_total: 4, step_id: "step-listen", answer_kind: "number" }
    } as never);
    if (number.status === "ok") {
      expect(number.answerKind).toBe("number");
    }
    const legacy = tutorStartOutcomeFromResponse({
      status: "ok",
      conversation_id: "tutor_kind_legacy",
      reply: "开场",
      step_index: 1,
      step_total: 3,
      step_id: "step-1"
    });
    if (legacy.status === "ok") {
      // 旧服务端 v0 回执（无 workflow_data）：不含 answerKind 键（436 键面不变，
      // UI 按缺省 number 渲染数字框）。
      expect(legacy.answerKind).toBeUndefined();
      expect(Object.keys(legacy)).not.toContain("answerKind");
    }
  });
});
