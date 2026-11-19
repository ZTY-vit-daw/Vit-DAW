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
