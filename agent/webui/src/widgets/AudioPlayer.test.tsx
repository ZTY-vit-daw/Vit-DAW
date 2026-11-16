import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AudioPlayer, audioPlayerSourceURL } from "./AudioPlayer";

// WIDGET-KIT-1（379）：轻量播放器组件测试。
// 复用语义锚点：App.tsx renderArtifactPreview audio 分支 = <audio controls
// src={artifactFileURL(id)}>——本组件提取同一 preview URL 机制（lib/api
// artifactFileURL），不改动 App.tsx 本体。

describe("audioPlayerSourceURL（preview URL 机制复用）", () => {
  it("artifactID → 既有 artifact 文件 URL 通道（与资料预览同源）", () => {
    expect(audioPlayerSourceURL({ artifactID: "art-9" })).toBe("/agent/artifact/file?id=art-9");
  });

  it("显式 src 优先于 artifactID；两者皆缺返回空串（不可播，不伪造）", () => {
    expect(audioPlayerSourceURL({ src: "data:audio/wav;base64,AAAA", artifactID: "art-9" })).toBe("data:audio/wav;base64,AAAA");
    expect(audioPlayerSourceURL({})).toBe("");
    expect(audioPlayerSourceURL({ artifactID: "  " })).toBe("");
  });
});

describe("AudioPlayer 渲染", () => {
  it("收起态：标题+文件图标+时长+展开钮在场；控制面不在场（按需展开）", () => {
    const markup = renderToStaticMarkup(
      <AudioPlayer title="示范旋律.wav" artifactID="art-demo-1" />
    );
    expect(markup).toContain('data-widget-audio-player="示范旋律.wav"');
    expect(markup).toContain('data-audio-expanded="false"');
    expect(markup).toContain('data-audio-title="示范旋律.wav"');
    expect(markup).toContain("data-audio-expand-toggle");
    expect(markup).not.toContain("data-audio-progress");
    // preview URL 机制在场：<audio> 元素带既有 artifact 文件 URL
    expect(markup).toContain('src="/agent/artifact/file?id=art-demo-1"');
    expect(markup).toContain("data-audio-element");
    // 与资料预览 audio 分支同款 media-player 语义类
    expect(markup).toContain("media-player");
  });

  it("展开态（expandable=false）：播放/暂停钮+进度+时值控制面常驻", () => {
    const markup = renderToStaticMarkup(
      <AudioPlayer title="a.wav" src="data:audio/wav;base64,AAAA" expandable={false} />
    );
    expect(markup).toContain('data-audio-expanded="true"');
    expect(markup).toContain('data-audio-play-toggle="play"');
    expect(markup).toContain("data-audio-progress");
    expect(markup).toContain("data-audio-time");
    expect(markup).toContain("aria-label=\"播放 a.wav\"");
  });

  it("负向：无 artifactID 且无 src → 诚实降级（无 audio 元素、控制禁用、占位披露）", () => {
    // 收起态也披露不可播（根级 data-audio-unavailable）
    const collapsed = renderToStaticMarkup(<AudioPlayer title="缺失.wav" />);
    expect(collapsed).toContain('data-audio-unavailable="true"');
    expect(collapsed).not.toContain("<audio");
    expect(collapsed).not.toContain("/agent/artifact/file");
    // 展开态控制面整体禁用（不伪造可播状态）
    const expanded = renderToStaticMarkup(<AudioPlayer title="缺失.wav" expandable={false} />);
    expect(expanded).toContain('data-audio-unavailable="true"');
    expect(expanded).toContain('data-audio-play-toggle="play"');
    expect(expanded).toContain("disabled");
    expect(expanded).toContain("音频源未就绪");
    expect(expanded).not.toContain("<audio");
  });

  it("durationHint 只作声明坐标披露（data 属性），不冒充实测时长", () => {
    const markup = renderToStaticMarkup(
      <AudioPlayer title="b.wav" src="data:audio/wav;base64,AAAA" expandable={false} durationHint={42} />
    );
    expect(markup).toContain('data-audio-duration-hint="42"');
    // 实测时长缺测时显示 --:--（不拿 hint 冒充）
    expect(markup).toContain("--:--");
  });
});
