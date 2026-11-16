import { useEffect, useRef, useState } from "react";
import { artifactFileURL } from "../lib/api";
import "./widgets.css";

// WIDGET-KIT-1（池序 379）：轻量音频播放器（控件收敛报告 §1 行 1）。
//
// 复用边界：资料预览 audio 分支的既有语义与 preview URL 机制——App.tsx
// renderArtifactPreview 的音频分支为 `<audio className="media-player" controls
// src={artifactFileURL(artifact.id)}>`（App.tsx:11496 起），本组件提取同一
// artifactFileURL 通道与同款 <audio> 元素语义，补文件名/播放图标/进度/时长/
// 按需展开的轻量外壳。App.tsx 本体只读不改（其收敛属 368 阶段 1 域）；真实
// 内核预览平面接线留后续真栈卡（卡面边界），当前播放通道=浏览器 audio 元素
// 直连（artifact 文件 URL 或显式 src）。
//
// 播放器只是 UI 呈现，不携带任何试听会话/实验身份；比较用途见 ComparePair。

export type AudioPlayerMediaRef = {
  /** 工件 id：走既有 preview URL 机制（artifactFileURL） */
  artifactID?: string;
  /** 显式音频 URL（fixture/外部直链），优先级高于 artifactID */
  src?: string;
  /** 文件名/材料标签（播放器头部展示） */
  label: string;
};

/** 与 App.tsx renderArtifactPreview 同源：artifactID → 既有 preview URL 通道 */
export function audioPlayerSourceURL(media: { artifactID?: string; src?: string }): string {
  if (media.src && media.src.trim() !== "") {
    return media.src.trim();
  }
  const artifactID = (media.artifactID ?? "").trim();
  if (artifactID !== "") {
    return artifactFileURL(artifactID);
  }
  return "";
}

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "--:--";
  }
  const total = Math.floor(seconds);
  const minutes = Math.floor(total / 60);
  const rest = total % 60;
  return `${String(minutes).padStart(2, "0")}:${String(rest).padStart(2, "0")}`;
}

/**
 * 轻量音频播放器：文件名/播放图标、播放/暂停、进度、时长、按需展开。
 * 收起态=标题行+播放按钮+时长（普通试听不强制展开控制面）；展开态=进度拖拽。
 * 无有效媒体引用时诚实降级（渲染占位、禁用播放），不伪造可播状态。
 */
