package tools

import (
	"strings"
	"testing"
)

// CAPABILITYPACK-KEYADVERT-1：mix 包提示词曾把 track.{id}.slow.time_energy.summary
// 宣传为稳定 catalog 键，但 full_project 观察上真实 track id 恒 miss
//（mixboard readObservationKey 的 time_energy case 臂只按观察自身 TargetRef 键控，
// full_project 包 TargetRef={id:"current"}；292 MIXBOARD-TRACKKEY-1 路由只覆盖
// stereo 双面）。宣传面修正：该键按观察自身目标 id 宣传（full_project 时为
// track.current.*），跨轨 time_energy 须走 per-track 观察，不宣传为 full_project
// 逐轨键。本测试钉住修正后的诚实形态不回退。
func TestMixPackCatalogKeyAdvertStatesOwnTargetKeying(t *testing.T) {
	packs := DefaultCatalog().CapabilityPacks([]string{"mix"})
	if len(packs) != 1 {
		t.Fatalf("mix capability packs = %+v", packs)
	}
	pack := packs[0]
	prose := strings.Join(pack.ResultHints, "\n") + "\n" +
		strings.Join(pack.StateSlices, "\n") + "\n" +
		strings.Join(pack.Examples, "\n")
	if strings.Contains(prose, "track.{id}.") {
		t.Fatalf("mix pack still advertises per-track-id catalog key template track.{id}.*:\n%s", prose)
	}
	for _, want := range []string{
		"observation's own target id",
		"track.current",
		"per-track observation",
	} {
		if !strings.Contains(prose, want) {
			t.Fatalf("mix pack catalog-key advert missing honest form %q:\n%s", want, prose)
		}
	}
	// 修正不得丢失真实可读的稳定键宣传。
	for _, want := range []string{
		"observation.digest",
		"project.tracks.summary",
		"project.rankings.level",
		"project.risks.headroom",
	} {
		if !strings.Contains(prose, want) {
			t.Fatalf("mix pack stable-key advert lost real key %q:\n%s", want, prose)
		}
	}
}

// 顺检反钉（全包）：任何能力包的宣传 prose 不得（再）出现 track.{id}.*
// 逐轨键模板——逐轨 id 键在 full_project 观察上不可稳定读取，宣传即误导。
func TestCapabilityPackProseAvoidsPerTrackIdKeyTemplate(t *testing.T) {
	for _, pack := range defaultCapabilityPacks() {
		prose := pack.Description + "\n" +
			pack.Domain + "\n" +
			strings.Join(pack.StateSlices, "\n") + "\n" +
			strings.Join(pack.Preconditions, "\n") + "\n" +
			strings.Join(pack.Effects, "\n") + "\n" +
			strings.Join(pack.ResultHints, "\n") + "\n" +
			strings.Join(pack.Verification, "\n") + "\n" +
			strings.Join(pack.Examples, "\n")
		if strings.Contains(prose, "track.{id}") {
			t.Fatalf("capability pack %s advertises per-track-id catalog key template:\n%s", pack.Name, prose)
		}
	}
}
