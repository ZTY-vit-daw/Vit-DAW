package chat

import (
	"os"
	"strings"
)

// CONTRACT-1 按域路由开关框架（三层面审计计划 §7 放行条件 2，2026-09-05）：
// 处理器选择路径（AGENT-1：static_eq/broadband_compression 迁入
// semantic_treatment_strategy 的插件选择链）的 D1 兼容开关必须——
//   1. 服务端：本文件在 Go 服务端读取，禁止 prompt 层切换；
//   2. 按域：VIT_AGENT_DOMAIN_ROUTES 逐域列出，未列出的域一律旧路径；
//   3. 可观测：每次路由决策把 domain/route/实际取值写入运行日志；
//   4. 预定义移除条件：AGENT-1 新路径在 D1 域冒烟 + 真实栈端侧烟测
//      （开放请求→处理器选择→执行→回读→A/B）两轮全绿后，把两域默认值
//      翻为 processor_selection，观察一个版本无回归即删除本开关与
//      legacy 分支（届时本文件只留移除记录）。
//
// 本框架已由 AGENT-1 消费：routeAcceptedImprovementProposalNativeDomain 在
// static_eq / broadband_compression 上按域取向（legacy_native 走原 mix-tick
// 拦截；processor_selection 冻结路由 marker 并放行到语义策略层），
// processor_selection.go 承接 A0 的持久化 selection 记录。

const (
	// DomainRouteLegacyNative 是现行路径：提案确认后直接落 pending mix tick
	// （static_eq/broadband_compression 现状，绕过处理器选择）。
	DomainRouteLegacyNative = "legacy_native"
	// DomainRouteProcessorSelection 是 AGENT-1 目标路径：进入语义处理器
	// 选择（existing_plugin / load_required / PCA 推荐）后再物化动作。
	DomainRouteProcessorSelection = "processor_selection"
)

const domainRoutingEnvName = "VIT_AGENT_DOMAIN_ROUTES"

// domainRouteOverrides 解析 "static_eq=processor_selection,broadband_compression=processor_selection"
// 形态的按域配置。域名小写归一；路由值只认两个预定义值，非法值忽略（回落
// 旧路径——开关是逃生门不是旁路）。
func domainRouteOverrides(raw string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		domain, route, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		domain = strings.ToLower(strings.TrimSpace(domain))
		route = strings.TrimSpace(route)
		if domain == "" || (route != DomainRouteLegacyNative && route != DomainRouteProcessorSelection) {
			continue
		}
		out[domain] = route
	}
	return out
}

// domainRouteFor 返回某 action domain 的路由取向并记录决策日志。默认
// legacy_native（放行条件 2：默认旧路径，新路径按域独立启用）。
func (s *Server) domainRouteFor(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	route := DomainRouteLegacyNative
	source := "default"
	if domain != "" {
		if mapped, ok := domainRouteOverrides(os.Getenv(domainRoutingEnvName))[domain]; ok {
			route = mapped
			source = domainRoutingEnvName
		}
	}
	if s != nil && s.logger != nil {
		s.logger.Info("[domain-route] domain=%s route=%s source=%s", domain, route, source)
	}
	return route
}
