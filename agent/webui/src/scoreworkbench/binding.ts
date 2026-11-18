import type { AgentUIState, JsonRecord } from "../types";

export type ScoreTarget = {
  projectKey: string;
  trackID: string;
  clipID: string;
  trackName: string;
  clipName: string;
};

export type ScoreTrack = {
  trackID: string;
  name: string;
  clips: Array<{ clipID: string; name: string }>;
  otherClipCount: number;
  unknownClipCount: number;
};

export type ScoreCatalog = { projectKey: string; tracks: ScoreTrack[] };
export type ScoreSessionState = { dirty: boolean; busy: boolean; loaded: boolean };

function text(value: unknown): string {
  return typeof value === "string" || typeof value === "number" ? String(value).trim() : "";
}

// /agent/ui/state.project discloses project_path, not project_uuid. Never infer
// identity from the title or the last GUI selection (which can be stale).
export function scoreCatalog(state: AgentUIState): ScoreCatalog {
  const projectKey = text(state.project?.project_path).replace(/\\/g, "/");
  if (!projectKey) throw new Error("工程身份不可得，请重新打开工程后刷新轨道");
  const tracks: ScoreTrack[] = [];
  for (const row of state.tracks ?? []) {
    const trackID = text(row.track_id ?? row.id);
    if (!trackID) continue;
    const track: ScoreTrack = {
      trackID, name: text(row.name ?? row.track_name) || "未命名轨道",
      clips: [], otherClipCount: 0, unknownClipCount: 0
    };
    for (const clip of (Array.isArray(row.clips) ? row.clips : []) as JsonRecord[]) {
      const clipID = text(clip.clip_id ?? clip.id);
      if (!clipID) continue;
      const kind = text(clip.clip_type).toLowerCase();
      if (kind === "midi") track.clips.push({ clipID, name: text(clip.name) || "未命名片段" });
      else if (!kind) track.unknownClipCount++;
      else track.otherClipCount++;
    }
    tracks.push(track);
  }
  return { projectKey, tracks };
}

export function scoreTargetKey(target: ScoreTarget | null): string {
  return target ? JSON.stringify([target.projectKey, target.trackID, target.clipID]) : "";
}

export function targetInCatalog(catalog: ScoreCatalog | null, target: ScoreTarget | null): boolean {
  return !!catalog && !!target && catalog.projectKey === target.projectKey &&
    catalog.tracks.some((track) => track.trackID === target.trackID && track.clips.some((clip) => clip.clipID === target.clipID));
}

export function targetFor(catalog: ScoreCatalog, track: ScoreTrack, clipID: string): ScoreTarget | null {
  const clip = track.clips.find((item) => item.clipID === clipID);
  return clip ? { projectKey: catalog.projectKey, trackID: track.trackID, clipID, trackName: track.name, clipName: clip.name } : null;
}