export function AudioPlayer({
  title,
  artifactID,
  src,
  defaultExpanded = false,
  expandable = true,
  durationHint,
  pauseNonce = 0,
  onPlaybackChange
}: {
  title: string;
  artifactID?: string;
  src?: string;
  /** 初始展开进度控制面（默认收起，按需展开） */
  defaultExpanded?: boolean;
  /** false = 固定展开（卡内小空间场景） */
  expandable?: boolean;
  /** 调用方声明的时长参考（秒）：只作元数据缺测时的坐标披露，不冒充实测 */
  durationHint?: number;
  /** 比较互斥缝：nonce 变化即暂停本实例（ComparePair 切换侧时 bump 对侧） */
  pauseNonce?: number;
  /** 播放状态上报（比较互斥与试听事件回流用） */
  onPlaybackChange?: (playing: boolean) => void;
}) {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const [expanded, setExpanded] = useState(defaultExpanded || !expandable);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(Number.NaN);
  const pauseNonceRef = useRef(pauseNonce);
  useEffect(() => {
    if (pauseNonce !== pauseNonceRef.current) {
      pauseNonceRef.current = pauseNonce;
      audioRef.current?.pause();
    }
  }, [pauseNonce]);
  const url = audioPlayerSourceURL({ artifactID, src });
  const playable = url !== "";
  const durationKnown = Number.isFinite(duration) && duration > 0;
  const displayTitle = title.trim() !== "" ? title.trim() : "音频";

  const togglePlay = () => {
    const audio = audioRef.current;
    if (!audio || !playable) return;
    if (audio.paused) {
      void audio.play().catch(() => setPlaying(false));
    } else {
      audio.pause();
    }
  };

  const seek = (value: number) => {
    const audio = audioRef.current;
    if (!audio || !durationKnown) return;
    const next = Math.min(Math.max(0, value), duration);
    audio.currentTime = next;
    setCurrentTime(next);
  };

  return (
    <div
      className={["widget-audio", playing ? "playing" : "", playable ? "" : "unavailable"].filter(Boolean).join(" ")}
      data-widget-audio-player={displayTitle}
      data-audio-expanded={expanded ? "true" : "false"}
      data-audio-playing={playing ? "true" : "false"}
      data-audio-unavailable={playable ? undefined : "true"}
    >
      <div className="widget-audio-head">
        <span className="widget-audio-icon" aria-hidden="true">
          <svg viewBox="0 0 12 12" className="ic-play" data-audio-file-icon="true">
            <path d="M2.5 1.5 10.5 6 2.5 10.5Z" fill="currentColor" />
          </svg>
        </span>
        <span className="widget-audio-title" data-audio-title={displayTitle}>{displayTitle}</span>
        <span
          className="widget-audio-time"
          data-audio-duration={durationKnown ? String(Math.round(duration)) : "unknown"}
          data-audio-duration-hint={typeof durationHint === "number" && Number.isFinite(durationHint) && durationHint > 0 ? String(durationHint) : undefined}
        >
          {formatTime(durationKnown ? duration : Number.NaN)}
        </span>
        {expandable && (
          <button
            type="button"
            className="widget-audio-expand"
            data-audio-expand-toggle="true"
            aria-expanded={expanded}
            aria-label={`${expanded ? "收起" : "展开"} ${displayTitle} 控制面`}
            onClick={() => setExpanded((current) => !current)}
          >
            {expanded ? "收起" : "展开"}
          </button>
        )}
      </div>
      {expanded && (
        <div className="widget-audio-body" data-audio-body="true">
          <button
            type="button"
            className="widget-audio-play"
            data-audio-play-toggle={playing ? "pause" : "play"}
            aria-label={`${playing ? "暂停" : "播放"} ${displayTitle}`}
            disabled={!playable}
            onClick={togglePlay}
          >
            <svg viewBox="0 0 12 12" className={playing ? "ic-pause" : "ic-play"} aria-hidden="true">
              {playing ? (
                <path d="M2.5 1.5h2.6v9H2.5zM6.9 1.5h2.6v9H6.9z" fill="currentColor" />
              ) : (
                <path d="M2.5 1.5 10.5 6 2.5 10.5Z" fill="currentColor" />
              )}
            </svg>
          </button>
          <input
            type="range"
            className="widget-audio-progress"
            data-audio-progress="true"
            min={0}
            max={durationKnown ? duration : 0}
            step={0.1}
            value={Math.min(currentTime, durationKnown ? duration : 0)}
            aria-label={`${displayTitle} 播放进度`}
            disabled={!playable || !durationKnown}
            onChange={(event) => seek(Number(event.currentTarget.value))}
          />
          <span className="widget-audio-time" data-audio-time={formatTime(currentTime)}>
            {formatTime(currentTime)} / {formatTime(duration)}
          </span>
          {!playable && (
            <span className="widget-audio-hint" data-audio-unavailable="true">音频源未就绪</span>
          )}
        </div>
      )}
      {playable && (
        <audio
          ref={audioRef}
          className="media-player widget-audio-element"
          src={url}
          preload="metadata"
          data-audio-element="true"
          onPlay={() => {
            setPlaying(true);
            onPlaybackChange?.(true);
          }}
          onPause={() => {
            setPlaying(false);
            onPlaybackChange?.(false);
          }}
          onEnded={() => {
            setPlaying(false);
            onPlaybackChange?.(false);
            if (durationKnown) setCurrentTime(duration);
          }}
          onTimeUpdate={(event) => setCurrentTime(event.currentTarget.currentTime)}
          onLoadedMetadata={(event) => {
            const value = event.currentTarget.duration;
            if (Number.isFinite(value) && value > 0) {
              setDuration(value);
            }
          }}
        />
      )}
    </div>
  );
}
