package check

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/submatrix/config"
	proxyutils "github.com/Wenaixi/submatrix/proxy"
	"github.com/Wenaixi/submatrix/save/method"
	"github.com/Wenaixi/submatrix/utils"
)

var (
	// CF 节点 (不一致): 匹配 US⁰
	reCFInconsistent = regexp.MustCompile(`(?:^|[^A-Z])([A-Z]{2})\x{2070}`)

	// CF 节点 (一致/Relay): 匹配 HK¹, SG⁺, HK¹⁺
	// 范围必须排除 U+2070（⁰）：\x{00B3}-\x{2079} 这一大段包含 ⁰，
	// 而 ⁰ 专属于 reCFInconsistent。不排除会让带 ⁰ 的节点被两条正则
	// 各加一次 Countries，地区之和大于 Total（实测 🇺🇸US⁰ 加出 2 份）。
	reCFConsistent = regexp.MustCompile(`(?:^|[^A-Z])([A-Z]{2})(?:[\x{00B9}\x{00B3}-\x{206F}]|[\x{2071}-\x{2079}]|[\x{207A}])+`)

	// CF 节点 (被墙/受限): 匹配 HK⁻¹
	reCFBlock = regexp.MustCompile(`(?:^|[^A-Z])([A-Z]{2})\x{207B}\x{00B9}`)

	// 非 CF 节点 (独立 VPS): 匹配 HK²
	reNonCF = regexp.MustCompile(`(?:^|[^A-Z])([A-Z]{2})\x{00B2}`)

	// 通用国旗匹配: 匹配 🇺🇸, 🇯🇵 等
	reFlag = regexp.MustCompile(`[\x{1F1E6}-\x{1F1FF}]{2}`)

	// 流媒体解锁特征
	reMediaGPT = regexp.MustCompile(`(?i)GPT`)
	reMediaGM  = regexp.MustCompile(`(?i)GM|Gemini`)
	reMediaCP  = regexp.MustCompile(`(?i)CP|Copilot`)
	reMediaTK  = regexp.MustCompile(`(?i)TK|TikTok`)
	reMediaYT  = regexp.MustCompile(`(?i)YT|YouTube`)
	reMediaNF  = regexp.MustCompile(`(?i)NF|Netflix`)
	reMediaDis = regexp.MustCompile(`(?i)D\+|Disney`)
)

// LastCheckResultStr 用于给 GUI 状态栏展示
var LastCheckResultStr atomic.Value

// globalAliveNodes 收集「本轮检测中所有组的可用节点」，按物理指纹去重。
//
// 为什么需要它：GenerateAnalysisReport 是 ProxyChecker 的方法，只统计本组结果，
// 而报告文件是固定文件名、每组覆盖一次，因此最终留下的 global_analysis
// 只反映最后检测的那一组。而用户看到的「地区分布」应该是所有组合并去重后的全貌。
//
// 去重键用 utils.GenerateProxyKey（server + 端口 + 凭据 + SNI/Host 的物理指纹），
// 同一节点被多个组引用时只算一次；同 IP 不同账号会保留为不同节点。
var (
	globalAliveMu    sync.Mutex
	globalAliveNodes = make(map[string]globalAliveNode)
	globalAliveOrder []string // 保留插入顺序，让报告输出稳定可复现
)

type globalAliveNode struct {
	Name  string
	Type  string
	Speed int
}

// ResetGlobalAliveNodes 清空全局节点集合，每轮完整检测开始时调用一次。
func ResetGlobalAliveNodes() {
	globalAliveMu.Lock()
	globalAliveNodes = make(map[string]globalAliveNode)
	globalAliveOrder = nil
	globalAliveMu.Unlock()
}

// CollectGroupAliveNodes 把某一组的可用节点并入全局集合（按物理指纹去重）。
func CollectGroupAliveNodes(results []Result) {
	globalAliveMu.Lock()
	defer globalAliveMu.Unlock()

	for _, r := range results {
		if r.Proxy == nil {
			continue
		}
		key := utils.GenerateProxyKey(r.Proxy)
		if key == "" {
			continue
		}
		if _, exists := globalAliveNodes[key]; exists {
			continue
		}
		name, _ := r.Proxy["name"].(string)
		pType, _ := r.Proxy["type"].(string)
		globalAliveNodes[key] = globalAliveNode{Name: name, Type: pType, Speed: r.Speed}
		globalAliveOrder = append(globalAliveOrder, key)
	}
}

// GlobalAliveCount 返回当前全局去重后的可用节点数。
func GlobalAliveCount() int {
	globalAliveMu.Lock()
	defer globalAliveMu.Unlock()
	return len(globalAliveNodes)
}

// AnalysisStats 统计结构
type AnalysisStats struct {
	Total      int
	Types      map[string]int
	Countries  map[string]int
	CFIncon    map[string]int // ⁰ (不一致)
	CFCon      map[string]int // ¹⁺ (一致)
	CFBlock    map[string]int // ⁻¹ (proxyIP 异常)
	NonCF      map[string]int // ² (独立VPS)
	Media      map[string]int
	SpeedSum   int // 速度总和
	SpeedCount int // 参与测速数量
}

func newAnalysisStats() *AnalysisStats {
	return &AnalysisStats{
		Types:     make(map[string]int),
		Countries: make(map[string]int),
		CFIncon:   make(map[string]int),
		CFCon:     make(map[string]int),
		CFBlock:   make(map[string]int),
		NonCF:     make(map[string]int),
		Media:     make(map[string]int),
	}
}

