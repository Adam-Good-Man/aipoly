package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Portfolio struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Balance        float64   `json:"balance"`
	InitialBalance float64   `json:"initial_balance"`
	TotalProfit    float64   `json:"total_profit"`
	TotalTrades    int       `json:"total_trades"`
	WonTrades      int       `json:"won_trades"`
	LostTrades     int       `json:"lost_trades"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (p *Portfolio) SettledCount() int {
	return p.WonTrades + p.LostTrades
}

func (p *Portfolio) WinRate() float64 {
	settled := p.SettledCount()
	if settled == 0 {
		return 0
	}
	return float64(p.WonTrades) / float64(settled) * 100
}

type TradeStatus string

const (
	TradeStatusPending  TradeStatus = "pending"
	TradeStatusOpen     TradeStatus = "open"
	TradeStatusWon      TradeStatus = "won"
	TradeStatusLost     TradeStatus = "lost"
	TradeStatusCanceled TradeStatus = "canceled"
)

type Trade struct {
	ID           uuid.UUID   `gorm:"type:uuid;primaryKey" json:"id"`
	MarketID     string      `gorm:"index" json:"market_id"`
	ConditionID  string      `json:"condition_id"`
	EventID      string      `gorm:"index" json:"event_id,omitempty"`
	EventTitle   string      `json:"event_title"`
	MarketTitle  string      `json:"market_title"`
	Outcome      string      `json:"outcome"`
	Side         string      `json:"side"`
	Amount       float64     `json:"amount"`
	Price        float64     `json:"price"`
	Shares       float64     `json:"shares"`
	Reasoning     string      `gorm:"type:text" json:"reasoning"`
	MarketReason  string      `gorm:"type:text" json:"market_reason,omitempty"`
	OutcomeReason string      `gorm:"type:text" json:"outcome_reason,omitempty"`
	SelfWinRate   float64     `json:"self_win_rate"`
	Status       TradeStatus `gorm:"index" json:"status"`
	PnL          float64     `gorm:"column:pnl" json:"pnl"`
	IsPaper      bool        `json:"is_paper"`
	TokenID      string      `json:"token_id"`
	ClobOrderID  string      `json:"clob_order_id,omitempty"`
	OrderError   string      `gorm:"type:text" json:"order_error,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	ResolvedAt   *time.Time  `json:"resolved_at,omitempty"`
}

type MarketEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	EventID     string    `gorm:"uniqueIndex" json:"event_id"`
	Title       string    `json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	Category    string    `json:"category"`
	Volume      float64   `json:"volume"`
	Liquidity   float64   `json:"liquidity"`
	IsFast      bool      `json:"is_fast"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	MarketsJSON string    `gorm:"type:text" json:"-"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type AnalysisLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Summary   string    `gorm:"type:text" json:"summary"`
	EventsAnalyzed int  `json:"events_analyzed"`
	TradesPlaced   int  `json:"trades_placed"`
	CreatedAt time.Time `json:"created_at"`
}

func BeforeCreateUUID(db *gorm.DB) {
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			if field.Name == "ID" {
				if id, ok := db.Statement.ReflectValue.FieldByName("ID").Interface().(uuid.UUID); ok && id == uuid.Nil {
					db.Statement.SetColumn("ID", uuid.New())
				}
			}
		}
	}
}
