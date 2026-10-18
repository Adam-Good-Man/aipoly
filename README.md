# aipoly

Polymarket 预测市场的自动交易系统：定时拉盘 → 模型初筛 → 带新闻的深度分析 → 风控钳制仓位 → 模拟或实盘下单，并用仪表盘看资金、胜率和持仓。

默认 **paper trading**。实盘会动用 Polygon 上的 USDC，亏损由使用者自行承担。本项目与 Polymarket 无官方关系。

仓库名：**aipoly**

## 架构

```
浏览器 :5173                         API :8081
Vue 3 + Vite                         Go + Gin
├─ Portal / Subscribe（公开页）      ├─ handlers        HTTP
├─ Dashboard / Trades / Events       ├─ trader.Engine   周期、下单、止损、结算
└─ 钱包栏（实盘）                    ├─ openai          初筛 + 深度决策（OpenAI 兼容，默认 Moonshot/Kimi）
                                     ├─ polymarket      Gamma 事件、CLOB 下单、赎回
                                     ├─ news            RSSHub 新闻
                                     └─ PostgreSQL      持仓、分析日志、组合统计
```

| 目录 | 作用 |
|------|------|
| `api/` | 交易引擎、风控、HTTP。入口 `api/main.go`，编译产物 `api/bin/aipoly` |
| `api/third_party/go-clob-client/` | CLOB 客户端（`go.mod` replace） |
| `client/` | 中英界面。英文无前缀（`/`、`/app`），中文在 `/cn/...` |
| `.env.example` | 配置模板。真实密钥只放本地 `.env` |

进程启动后同时跑三件事：交易循环、结算/赎回、止损监视。HTTP 只读状态并提供手动触发。

## 一轮交易在做什么

间隔由 `TRADING_INTERVAL_MINUTES` 决定（示例为 10 分钟）。上一轮还在跑则跳过。

1. **实盘前置**（paper 跳过）  
   同步 CLOB 现金、结算已结束仓位。地区被 Polymarket geoblock 挡住则本轮不下单。
2. **资金门**  
   可部署现金低于最小可下单额时暂停调用模型，避免空转烧 token。
3. **市场池**  
   拉取短线、热门、快讯事件，只留 `MAX_MARKET_DAYS` 天内结算的盘，并去掉加密现货价位盘。
4. **轻量初筛**  
   每批最多 30 个市场，最多 5 轮。加密涨跌方向每批最多 4 个，避免占满批次。选出最多 **3** 个进入深析。
5. **深度分析**  
   为这 3 个盘拉新闻（RSSHub）、盘口和近期成交，模型给出方向、`self_win_rate`、理由和金额。无新闻时要求更保守，禁止编造新闻。
6. **下单**  
   只保留自评胜率 ≥ `MIN_WIN_RATE` 的盘，按胜率取前 **0–2** 笔（`MAX_TRADES_PER_CYCLE` 更小时以它为准）。金额由模型提议，引擎钳制后再下。模拟单只写库；实盘 CLOB 成交失败不入库。

## 仓位与风控

新开仓名义金额落在 **交易所最低额的 1–3 倍**。模型没给金额时用 1 倍。单笔还不能超过可部署现金的 12%。加仓另受 `MAX_ADD_FRACTION` 限制。

一笔要能过这些门才会真正下出去：

| 规则 | 作用 |
|------|------|
| 最低胜率 | 默认自评 ≥ 68% |
| 边际 | 自评要高于盘口隐含概率至少 `MIN_EDGE_PERCENT` 个百分点 |
| 价格带 | 新开仓约 52¢–86¢。更高的热门上行太薄，手续费吃掉大部分利润 |
| 手续费覆盖 | 按约 2.5% taker 估算；若赢了的毛利盖不住约 2 倍手续费，拒单 |
| 持仓 | 同一市场已有反向仓则拒绝；同向加仓要浮盈，且次数、金额有上限 |
| 冷却 | 近期亏损过的市场在 `LOSS_COOLDOWN_HOURS` 内不再开 |
| 日亏 | 当日已实现亏损过大则暂停新开仓 |
| 止损 | 标记价相对买入价回撤达到 `STOP_LOSS_PERCENT` 时尝试卖出 |

「历史持仓数」按 `condition + outcome` 计，和 Polymarket 网页上的成交笔数不是同一个东西。胜率是已结算仓位里赢的比例，不是按金额加权的收益。

## 环境

- Go 1.26+
- Node.js 18+
- PostgreSQL 14+

## 跑起来

```bash
cp .env.example .env
createdb -U postgres aipoly   # 或 psql: CREATE DATABASE aipoly;

make build-api
make run-api       # http://127.0.0.1:8081

cd client && npm install && npm run dev   # http://localhost:5173
```

`make dev` 会同时起 API 和前端。启动时自动建表。默认每 10 分钟一轮分析（以 `.env` 为准），约每 3 分钟检查结算。

## 配置

完整列表见 `.env.example`。常用项：

| 变量 | 含义 | 示例默认 |
|------|------|----------|
| `OPENAI_API_KEY` / `MOONSHOT_API_KEY` | 模型密钥 | 必填 |
| `OPENAI_BASE_URL` | OpenAI 兼容地址 | Moonshot |
| `OPENAI_MODEL` | 模型名 | `kimi-k3` |
| `DB_*` | PostgreSQL | `localhost` / `aipoly` |
| `POLYMARKET_PAPER_TRADING` | `false` 才实盘 | `true` |
| `INITIAL_BALANCE` | 模拟初始资金 | `1000` |
| `TRADING_INTERVAL_MINUTES` | 分析间隔 | `10` |
| `MAX_MARKET_DAYS` | 只做 N 天内结算 | `2` |
| `MIN_WIN_RATE` | 最低自评胜率 | `68` |
| `MIN_EDGE_PERCENT` | 相对盘口至少高出的点数 | `2` |
| `MAX_TRADES_PER_CYCLE` | 每轮下单上限 | `1`（引擎硬顶仍是 2） |
| `STOP_LOSS_PERCENT` | 止损回撤 | `30` |
| `RSSHUB_BASE_URL` | 新闻源 | `https://rsshub.app` |
| `API_PORT` | API 端口 | `8081` |

改 `.env` 后要重启 API。

## HTTP

| 方法 | 路径 | 作用 |
|------|------|------|
| GET | `/api/dashboard` | 资金、胜率、最近交易、上一轮状态 |
| GET | `/api/trades` | 交易列表 |
| GET | `/api/events` | 缓存的市场事件 |
| GET | `/api/analysis` | 模型分析日志 |
| POST | `/api/trading/run` | 立刻跑一轮 |
| POST | `/api/trades/:id/resolve` | 手动结算一笔 |
| GET | `/api/wallet` | 实盘钱包状态 |

## 实盘

```bash
POLYMARKET_PAPER_TRADING=false
POLYMARKET_PRIVATE_KEY=0x...
POLYMARKET_FUNDER_ADDRESS=0x...
# 0=EOA  1=PolyProxy  2=Gnosis Safe  3=存入式钱包
POLYMARKET_SIGNATURE_TYPE=0
```

CLOB API 凭证可留空，首次会从私钥派生。赎回若走 Builder Relayer，需要 Builder 三项密钥（也可在首次运行后写回 `.env`）。访问 Polymarket 或模型接口受阻时，设置 `HTTPS_PROXY`。

实盘前请确认地区未被 geoblock，且 funder 里有足够 USDC。