// applyNodeToStats 把单个可用节点计入统计。
//
// 抽成独立函数是为了让「单组报告」与「全局合并报告」共用同一套解析口径：
// 地区来自节点名里的国旗/角标（机场宣称的地区），协议来自 type 字段，
// 流媒体/AI 解锁来自名称特征。两处若各写一套正则，口径迟早会分叉。
func applyNodeToStats(s *AnalysisStats, name, pType string, speed int, speedON bool) {
	s.Total++
	s.Types[pType]++

	// 如果有速度则累计平均速度
	if speedON && speed > 0 {
		s.SpeedSum += speed
		s.SpeedCount++
	}

	// 节点属性识别
	hasTag := false
	// 由于增加了前缀匹配，submatch 的 index 1 才是真正的国家代码
	if m := reCFInconsistent.FindStringSubmatch(name); len(m) > 1 {
		s.CFIncon[m[1]]++
		s.Countries[m[1]]++
		hasTag = true
	}
	if m := reCFConsistent.FindStringSubmatch(name); len(m) > 1 {
		s.CFCon[m[1]]++
		s.Countries[m[1]]++
		hasTag = true
	}
	if m := reCFBlock.FindStringSubmatch(name); len(m) > 1 {
		s.CFBlock[m[1]]++
		s.Countries[m[1]]++
		hasTag = true
	}
	if m := reNonCF.FindStringSubmatch(name); len(m) > 1 {
		s.NonCF[m[1]]++
		s.Countries[m[1]]++
		hasTag = true
	}

	// 如果没有上角标，从国旗 Emoji 提取
	if !hasTag {
		matched := false
		if flags := reFlag.FindAllString(name, -1); len(flags) > 0 {
			for _, f := range flags {
				code := flagToCode(f)
				if code != "" {
					s.Countries[code]++
					matched = true
				}
			}
		}
		// 既无角标也无国旗：典型是 🏴‍☠️（海盗旗，多码点组合 emoji）这类
		// 机场用来标记「匿名/未知地区」的节点。显式记入 Unknown，
		// 保证「地区分布各项之和 == Total」，否则这些节点会静默消失，
		// 让统计看起来总是差几个人对不上。
		if !matched {
			s.Countries["Unknown"]++
		}
	}

	// AI解锁
	if reMediaGPT.MatchString(name) {
		if strings.Contains(name, "GPT⁺") {
			s.Media["GPT+"]++
		} else {
			s.Media["GPT"]++
		}
	}
	if reMediaGM.MatchString(name) {
		s.Media["Gemini"]++
	}
	if reMediaCP.MatchString(name) {
		s.Media["Copilot"]++
	}
	// 流媒体解锁
	if reMediaNF.MatchString(name) {
		s.Media["Netflix"]++
	}
	if reMediaYT.MatchString(name) {
		s.Media["YouTube"]++
	}
	if reMediaTK.MatchString(name) {
		s.Media["TikTok"]++
	}
	if reMediaDis.MatchString(name) {
		s.Media["Disney+"]++
	}
}

// GenerateAnalysisReport 生成节点质量分析报告
func (pc *ProxyChecker) GenerateAnalysisReport() {
	// 统计可用节点数量。
	//
	// 同一订阅链接可能被多个组引用（如 free-a 与 free-b 共用 5 个源），
	// 若逐条 ++ 累加，Success 会按组重复计数，与「取最大值」口径的 Total 不一致，
	// 导致存活率虚高。这里先汇总本组对该 URL 的可用数，再与已有值取较大者。
	localSuccess := make(map[string]int)
	for _, result := range pc.results {
		if result.Proxy != nil {
			if subURL, ok := result.Proxy["sub_url"].(string); ok {
				localSuccess[subURL]++
			}
		}
	}
	if len(localSuccess) > 0 {
		proxyutils.SubStatsMutex.Lock()
		for subURL, n := range localSuccess {
			stats := proxyutils.SubStats[subURL]
			if n > stats.Success {
				stats.Success = n
			}
			proxyutils.SubStats[subURL] = stats
		}
		proxyutils.SubStatsMutex.Unlock()
	}

	globalAnalysis := newAnalysisStats()
	subAnalysis := make(map[string]*AnalysisStats)

	for _, result := range pc.results {
		if result.Proxy == nil {
			continue
		}

		subURL, _ := result.Proxy["sub_url"].(string)
		pType, _ := result.Proxy["type"].(string)
		name, _ := result.Proxy["name"].(string)

		applyNodeToStats(globalAnalysis, name, pType, result.Speed, speedON)
		if subURL != "" {
			if _, ok := subAnalysis[subURL]; !ok {
				subAnalysis[subURL] = newAnalysisStats()
			}
			applyNodeToStats(subAnalysis[subURL], name, pType, result.Speed, speedON)
		}
	}

	// 排序
	// sortedURLs := make([]string, 0, len(subAnalysis))
	// for u := range subAnalysis {
	// 	sortedURLs = append(sortedURLs, u)
	// }

	// 从 SubStats 获取所有订阅 URL，包含成功率为 0 的订阅
	sortedURLs := make([]string, 0, len(proxyutils.SubStats))
	for u := range proxyutils.SubStats {
		sortedURLs = append(sortedURLs, u)
	}
	slices.SortFunc(sortedURLs, func(a, b string) int {
		statA, statB := proxyutils.SubStats[a], proxyutils.SubStats[b]
		rateA := float64(statA.Success) / float64(max(1, statA.Total))
		rateB := float64(statB.Success) / float64(max(1, statB.Total))
		return cmpFloat(rateB, rateA)
	})

	// 并保存订阅成功率统计并打印成功率过低日志
	checkSubsSuccessRate(subAnalysis, sortedURLs)
	// 保存深度分析报告
	saveDetailedAnalysis(globalAnalysis, subAnalysis, sortedURLs)

	// 终端输出总结
	logSummary(globalAnalysis)

	// 获取最终处理的节点总数 (直接复用上面报告里计算 checkCount 的逻辑)
	checkCount := Progress.Load()
	if config.GlobalConfig.ProgressMode == "stage" || ForceClose.Load() || Successlimited.Load() {
		checkCount = AliveCount.Load()
	}

	// 组装要在 GUI 菜单栏展示的一行内容
	// 例如："上次检测: 15:38 | 可用: 100/300 | 耗时: 1分30秒 | 消耗:  "
	guiLine := fmt.Sprintf("%s丨%s丨%s · %d/%d",
		time.Now().Format("15:04"), // 只显示 时:分 比较紧凑
		prettyDuration(CheckDuration),
		CheckTrafficTotal,
		globalAnalysis.Total, // 成功可用的节点数
		int(checkCount),      // 检测的总节点数
	)

	// 存入全局变量供 GUI 读取
	LastCheckResultStr.Store(guiLine)

	// 把本组的订阅源统计累积到全局，供最终合并报告复用。
	// 必须在 saveDetailedAnalysis 之后、且 subAnalysis 仍有效时执行。
	mergeSubAnalysis(subAnalysis)
}

