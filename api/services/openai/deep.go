package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"aipoly/api/services/polymarket"
)

// DeepAnalyzeMarkets decides bet direction and size from shortlisted dossiers (news-first).
// Amount must be in [item_min, 3×item_min]; the engine clamps to that band.
func (s *Service) DeepAnalyzeMarkets(
	dossiers []polymarket.MarketDossier,
	pctx MarketPromptContext,
	maxDecisions int,
) (*AnalysisResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY / MOONSHOT_API_KEY not configured")
	}
	maxTrades := pctx.MaxTradesPerCycle
	if maxTrades <= 0 {
		maxTrades = 1
	}
	if maxDecisions > 0 {
		maxTrades = maxDecisions
	}
	if maxTrades > len(dossiers) {
		maxTrades = len(dossiers)
	}

	placeLimit := pctx.MaxPlacePerCycle
	if placeLimit <= 0 {
		placeLimit = 2
	}
	if placeLimit > maxTrades {
		placeLimit = maxTrades
	}

	system := fmt.Sprintf(`你是 Polymarket 方向与仓位决策员：对初筛入选的最多 %d 个候选做详尽对比（含新闻等），明确每个可交易盘的方向、胜率与下注金额。
核心目标：减少亏损、增加净收益（不是追求最高胜率数字）。

只输出 JSON:
{"summary":"中文简述","decisions":[{"should_trade":true,"market_id":"id","outcome":"方向名","amount":3.5,"market_reason":"中文详细","outcome_reason":"中文详细","sources_cited":"查阅了哪些资料","analysis_process":"分析步骤","self_win_rate":68,"confidence":"medium","risk_level":"medium"}]}

硬性:
- 候选来自初筛（通常 ≤%d 个）；请逐个评估；合格的都写入 decisions（最多 %d 条），按 self_win_rate 从高到低排序
- 系统最终只从达标盘中选取「最可能获胜」的最多 %d 笔下注（self_win_rate ≥ %.0f）；未达门槛的不下；若全部不达标则本轮零下注
- 务必诚实填 self_win_rate，禁止为凑数抬高
- 主要依据：News(RSSHub) + 题面赔率；订单簿/成交/评论为辅助
- 有 News 时：必须在 sources_cited / outcome_reason 中引用具体标题（可写「新闻1: …」），优先用标注了「Xh ago / Xd ago」的最新稿；过旧新闻权重低
- 无 News / IntelGaps 含 rsshub：只能依据赔率/订单簿/成交，self_win_rate 更保守，禁止编造新闻
- 合格：诚实 self_win_rate ≥ %.0f；一旦方向明确且达门槛就 should_trade=true
- self_win_rate 必须明显高于买入价（¢）至少约 2 个百分点，否则引擎会因无 edge 拒单
- self_win_rate 必须是 0–100 的数字（例如 68），禁止填 0、禁止用 0.68、禁止只在文字里写胜率却漏填字段
- amount（美元名义）必填：对该 outcome 取提示中的 size_floor～item_max（即 1～3 倍最低额；默认建议 1×）；引擎会钳制到该区间
- 仓位原则（控制单笔亏损 + 费后净收益）:
  · 达标且可交易 → 默认 1×（约交易所最小名义）；仅高确信 + 买入价有合理上行（约 52–82¢）→ 偏 2～3×
  · 禁止无故顶到 3×：单笔过大时一错就亏很多，优先小仓位
  · 同一赛事（同一 Event）只允许一笔敞口：已有 BO3/整场持仓时，不要再选同场 Game1/让分/大小；引擎会拒绝对同事件叠仓
  · 买入价 ≥86¢ 的薄利大热：优先 should_trade=false（赢了也几乎全被手续费吃掉）；不要用小仓位硬凑
  · 有清晰方向且 edge 达标时，请写入 should_trade=true（不要只在 summary 里说「建议下注」却空 decisions）
  · 宁可少下：只选「赢了净利明显 > 手续费」且自评胜率明显高于盘口的盘；不要追求虚高胜率数字
  · 加仓（addable）时 amount 不得超过提示的 max_add
  · 禁止 amount=0；禁止超过 item_max；不得超过可部署资金
- 禁止编造未提供的新闻/数据
- 方向不清、五五开、锁盘、blocked、自评不足门槛、薄利高价 → should_trade=false 或不要写入；零合格时 decisions=[]

理由字段（每条 decision 必填）:
1) market_reason（≥40字）
2) outcome_reason（≥40字；有新闻则写清新闻如何支持该方向）
3) sources_cited（≥18字；有新闻则点名标题）
4) analysis_process（≥30字；须简要说明为何选该 amount 倍数）`,
		maxTrades, maxTrades, maxTrades, placeLimit, pctx.MinWinRate, pctx.MinWinRate)

	user := buildDeepPrompt(dossiers, pctx, maxTrades, placeLimit)
	req := chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat:  &responseFormat{Type: "json_object"},
		Temperature:     modelTemperature(s.model),
		ReasoningEffort: modelReasoningEffort(s.model),
	}
	body, _ := json.Marshal(req)

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		result, err := s.postChatCompletion(body)
		if err == nil {
			result.Decisions = SelectTradeDecisionsAllowZeroAmount(result.Decisions, maxTrades)
			NormalizeDecisionWinRates(result.Decisions)
			return result, nil
		}
		lastErr = err
		if !isRetryableOpenAIErr(err) {
			break
		}
	}
	return nil, lastErr
}

