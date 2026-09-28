# DOSE-AUDIBLE-1 验收取证工件

- 主树（新代码）run：artifacts/free_state_d1_s1/20260928_120230 与 _120725（未跟踪运行时工件）——剂量链四旗标全 true，死于 settle 链 'acoustic materiality record is missing'（同断点两次，§8 止损）。
- 基线复现（干净 HEAD worktree @8ab2c8f6，旧 ±2 代码）：115926=diclosure×断言形态（model selected broadband_compression without broadband_threshold_adjust）；121206=revision 无区分形态（applied=false）；115758/115818=前两轮取证。
- 两阻断均 main 既有，非本卡回归。结论链详见 rulings/2026-09-28-DOSE-AUDIBLE-1-pass.md。