// 全局订阅源统计：各组 GenerateAnalysisReport 把自己算好的 subAnalysis
// 累积到这里，供 WriteGlobalAnalysisReport 直接复用。
//
// 为什么不能从全局去重节点重建：节点的 sub_url 是「该节点来自哪个原始订阅源」
// （如 raw.githubusercontent.com/...），而 SubStats 的键是「本组拉取过哪些 URL」
// （聚合组拉的是前组的产物地址 sub-s.wenxi.dev/group/...）。两者是完全不同的
// URL 集合，实测交集为 0。硬按 sub_url 归类会让所有源都掉进 subs_ranking_bad。
//
// 合并口径与 SubStats 一致：同一 URL 在多组出现时取「较大的那份」，
// 避免聚合组与原始组重复计入。
var (
	globalSubAnalysisMu sync.Mutex
	globalSubAnalysis   = make(map[string]*AnalysisStats)
)

// ResetGlobalSubAnalysis 清空全局订阅源统计，每轮检测开始时调用。
func ResetGlobalSubAnalysis() {
	globalSubAnalysisMu.Lock()
	globalSubAnalysis = make(map[string]*AnalysisStats)
	globalSubAnalysisMu.Unlock()
}

// mergeSubAnalysis 把一组的订阅源统计并入全局，同 URL 取较大者。
func mergeSubAnalysis(local map[string]*AnalysisStats) {
	globalSubAnalysisMu.Lock()
	defer globalSubAnalysisMu.Unlock()
	for u, st := range local {
		if st == nil {
			continue
		}
		cur, ok := globalSubAnalysis[u]
		if !ok || cur == nil {
			globalSubAnalysis[u] = st
			continue
		}
		// 取「节点更多」的那份：同一个 URL 在不同组里节点数应相同，
		// 不一致时取大值，与 SubStats 的 Total/Success 取最大值口径保持一致。
		if st.Total > cur.Total {
			globalSubAnalysis[u] = st
		}
	}
}

