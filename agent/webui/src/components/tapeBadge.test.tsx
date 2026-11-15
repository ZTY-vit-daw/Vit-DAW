/*
 * UI-REC-TAPE-1 磁带 cassette 头像单元测试（静态渲染 + 纯函数）。
 *
 * 造型冻结源：main-window-v3/SPEC.md §3 tapeSVG（结构/令牌/带量公式逐字）；
 * 氛围系统源：launcher-route-mondrian-v2.html:474-499 tape_atmosphere.gd 移植
 * （FNV-1a 含非 ASCII 补轮 → 6 色对 → c1/c2/mid + L/R 带量）。
 * 场用位裁定：用户裁定 2026-11-15 第 1 条（启动台工程列表=cassette 造型；
 * i-project 简化标志保留给其余 ≤48px 场用位，SPEC-V5 §7.3.1 修订段）。
 * fixture 纪律：本文件工程名均为测试 fixture（与 gallery fixture 同源命名），
 * 不进生产代码。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TapeBadge, tapeAtmos, tapeAtmosHash, TAPE_ATMOS_PALETTE } from "./ProjectBadge";

const markup = (jsx: React.ReactElement) => renderToStaticMarkup(jsx);

/** gallery fixture 同源三工程名（氛围色确定性断言用）。 */
const FIXTURE_NAMES = ["示例工程甲（fixture）", "示例工程乙（fixture）", "示例工程丙（fixture）"];

describe("TapeBadge 磁带 cassette 头像（UI-REC-TAPE-1）", () => {
  it("24px（recents 档）：磁带盒拓扑在场（壳+窗+双带轮+底标线），24 网格等比简化档", () => {
    const html = markup(<TapeBadge size={24} name={FIXTURE_NAMES[0]} />);
    expect(html).toContain("data-tape-badge=\"atmos\"");
    expect(html).toContain("data-tape-shell");
    expect(html).toContain("data-tape-window");
    expect(html.match(/data-tape-reel/g) ?? []).toHaveLength(2);
    expect(html).toContain("data-tape-label");
    expect(html).toContain("data-tape-sublabel");
    // <48px 不走冻结几何（冻结源 20px 级直绘窗口宽=0）——24 网格简化档
    expect(html).toContain('viewBox="0 0 24 24"');
    expect(html).toContain('width="24"');
  });

  it("20px（建议卡档）：viewBox 24 网格按显示尺寸缩放，拓扑不缺项", () => {
    const html = markup(<TapeBadge size={20} name={FIXTURE_NAMES[1]} />);
    expect(html).toContain('viewBox="0 0 24 24"');
    expect(html).toContain('width="20"');
    expect(html.match(/data-tape-reel/g) ?? []).toHaveLength(2);
    expect(html).toContain("data-tape-window");
  });

  it("≥48px 自动切冻结 tapeSVG 几何（viewBox=显示尺寸，结构笔触 2px）", () => {
    const html = markup(<TapeBadge size={48} name={FIXTURE_NAMES[0]} />);
    expect(html).toContain('viewBox="0 0 48 48"');
    expect(html).toContain('stroke-width="2"');
    expect(html.match(/data-tape-reel/g) ?? []).toHaveLength(2);
  });

  it("冻结令牌逐字（main-window-v3/SPEC.md §3）", () => {
    const html = markup(<TapeBadge size={24} name={FIXTURE_NAMES[0]} />);
    // 墨描边/暖纸窗/浅毂/深盘全部在卷
    expect(html).toContain('stroke="#090B0E"');
    expect(html).toContain('fill="#FBF9F1"');
    expect(html).toContain('fill="#FAFAF2"');
    expect(html).toContain('fill="#1A1A1F"');
    expect(html).toContain('stroke="rgba(31,36,43,0.6)"');
  });

  it("氛围色=工程名哈希：同名字节级确定（FNV-1a）", () => {
    expect(tapeAtmosHash("")).toBe(0x811c9dc5); // FNV-1a offset basis（空串原样）
    const a1 = tapeAtmos(FIXTURE_NAMES[0]);
    const a2 = tapeAtmos(FIXTURE_NAMES[0]);
    expect(a1).toEqual(a2);
    // 非 ASCII 补轮路径（中文工程名 >0xFF 码元参与第二字节轮）
    const ascii = tapeAtmos("demo-project");
    const cjk = tapeAtmos("示例工程甲");
    expect(tapeAtmosHash("示例工程甲")).not.toBe(tapeAtmosHash("demo-project"));
    expect(ascii.c1).not.toEqual(cjk.c1); // 弱断言：不同名落在不同色对（域内 6 对非全碰撞）
  });

  it("氛围色域：c1 必为 6 色对之一（tape_atmosphere.gd 移植域）", () => {
    for (const name of [...FIXTURE_NAMES, "小星星乐局", "ProQ3 裸控实验", "鼓组混音 v2"]) {
      const a = tapeAtmos(name);
      const domain = new Set(TAPE_ATMOS_PALETTE.map((p) => p[0].join(",")));
      expect(domain.has(a.c1.join(","))).toBe(true);
    }
  });

  it("带量公式（§7.4/v2 atmos）：L/R∈[0.25,0.8] 且 R=1.05-L", () => {
    for (const name of [...FIXTURE_NAMES, "小星星乐局", "鼓组混音 v2"]) {
      const a = tapeAtmos(name);
      expect(a.L).toBeGreaterThanOrEqual(0.25);
      expect(a.L).toBeLessThanOrEqual(0.8);
      expect(a.R).toBeGreaterThanOrEqual(0.25);
      expect(a.R).toBeLessThanOrEqual(0.8);
      expect(a.R).toBeCloseTo(1.05 - a.L, 10);
    }
  });

  it("氛围壳填充 = rgba(c1,0.62)（冻结公式），三 fixture 行色值可辨", () => {
    const shells = FIXTURE_NAMES.map((name) => {
      const a = tapeAtmos(name);
      const html = markup(<TapeBadge size={24} name={name} />);
      return {
        expected: `fill="rgba(${a.c1[0]},${a.c1[1]},${a.c1[2]},0.62)"`,
        html,
      };
    });
    for (const s of shells) {
      expect(s.html).toContain(s.expected);
    }
    // 三行氛围至少两色（哈希分布弱断言——避免全碰撞导致列表无氛围差）
    expect(new Set(shells.map((s) => s.expected)).size).toBeGreaterThanOrEqual(2);
  });

  it("无工程名：中性灰壳满盘（冻结源 null 氛围语义）", () => {
    const html = markup(<TapeBadge size={20} />);
    expect(html).toContain('data-tape-badge="neutral"');
    expect(html).toContain('fill="#E6E6E6"'); // TINT 灰壳
    expect(html).not.toContain("0.62)"); // 无氛围填充
  });

  it("空白工程名视同缺省（trim 后空串→中性）", () => {
    const html = markup(<TapeBadge size={20} name="   " />);
    expect(html).toContain('data-tape-badge="neutral"');
  });
});