func buildDeepPrompt(dossiers []polymarket.MarketDossier, pctx MarketPromptContext, maxTrades, placeLimit int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`Portfolio:
- Equity $%.2f | Deployable $%.2f | Open $%.2f (%d)
- Sizing: AI chooses amount in [1×, 3×] item minimum (default 1×); engine clamps + cash-fraction cap
- Goal: small tickets + net PnL (avoid thin ≥86¢ favorites; one open ticket per event; size up only on clear edges)
- Min win %.0f%% | evaluate up to %d shortlisted candidates
- System will pick the TOP %d by self_win_rate among those ≥ %.0f%% (0–%d bets). If none qualify, place zero.

`, pctx.Balance, pctx.Deployable, pctx.OpenExposure, pctx.OpenPositions,
		pctx.MinWinRate, maxTrades, placeLimit, pctx.MinWinRate, placeLimit))

	if len(pctx.BlockedMarketIDs) > 0 {
		blocked := pctx.BlockedMarketIDs
		if len(blocked) > 12 {
			blocked = blocked[:12]
		}
		sb.WriteString("Blocked: " + strings.Join(blocked, ", ") + "\n")
	}

	sb.WriteString("\nCandidates:\n")
	for i, d := range dossiers {
		sb.WriteString(fmt.Sprintf("\n===== Candidate %d =====\n", i+1))
		sb.WriteString(polymarket.FormatDossierForPrompt(d))
		sb.WriteString(formatDossierSizingHint(d, pctx.LiveTrading, pctx.Deployable))
	}
	sb.WriteString(fmt.Sprintf(`
Compare all candidates. Return up to %d should_trade=true decisions for those that honestly clear ≥%.0f%% self_win_rate, ranked high→low, with amount in each outcome's [size_floor, item_max] (1–3×, prefer ~1×) and full Chinese reasons (include size rationale). Skip thin ≥86¢ names where win profit cannot beat fees. Engine will then place only the top %d.`, maxTrades, pctx.MinWinRate, placeLimit))
	return sb.String()
}

// formatDossierSizingHint lists per-outcome 1×–3× USDC bands (mirrors trader sizing).
func formatDossierSizingHint(d polymarket.MarketDossier, liveTrading bool, deployable float64) string {
	s := d.Snapshot
	var sb strings.Builder
	sb.WriteString("Sizing (USDC notional for BUY; pick amount in band; engine floor=1× default=1× max=3×; also ≤12% cash):\n")
	for i, o := range s.Outcomes {
		p := 0.0
		if i < len(s.Prices) {
			p = s.Prices[i]
		}
		if p <= 0 {
			continue
		}
		minAmt := promptItemMin(p, s.OrderMinSize, liveTrading)
		floorAmt := minAmt * 1
		defaultAmt := minAmt * 1
		maxAmt := minAmt * 3
		if deployable > 0 {
			fracCap := deployable * 0.12
			if fracCap >= minAmt && maxAmt > fracCap {
				maxAmt = fracCap
			}
			if maxAmt > deployable {
				maxAmt = deployable
			}
		}
		if maxAmt < floorAmt {
			maxAmt = floorAmt
		}
		thin := ""
		if p > 0.86 {
			thin = " [thin≥86¢ — prefer skip]"
		}
		sb.WriteString(fmt.Sprintf("  %s @%.1f¢ → size_floor=$%.2f default=$%.2f item_max=$%.2f (1–3×)%s\n",
			truncatePromptRunes(o, 40), p*100, floorAmt, defaultAmt, maxAmt, thin))
	}
	return sb.String()
}

func promptItemMin(price, orderMinSize float64, liveTrading bool) float64 {
	if price <= 0 {
		return 0
	}
	shares := orderMinSize
	if shares <= 0 {
		shares = 5
	}
	notional := price * shares
	if liveTrading {
		if notional < 1.0 {
			notional = 1.0
		}
	} else if notional < 0.5 {
		notional = 0.5
	}
	return notional
}

func truncatePromptRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if max <= 0 || len(r) <= max {
		return string(r)
	}
	return string(r[:max])
}