// WriteGlobalAnalysisReport 用「所有组合并去重后」的可用节点重写报告。
//
// 背景：GenerateAnalysisReport 是单组的，每组检测结束时调用一次并把
// stats/subs-analysis.yaml 整个覆盖，因此磁盘上最终只剩最后一组的 global_analysis。
// 而报告里的「地区分布 / 协议分布 / 解锁情况」在用户看来应该是全部组的全貌。
//
// 本函数在所有组检测完成后调用一次：global_analysis 用全局去重节点重算，
// 订阅源级数据（subs_ranking）直接复用各组累积的 globalSubAnalysis
// （不能用去重节点的 sub_url 重建，两者 URL 集合完全不同，实测交集为 0）。
func WriteGlobalAnalysisReport(totalPulled int) {
	globalAliveMu.Lock()
	keys := make([]string, len(globalAliveOrder))
	copy(keys, globalAliveOrder)
	nodes := make(map[string]globalAliveNode, len(globalAliveNodes))
	for k, v := range globalAliveNodes {
		nodes[k] = v
	}
	globalAliveMu.Unlock()

	if len(keys) == 0 {
		slog.Warn("全局报告跳过：本轮没有任何可用节点")
		return
	}

	globalAnalysis := newAnalysisStats()
	for _, k := range keys {
		n := nodes[k]
		applyNodeToStats(globalAnalysis, n.Name, n.Type, n.Speed, speedON)
	}
	// 不变式兜底：地区分布之和必须等于节点总数。
	// 差 2 的历史成因尚未完全定位（疑与某些多码点 emoji 的码点边界有关），
	// 这里做最后一道保证：把差额计入 Unknown，让前端「地区之和」永远等于「可用节点数」，
	// 避免用户看到一组对不上的数字。同时打 Error 日志保留线索。
	{
		sum := 0
		for _, v := range globalAnalysis.Countries {
			sum += v
		}
		if diff := sum - globalAnalysis.Total; diff != 0 {
			slog.Error("地区统计不一致，差额计入 Unknown", "total", globalAnalysis.Total, "countries_sum", sum, "diff", diff)
			adj := globalAnalysis.Total - sum
			globalAnalysis.Countries["Unknown"] += adj
			if globalAnalysis.Countries["Unknown"] <= 0 {
				delete(globalAnalysis.Countries, "Unknown")
			}
		}
	}

	// 订阅源级统计取自各组累积结果（每组的 GenerateAnalysisReport 已 merge 进来）
	globalSubAnalysisMu.Lock()
	subAnalysis := make(map[string]*AnalysisStats, len(globalSubAnalysis))
	for u, st := range globalSubAnalysis {
		subAnalysis[u] = st
	}
	globalSubAnalysisMu.Unlock()

	// 订阅源级数据沿用当前 SubStats（已是跨组共享、按 URL 取最大值）
	sortedURLs := make([]string, 0, len(proxyutils.SubStats))
	proxyutils.SubStatsMutex.Lock()
	for u := range proxyutils.SubStats {
		sortedURLs = append(sortedURLs, u)
	}
	proxyutils.SubStatsMutex.Unlock()

	// 用与本组报告一致的排序口径
	slices.SortFunc(sortedURLs, func(a, b string) int {
		proxyutils.SubStatsMutex.Lock()
		statA, statB := proxyutils.SubStats[a], proxyutils.SubStats[b]
		proxyutils.SubStatsMutex.Unlock()
		rateA := float64(statA.Success) / float64(max(1, statA.Total))
		rateB := float64(statB.Success) / float64(max(1, statB.Total))
		return cmpFloat(rateB, rateA)
	})

	// 补齐「没有可用节点」的订阅源：它们仍应出现在 subs_ranking_bad 里，
	// saveDetailedAnalysis 用 subs[u] == nil 区分好/坏榜单，所以这里只补空条目。
	matched := 0
	for _, u := range sortedURLs {
		if st, ok := subAnalysis[u]; ok && st != nil {
			matched++
		} else {
			subAnalysis[u] = nil
		}
	}
	slog.Info("全局报告订阅源匹配情况",
		"subAnalysis键数", len(subAnalysis), "sortedURLs数", len(sortedURLs), "匹配数", matched)
	if matched == 0 && len(sortedURLs) > 0 {
		// 只在全军覆没时才打样本，避免日志噪音
		for u := range subAnalysis {
			slog.Warn("subAnalysis 样本键", "key", u)
			break
		}
		slog.Warn("sortedURLs 样本键", "key", sortedURLs[0])
	}

	saveDetailedAnalysis(globalAnalysis, subAnalysis, sortedURLs, totalPulled)
	slog.Info("全局合并报告已生成",
		"去重可用节点", globalAnalysis.Total,
		"全组拉取总数", totalPulled,
		"覆盖地区数", len(globalAnalysis.Countries))
}

