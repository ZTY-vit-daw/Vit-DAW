export const composerLayout = {
  main: "composer-main",
  inputRow: "composer-input-row",
  controlsRow: "composer-controls-row",
  controlsLeft: "composer-controls-left",
  controlsRight: "composer-controls-right",
  authority: "authority-mode-control",
  srOnly: "sr-only"
} as const;

export const composerControlOrder = ["attachment", "authority", "send_or_stop"] as const;
