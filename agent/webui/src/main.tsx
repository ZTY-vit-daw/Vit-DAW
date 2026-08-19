import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { TrajectoryMockView } from "./trajectory/TrajectoryMockView";
import "./styles.css";

const surface = new URLSearchParams(window.location.search).get("surface")?.toLowerCase();

createRoot(document.getElementById("root") as HTMLElement).render(
  <StrictMode>
    {surface === "trajectory" ? <TrajectoryMockView /> : <App />}
  </StrictMode>
);