// saveDetailedAnalysis 输出包含总结和可视化数据的报告。
// checkCountOverride > 0 时用它作为「探测节点总数」，
// 全局报告需要传全组累加值：包级 Progress 只保留最后一组的值，
// 若沿用会让前端「存活率 = 全局可用 / 单组测活」算出 900%+ 的荒谬数字。
func saveDetailedAnalysis(global *AnalysisStats, subs map[string]*AnalysisStats, sortedURLs []string, checkCountOverride ...int) {
	var sb strings.Builder
	sb.WriteString("# 检测结果分析报告\n")
	sb.WriteString("# 生成时间: ")
	sb.WriteString(time.Now().Format(time.DateTime))
	sb.WriteString("\n\n")

	// 1. 总结性文案 (用于快速预览)
	sb.WriteString("summary: |\n")
	summary := generateSummary(global)
	sb.WriteString("  ")
	sb.WriteString(summary)
	sb.WriteString("\n\n")

	checkCount := Progress.Load()
	if config.GlobalConfig.ProgressMode == "stage" || ForceClose.Load() || Successlimited.Load() {
		checkCount = AliveCount.Load()
	}
	// 全局报告用调用方给的「全组累加值」，避免与 alive_count 口径不一致
	if len(checkCountOverride) > 0 && checkCountOverride[0] > 0 {
		checkCount = uint32(checkCountOverride[0])
	}

	sb.WriteString("check_info:\n")
	sb.WriteString("  check_time: ")
	sb.WriteString(prettyTime(CheckStartTime))
	sb.WriteString("\n")
	sb.WriteString("  check_time_raw: ")
	sb.WriteString(CheckStartTime.Format(time.RFC3339))
	sb.WriteString("\n")
	sb.WriteString("  check_end_time_raw: ")
	sb.WriteString(CheckEndTime.Format(time.RFC3339))
	sb.WriteString("\n")
	sb.WriteString("  check_duration: ")
	sb.WriteString(prettyDuration(CheckDuration))
	sb.WriteString("\n")
	sb.WriteString("  check_duration_raw: ")
	sb.WriteString(strconv.FormatInt(int64(CheckDuration.Seconds()), 10))
	sb.WriteString("\n")
	sb.WriteString("  check_count: ")
	sb.WriteString(prettyTotal(int(checkCount)))
	sb.WriteString("\n")
	sb.WriteString("  check_count_raw: ")
	sb.WriteString(strconv.Itoa(int(checkCount)))
	sb.WriteString("\n")
	sb.WriteString("  check_traffic_total: ")
	sb.WriteString(CheckTrafficTotal)
	sb.WriteString("\n")
	sb.WriteString("  check_traffic_total_raw: ")
	sb.WriteString(strconv.FormatUint(TotalBytes.Load(), 10))
	sb.WriteString("\n")
	sb.WriteString("  check_traffic_upload_raw: ")
	sb.WriteString(strconv.FormatUint(UP.Load(), 10))
	sb.WriteString("\n")
	sb.WriteString("  check_traffic_download_raw: ")
	sb.WriteString(strconv.FormatUint(DOWN.Load(), 10))
	sb.WriteString("\n")
	// 计算本次检测所有订阅文件的大小
	var totalSubSize uint64
	for _, stat := range proxyutils.SubStats {
		totalSubSize += uint64(stat.Size)
	}
	sb.WriteString("  total_subs_size: ")
	sb.WriteString(utils.FormatTraffic(totalSubSize))
	sb.WriteString("\n")
	sb.WriteString("  total_subs_size_raw: ")
	sb.WriteString(strconv.FormatUint(totalSubSize, 10))
	sb.WriteString("\n")

	var speedText string
	if speedON {
		speedText = strconv.Itoa(config.GlobalConfig.MinSpeed)
	} else {
		speedText = "0"
	}
	sb.WriteString("  check_min_speed: ")
	sb.WriteString(speedText)
	sb.WriteString("\n")
	sb.WriteString("  check_success_limit: ")
	sb.WriteString(strconv.FormatInt(int64(config.GlobalConfig.SuccessLimit), 10))
	sb.WriteString("\n")
	sb.WriteString("\n")

	// 2. 全局统计 (可视化友好结构)
	sb.WriteString("global_analysis:\n")
	sb.WriteString("  alive_count: ")
	sb.WriteString(strconv.Itoa(global.Total))
	sb.WriteString("\n")
	sb.WriteString("  geography_distribution:")
	sb.WriteString(formatMap(global.Countries, "    "))
	sb.WriteString("\n")
	sb.WriteString("  protocol_distribution:")
	sb.WriteString(formatMap(global.Types, "    "))
	sb.WriteString("\n")

	sb.WriteString("  quality_metrics:\n")
	ratio := float64(getSum(global.CFCon)) / float64(max(1, global.Total)) * 100
	sb.WriteString("    cf_consistent_ratio: ")
	sb.WriteString(strconv.FormatFloat(ratio, 'f', 1, 64))
	sb.WriteString("%\n")

	sb.WriteString("    cf_details:\n")
	sb.WriteString("      consistent_¹⁺:")
	sb.WriteString(formatMap(global.CFCon, "        "))
	sb.WriteString("\n")
	sb.WriteString("      inconsistent_⁰:")
	sb.WriteString(formatMap(global.CFIncon, "        "))
	sb.WriteString("\n")
	sb.WriteString("      blocked_⁻¹:")
	sb.WriteString(formatMap(global.CFBlock, "        "))
	sb.WriteString("\n")
	sb.WriteString("    vps_details_²:")
	sb.WriteString(formatMap(global.NonCF, "      "))
	sb.WriteString("\n")

	// 3. 订阅排行与明细
	sb.WriteString("\nsubs_ranking:\n")

	var sbBad strings.Builder
	sbBad.WriteString("\nsubs_ranking_bad:\n")
	for _, u := range sortedURLs {
		st := subs[u] // 可能是 nil
		pStat := proxyutils.SubStats[u]
		rate := float64(pStat.Success) / float64(max(1, pStat.Total))

		if st != nil {
			sb.WriteString("  - url: ")
			// URL 必须 Quote：订阅地址可能带 #（组间引用常用 /group/x/all.yaml#tag），
			// YAML 会把裸写的 # 之后当成注释，导致该行截断。
			sb.WriteString(strconv.Quote(u))
			sb.WriteString("\n")
			sb.WriteString("    stats: { rate: ")
			sb.WriteString(strconv.FormatFloat(rate*100, 'f', 4, 64))
			sb.WriteString("%, success: ")
			sb.WriteString(strconv.Itoa(pStat.Success))
			sb.WriteString(", total: ")
			sb.WriteString(strconv.Itoa(pStat.Total))
			sb.WriteString(" }\n")
			// 输出平均速度
			if speedON && st.SpeedCount > 0 {
				avgSpeed := st.SpeedSum / st.SpeedCount
				sb.WriteString("    avg_speed: ")
				if avgSpeed < 100 {
					sb.WriteString(strconv.Itoa(avgSpeed))
					sb.WriteString(" KB/s\n")
				} else {
					sb.WriteString(strconv.FormatFloat(float64(avgSpeed)/1024, 'f', 1, 64))
					sb.WriteString(" MB/s\n")
				}
			}
			// 输出流量
			if pStat.Traffic > 0 {
				sb.WriteString("    traffic: ")
				sb.WriteString(utils.FormatTraffic(pStat.Traffic))
				sb.WriteString("\n")
			}
			// 输出该订阅文件的大小
			if pStat.Size > 0 {
				sb.WriteString("    file_size: ")
				sb.WriteString(utils.FormatTraffic(uint64(pStat.Size)))
				sb.WriteString("\n")
			}
			sb.WriteString("    protocols: { ")
			sb.WriteString(formatMapToInline(st.Types))
			sb.WriteString(" }\n")
			sb.WriteString("    locations: { ")
			sb.WriteString(formatMapToInline(st.Countries))
			sb.WriteString(" }\n")
			sb.WriteString("    top_locations: [")
			sb.WriteString(getTopKeys(st.Countries, 3))
			sb.WriteString("]\n")
		} else {
			sbBad.WriteString("  - url: ")
			sbBad.WriteString(strconv.Quote(u))
			sbBad.WriteString("\n")
			sbBad.WriteString("    stats: { rate: ")
			sbBad.WriteString(strconv.FormatFloat(rate*100, 'f', 4, 64))
			sbBad.WriteString("%, success: ")
			sbBad.WriteString(strconv.Itoa(pStat.Success))
			sbBad.WriteString(", total: ")
			sbBad.WriteString(strconv.Itoa(pStat.Total))
			sbBad.WriteString(" }\n")
			// 输出消耗的流量
			if pStat.Traffic > 0 {
				sbBad.WriteString("    traffic: ")
				sbBad.WriteString(utils.FormatTraffic(pStat.Traffic))
				sbBad.WriteString("\n")
			}
			// 输出该订阅文件的大小
			if pStat.Size > 0 {
				sbBad.WriteString("    file_size: ")
				sbBad.WriteString(utils.FormatTraffic(uint64(pStat.Size)))
				sbBad.WriteString("\n")
			}
			// 写入明确的失效原因
			// 必须用 strconv.Quote：ErrMsg 常是 Go 标准库的错误文本，
			// 形如 `Get "https://...": EOF`，本身自带双引号。
			// 手工用 "..." 包裹时内部引号不转义，YAML 会在此处截断成非法结构，
			// 导致整份报告解析失败、接口退化成只返回原始字符串（前端统计全 0）。
			if pStat.ErrMsg != "" {
				sbBad.WriteString("    error: ")
				sbBad.WriteString(strconv.Quote(pStat.ErrMsg))
				sbBad.WriteString("\n")
			}
		}
	}

	// 4. 远程订阅清单：每个清单拉取到的订阅链接及其归属关系。
	// 链接自身的统计（成功率/协议/地区/错误原因）已在上方 subs_ranking / subs_ranking_bad 中，
	// 前端按 URL 关联后展示在对应的远程订阅下方，这里不重复存一份。
	sbRemote := buildRemoteSubsYAML(proxyutils.SnapshotRemoteStats())

	_ = method.SaveToStats([]byte(sb.String()+sbBad.String()+sbRemote), "subs-analysis.yaml", "分析结果")
}

