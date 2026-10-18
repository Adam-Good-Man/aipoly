package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"aipoly/api/services/polymarket"
)

// Max markets sent to the model each cycle (cost control).
const maxMarketsInPrompt = 144
// Max chars of market description kept in the prompt.
const maxContextChars = 120

type Service struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewService(apiKey, model, baseURL string) *Service {
	if baseURL == "" {
		baseURL = "https://api.moonshot.cn/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Service{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			// Kimi K3 with reasoning can take longer than GPT chat models.
			Timeout: 300 * time.Second,
			Transport: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				DisableKeepAlives: true,
			},
		},
	}
}

type TradeDecision struct {
	ShouldTrade      bool    `json:"should_trade"`
	MarketID         string  `json:"market_id"`
	Outcome          string  `json:"outcome"`
	Amount           float64 `json:"amount"`
	MarketReason     string  `json:"market_reason"`
	OutcomeReason    string  `json:"outcome_reason"`
	AnalysisProcess  string  `json:"analysis_process"`
	SourcesCited     string  `json:"sources_cited"`
	Reasoning        string  `json:"reasoning"`
	SelfWinRate      float64 `json:"self_win_rate"`
	Confidence       string  `json:"confidence"`
	RiskLevel        string  `json:"risk_level"`
}

type AnalysisResult struct {
	Summary   string          `json:"summary"`
	Decisions []TradeDecision `json:"decisions"`
}

// MarketPromptContext carries portfolio constraints for the AI analyst.
type MarketPromptContext struct {
	Balance           float64
	Deployable        float64
	ReserveAmount     float64
	OpenExposure      float64
	OpenPositions     int
	MaxBetPercent     float64
	MaxMarketDays     int
	MinWinRate        float64
	MinEdgePercent    float64
	MaxTradesPerCycle int // how many shortlisted candidates to evaluate in deep pass
	MaxPlacePerCycle  int // how many qualifying bets the engine may place (0–2)
	BlockedMarketIDs  []string
	AddableHints      []string // profitable opens that may receive a small add
	MaxSingleBet      float64
	LiveTrading       bool // true when not paper — affects item_min floor in sizing hints
}

