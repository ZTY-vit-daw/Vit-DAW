import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { TrajectoryMockView } from "./trajectory/TrajectoryMockView";
// UI-SPRITE-MOUNT-1（361）：v5 sprite 全局挂载——symbol 定义一次挂载全树可用，
// LauncherSurface/ProjectSurface 的 <use href="#i-*"> 引用由此生效。
// sprite.css 提供 .v5-sprite（0 尺寸隐藏）与 .v5-design .v5-icon 渲染契约；
// 不导入会退化成默认 300×150 裸 svg（样式泄漏级故障），故与挂载同处入口导入。
import { DesignSprite } from "./design/sprite";
import "./design/sprite.css";
import "./styles.css";

const surface = new URLSearchParams(window.location.search).get("surface")?.toLowerCase();

createRoot(document.getElementById("root") as HTMLElement).render(
  <StrictMode>
    <DesignSprite />
    {surface === "trajectory" ? <TrajectoryMockView /> : <App />}
  </StrictMode>
);