// buildRemoteSubsYAML 生成 remote_subs 段，结构示例：
//
//	remote_subs:
//	  - url: "https://example.com/list.txt"
//	    count: 12
//	    error: "..."        # 仅在清单拉取失败/解析不到链接时出现
//	    urls:
//	      - "https://a.example.com/sub"
//
// 字符串统一使用 strconv.Quote 输出双引号标量：其转义序列均为合法的 YAML 双引号转义，
// 可避免 URL 中的 # {} : 等字符被 YAML 误解析。
func buildRemoteSubsYAML(remotes []proxyutils.RemoteStat) string {
	if len(remotes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nremote_subs:\n")
	for _, r := range remotes {
		sb.WriteString("  - url: ")
		sb.WriteString(strconv.Quote(r.URL))
		sb.WriteString("\n    count: ")
		sb.WriteString(strconv.Itoa(r.Count))
		sb.WriteString("\n")
		if r.ErrMsg != "" {
			sb.WriteString("    error: ")
			sb.WriteString(strconv.Quote(r.ErrMsg))
			sb.WriteString("\n")
		}
		if len(r.URLs) == 0 {
			sb.WriteString("    urls: []\n")
			continue
		}
		sb.WriteString("    urls:\n")
		for _, u := range r.URLs {
			sb.WriteString("      - ")
			sb.WriteString(strconv.Quote(u))
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// generateSummary 生成单段落详细摘要
func generateSummary(s *AnalysisStats) string {
	if s.Total == 0 {
		return "未探测到有效节点数据，请检查订阅源。"
	}

	// 1. 基础统计
	topCountry := getTopKeys(s.Countries, 1)
	cfConCount := getSum(s.CFCon)
	cfRatio := float64(cfConCount) / float64(max(1, s.Total)) * 100
	vpsCount := getSum(s.NonCF)
	vpsRatio := float64(vpsCount) / float64(max(1, s.Total)) * 100

	// 2. 线路特征描述
	lineFeature := "线路分布多样"
	if cfRatio > 70 {
		lineFeature = "以 Cloudflare 中转代理为主"
	} else if vpsRatio > 50 {
		lineFeature = "以 VPS 为主"
	}

	// 3. 分类获取流媒体和 AI 的前几名
	topMedia := getTopFiltered(s.Media, []string{"Netflix", "YouTube", "Disney+", "TikTok"}, 5)
	topAI := getTopFiltered(s.Media, []string{"GPT", "GPT+", "Gemini", "Copilot"}, 4)

	var speedText string
	if speedON {
		speedText = "，设置速度下限 " + strconv.Itoa(config.GlobalConfig.MinSpeed) + " KB/s"
	} else {
		speedText = "，未开启下载测速"
	}

	return "用时 " + prettyDuration(CheckDuration) +
		",消耗流量 " + CheckTrafficTotal +
		", 检测到 " + prettyTotal(s.Total) + " 个可用节点" + speedText + "。" +
		"覆盖 " + strconv.Itoa(len(s.Countries)) + " 个国家/地区 [Top: " + topCountry + "]; " +
		lineFeature + " [CF 中转 " + strconv.FormatFloat(cfRatio, 'f', 1, 64) +
		"%, VPS " + strconv.FormatFloat(vpsRatio, 'f', 1, 64) + "%]; " +
		"流媒体解锁: [" + topMedia + "]; AI 解锁[" + topAI + "]; " +
		"代理协议: " + getTopKeys(s.Types, 10) + "。"
}

// logSummary 终端结构化输出
func logSummary(s *AnalysisStats) {
	if s.Total == 0 {
		slog.Warn("分析完成：未发现有效节点")
		return
	}

	cfRatio := float64(getSum(s.CFCon)) / float64(max(1, s.Total)) * 100
	vpsRatio := float64(getSum(s.NonCF)) / float64(max(1, s.Total)) * 100

	slog.Info("检测摘要",
		"耗时", prettyDuration(CheckDuration),
		"CF", strconv.FormatFloat(cfRatio, 'f', 0, 64)+"%",
		"VPS", strconv.FormatFloat(vpsRatio, 'f', 0, 64)+"%",
		// "媒体解锁", getTopFiltered(s.Media, []string{"Netflix", "YouTube", "Disney+", "TikTok"}, 5),
		// "AI解锁", getTopFiltered(s.Media, []string{"GPT", "GPT+", "Gemini"}, 3),
		"协议", getTopKeys(s.Types, 10),
	)
}

// 工具函数
func prettyTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("01-02 15:04") // 月-日 时:分
}

func prettyDuration(d time.Duration) string {
	sec := int(d.Seconds())
	switch {
	case sec >= 3600:
		// 超过 60 分钟只显示分钟
		return strconv.Itoa(sec/60) + "分"
	case sec >= 60:
		return strconv.Itoa(sec/60) + "分 " + strconv.Itoa(sec%60) + "秒"
	default:
		return strconv.Itoa(sec) + "秒"
	}
}

func prettyTotal(n int) string {
	switch {
	case n >= 1000000:
		return strconv.Itoa(n/10000) + "万"
	case n >= 10000:
		return strconv.FormatFloat(float64(n)/10000.0, 'f', 1, 64) + "万"
	default:
		return strconv.Itoa(n)
	}
}

// getTopFiltered 根据白名单过滤并返回前 N 个统计项
func getTopFiltered(m map[string]int, filter []string, limit int) string {
	type kv struct {
		K string
		V int
	}
	var filtered []kv
	filterMap := make(map[string]bool)
	for _, f := range filter {
		filterMap[f] = true
	}

	for k, v := range m {
		// 检查是否在过滤名单内 (如果是 GPT-US 等前缀匹配也可在此调整)
		if filterMap[k] || strings.HasPrefix(k, "YT") || strings.HasPrefix(k, "TK") {
			filtered = append(filtered, kv{k, v})
		}
	}

	slices.SortFunc(filtered, func(a, b kv) int { return b.V - a.V })

	var parts []string
	for i := 0; i < len(filtered) && i < limit; i++ {
		parts = append(parts,
			filtered[i].K+":"+strconv.Itoa(filtered[i].V),
		)
	}

	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, ", ")
}

func formatMap(m map[string]int, indent string) string {
	if len(m) == 0 {
		return " {}"
	}
	type kv struct {
		K string
		V int
	}
	var res []kv
	for k, v := range m {
		res = append(res, kv{k, v})
	}
	slices.SortFunc(res, func(a, b kv) int { return b.V - a.V })
	var out strings.Builder
	out.WriteString("\n")
	for _, item := range res {
		fmt.Fprintf(&out, "%s%s: %d\n", indent, item.K, item.V)
	}
	return strings.TrimRight(out.String(), "\n")
}

// formatMapToInline 将 map 转换为内联字符串: "ss: 10, trojan: 5"
func formatMapToInline(m map[string]int) string {
	if len(m) == 0 {
		return ""
	}
	type kv struct {
		K string
		V int
	}
	var res []kv
	for k, v := range m {
		res = append(res, kv{k, v})
	}
	// 按数量降序排列
	slices.SortFunc(res, func(a, b kv) int { return b.V - a.V })

	var parts []string
	for _, item := range res {
		parts = append(parts, item.K+": "+strconv.Itoa(item.V))
	}

	return strings.Join(parts, ", ")
}

func getTopKeys(m map[string]int, limit int) string {
	if len(m) == 0 {
		return ""
	}
	type kv struct {
		K string
		V int
	}
	var res []kv
	for k, v := range m {
		res = append(res, kv{k, v})
	}
	slices.SortFunc(res, func(a, b kv) int { return b.V - a.V })
	var keys []string
	for i := 0; i < len(res) && i < limit; i++ {
		keys = append(keys, res[i].K)
	}
	return strings.Join(keys, "|")
}

func getSum(m map[string]int) int {
	s := 0
	for _, v := range m {
		s += v
	}
	return s
}

// checkSubsSuccessRate 将成功率筛选与协议统计整合输出
func checkSubsSuccessRate(subs map[string]*AnalysisStats, sortedURLs []string) {
	threshold := config.GlobalConfig.SuccessRate
	var goodPart, lowPart, zeroPart strings.Builder

	// 1. 遍历并分类
	for _, u := range sortedURLs {
		st := subs[u]
		pStat := proxyutils.SubStats[u]

		rate := 0.0
		if pStat.Total > 0 {
			rate = float64(pStat.Success) / float64(pStat.Total)
		}

		// 构造协议字符串
		protoStr := ""
		if st != nil && len(st.Types) > 0 {
			type kv struct {
				k string
				v int
			}
			var protos []kv
			for k, v := range st.Types {
				protos = append(protos, kv{k, v})
			}
			slices.SortFunc(protos, func(a, b kv) int { return b.v - a.v })

			var protoParts []string
			for _, p := range protos {
				protoParts = append(protoParts, p.k+": "+strconv.Itoa(p.v))

			}
			protoStr = "[" + strings.Join(protoParts, "; ") + "]"
		}

		// 格式化行：- URL # 46.667% (7/15) ; vless: 8
		line := "  - " + u + " # " + strconv.FormatFloat(rate*100, 'f', 4, 64) + "% (" + strconv.Itoa(pStat.Success) + "/" + strconv.Itoa(pStat.Total) + ")" + protoStr + "\n"

		// 分类逻辑
		switch {
		case pStat.Success == 0:
			zeroPart.WriteString(line)

		case rate < threshold:
			slog.Warn("订阅成功率过低: "+u,
				"Rate", strconv.FormatFloat(rate*100, 'f', 1, 64)+"%",
				"Count", strconv.Itoa(pStat.Success)+"/"+strconv.Itoa(pStat.Total),
			)
			lowPart.WriteString(line)

		default:
			goodPart.WriteString(line)
		}
	}

	// 2. 组装最终 YAML 内容
	var finalSB strings.Builder
	finalSB.WriteString("# 订阅质量统计报告\n")
	finalSB.WriteString("# 生成时间: ")
	finalSB.WriteString(time.Now().Format(time.DateTime))
	finalSB.WriteString("\n\n")

	if goodPart.Len() > 0 {
		finalSB.WriteString("# 达标订阅列表 (>=")
		finalSB.WriteString(strconv.FormatFloat(threshold*100, 'f', 2, 64))
		finalSB.WriteString("%)\nsub-urls:\n")
		finalSB.WriteString(goodPart.String())
		finalSB.WriteString("\n")
	}

	if lowPart.Len() > 0 {
		finalSB.WriteString("# 未达标订阅列表 (<")
		finalSB.WriteString(strconv.FormatFloat(threshold*100, 'f', 2, 64))
		finalSB.WriteString("%)\nsub-urls-low:\n")
		finalSB.WriteString(lowPart.String())
		finalSB.WriteString("\n")
	}

	if zeroPart.Len() > 0 {
		finalSB.WriteString("# 成功率为 0 的订阅\nsub-urls-bad:\n")
		finalSB.WriteString(zeroPart.String())
	}

	// 3. 保存文件
	_ = method.SaveToStats([]byte(finalSB.String()), "subs-filter.yaml", "订阅统计")
}

// flagToCode 将 Emoji 国旗转换为两位 ISO 国家代码 (例如 🇯🇵 -> JP)
func flagToCode(flag string) string {
	runes := []rune(flag)
	if len(runes) != 2 {
		return ""
	}
	// Regional Indicator Symbol A is U+1F1E6. 'A' is U+0041.
	// Difference is 0x1F1A5
	c1 := runes[0] - 0x1F1A5
	c2 := runes[1] - 0x1F1A5
	return string([]rune{c1, c2})
}

// 辅助函数
func cmpFloat(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// CleanupMetadata 清理元数据，并为输出结果进行 map 隔离复制
func (pc *ProxyChecker) CleanupMetadata() {
	for i, result := range pc.results {
		if result.Proxy != nil {
			// 1. 创建一个新的干净 Map (浅拷贝)
			cleanProxy := make(map[string]any, len(result.Proxy))

			// 2. 将除了需要清理的元数据之外的字段复制到新 Map
			//
			// group_proxy 必须剥离：它是组级前置代理的完整 URL，可能含
			// http://user:pass@host 形式的账密，而这些 Map 会落盘到
			// all.yaml / all-raw.yaml / history.yaml 并对外分发。
			for k, v := range result.Proxy {
				if k == "sub_url" || k == "sub_tag" || k == "history_score" || k == "group_proxy" {
					continue
				}
				cleanProxy[k] = v
			}

			// 3. 将新 Map 替换回 results 数组中
			// 这样 all.yaml 会用这个干净的 Map
			// 而 history.yaml 依然保留着最初拥有 sub_url 和 history_score 的完整 Map
			pc.results[i].Proxy = cleanProxy
		}
	}
}