type chatRequest struct {
	Model           string          `json:"model"`
	Messages        []chatMessage   `json:"messages"`
	Temperature     *float64        `json:"temperature,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	ResponseFormat  *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Service) AnalyzeMarkets(
	snapshots []polymarket.MarketSnapshot,
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

	prompt := buildPrompt(snapshots, pctx, maxTrades)
	maxRule := fmt.Sprintf("本轮目标：积极寻找合格机会，最多 %d 笔。凡诚实自评达标的盘都应纳入 decisions；按「最可能获胜」排序后输出最多 %d 条 should_trade=true。仅在确实无一盘达标时才 0 笔。禁止只在 summary 口头说「适合交易」却不下单。", maxTrades, maxTrades)

	systemContent := fmt.Sprintf(`你是 Polymarket 主动型交易员：敢下但仍控风险——有把握就下，无把握才空仓；不要默认观望。

只输出 JSON:
{"summary":"中文简述","decisions":[{"should_trade":true,"market_id":"id","outcome":"方向名","amount":1.5,"market_reason":"中文>=20字","outcome_reason":"中文>=20字","self_win_rate":68,"confidence":"medium","risk_level":"medium"}]}

硬性:
- 必须通读列表中的市场（通常上百个），筛选出所有同时满足下列条件的机会，再按获胜概率从高到低取最多 %d 条写入 decisions
- 合格条件：self_win_rate ≥ %.0f（自评必须诚实，禁止为凑单抬高胜率；也禁止明明达标却因「偏保守」故意不下）
- 排序原则：优先「最可能获胜」的合格盘（更高 self_win_rate）；不要只盯列表第一条
- 有 ≥1 个合格机会时：decisions 里必须给出那些机会（最多 %d 条 should_trade=true），禁止「summary 说有机会、decisions 却为空或全 false」
- 默认真探：体育/电竞强弱分明、信息充分、流动性尚可的盘，只要诚实自评 ≥ 门槛就应交易；热门强队 62～85¢ 区间若你认为仍更可能赢，可以下（不要只因「赔率已反映」就一律空仓）
- 仍禁止：纯猜盘、信息极差、锁盘（≥97¢ 或 ≤3¢）、blocked、加密价位/Up-Down；不要为了开单硬选五五开
- 零合格机会时：summary 说明原因，decisions 为 [] 或全 should_trade=false（本轮 0 笔合法，但应是例外而非常态）
- 只交易列表中、%d 天内结算的市场
- amount 为美元名义：落在 1×～3× item_min（默认约 1×）；≥86¢ 薄利大热优先不下；高确信且 52–82¢ → 最多偏 2～3×
- 加仓规则：仅当列表中标注为 addable（已有同向持仓且浮盈）时可小额加仓，amount 不得超过提示的 max_add；未标注 addable 的持仓市场严禁再下
- blocked 列表中的市场绝对禁止交易
- %s`,
		maxTrades, pctx.MinWinRate, maxTrades, pctx.MaxMarketDays, maxRule)

	reqBody := chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemContent},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
	}
	if temp := modelTemperature(s.model); temp != nil {
		reqBody.Temperature = temp
	}
	if effort := modelReasoningEffort(s.model); effort != "" {
		reqBody.ReasoningEffort = effort
	}

	body, _ := json.Marshal(reqBody)

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		result, err := s.postChatCompletion(body)
		if err == nil {
			result.Decisions = SelectTradeDecisions(result.Decisions, maxTrades)
			return result, nil
		}
		lastErr = err
		if !isRetryableOpenAIErr(err) || attempt == 3 {
			break
		}
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	return nil, lastErr
}

func (s *Service) postChatCompletion(body []byte) (*AnalysisResult, error) {
	content, err := s.postChatCompletionRaw(body)
	if err != nil {
		return nil, err
	}
	content = extractJSONObject(content)
	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse analysis JSON: %w (content: %s)", err, content[:min(200, len(content))])
	}
	return &result, nil
}

func (s *Service) postChatCompletionRaw(body []byte) (string, error) {
	endpoint := s.baseURL + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse llm response: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("llm error: %s", chatResp.Error.Message)
	}
	if chatResp.Usage != nil {
		log.Printf("llm usage: model=%s prompt=%d completion=%d total=%d",
			s.model, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

func isRetryableOpenAIErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Never retry billing/quota — it burns more tokens and never succeeds.
	if strings.Contains(msg, "quota") ||
		strings.Contains(msg, "billing") ||
		strings.Contains(msg, "insufficient_quota") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "429") {
		return false
	}
	return strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporary failure") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "tls:") ||
		strings.Contains(msg, "parse analysis json") ||
		strings.Contains(msg, "parse screen json")
}

// extractJSONObject strips markdown fences / leading prose so model output can be unmarshaled.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		if len(s) >= 4 && (strings.HasPrefix(strings.ToLower(s), "json") || strings.HasPrefix(strings.ToLower(s), "jsonc")) {
			// drop language tag on first line
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
			} else {
				s = strings.TrimPrefix(strings.TrimPrefix(s, "json"), "JSON")
			}
		}
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	if strings.HasPrefix(s, "{") {
		return s
	}
	// Leading prose before JSON object
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
		if j := strings.LastIndex(s, "}"); j >= 0 {
			s = s[:j+1]
		}
	}
	return strings.TrimSpace(s)
}

func buildPrompt(snapshots []polymarket.MarketSnapshot, pctx MarketPromptContext, maxTrades int) string {
	maxBet := pctx.MaxSingleBet
	if maxBet <= 0 {
		maxBet = pctx.Deployable / 25
		if maxBet < 1 {
			maxBet = 1
		}
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`Portfolio:
- Equity $%.2f | Deployable $%.2f | Reserve $%.2f | Open $%.2f (%d)
- Max single bet $%.2f | Resolve within %d days

`, pctx.Balance, pctx.Deployable, pctx.ReserveAmount, pctx.OpenExposure, pctx.OpenPositions,
		maxBet, pctx.MaxMarketDays))

	if len(pctx.BlockedMarketIDs) > 0 {
		blocked := pctx.BlockedMarketIDs
		if len(blocked) > 12 {
			blocked = blocked[:12]
		}
		sb.WriteString("Blocked market_ids: ")
		sb.WriteString(strings.Join(blocked, ", "))
		sb.WriteString("\n")
	}
	if len(pctx.AddableHints) > 0 {
		sb.WriteString("Addable (profitable open — small add only, same outcome, amount ≤ max_add):\n")
		for i, h := range pctx.AddableHints {
			if i >= 8 {
				break
			}
			sb.WriteString("  - ")
			sb.WriteString(h)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("Markets (top by tradability):\n")

	limit := min(maxMarketsInPrompt, len(snapshots))
	for i := 0; i < limit; i++ {
		s := snapshots[i]
		daysTag := ""
		if s.DaysToEnd >= 0 {
			daysTag = fmt.Sprintf(" ~%dd", s.DaysToEnd)
		}
		sb.WriteString(fmt.Sprintf("\n%d) id=%s%s vol=$%.0f liq=$%.0f\n", i+1, s.MarketID, daysTag, s.Volume, s.Liquidity))
		if s.EventTitle != "" && s.EventTitle != s.Question {
			sb.WriteString(fmt.Sprintf("Event: %s\n", truncateRunes(s.EventTitle, 80)))
		}
		sb.WriteString(fmt.Sprintf("Q: %s\n", truncateRunes(s.Question, 120)))
		if ctx := truncateRunes(s.Description, maxContextChars); ctx != "" {
			sb.WriteString(fmt.Sprintf("Ctx: %s\n", ctx))
		}
		if s.PriceChange != 0 {
			sb.WriteString(fmt.Sprintf("24h: %.1f%%\n", s.PriceChange*100))
		}
		for j, outcome := range s.Outcomes {
			price := 0.0
			if j < len(s.Prices) {
				price = s.Prices[j]
			}
			sb.WriteString(fmt.Sprintf("  %s %.0f¢\n", truncateRunes(outcome, 40), price*100))
		}
	}

	sb.WriteString(fmt.Sprintf(`
Scan ALL markets above. Actively look for trades: return up to %d decisions with honest self_win_rate at/above the minimum, ranked by highest chance to win. Prefer filling 1-%d when clear favorites / informed edges exist; return 0 only if none truly qualify. Favorites in the ~62-85¢ range are allowed when your win-rate clears the bar. If you trade, each decision MUST be should_trade=true with market_id from the list — never only in summary.`, maxTrades, maxTrades))
	return sb.String()
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func modelTemperature(model string) *float64 {
	m := strings.ToLower(model)
	if strings.HasPrefix(m, "gpt-5") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") ||
		strings.HasPrefix(m, "kimi-k3") {
		return nil
	}
	// Slightly higher than ultra-conservative 0.35 so the model explores more candidates,
	// while JSON schema + win-rate gate still constrain risk.
	t := 0.45
	return &t
}

// modelReasoningEffort sets Kimi K3 thinking strength. "high" balances quality vs latency/cost
// for periodic trading scans; override later via env if needed.
func modelReasoningEffort(model string) string {
	m := strings.ToLower(model)
	if strings.HasPrefix(m, "kimi-k3") {
		return "high"
	}
	return ""
}
