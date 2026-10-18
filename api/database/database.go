package database

import (
	"fmt"
	"log"

	"aipoly/api/config"
	"aipoly/api/models"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Portfolio{},
		&models.Trade{},
		&models.MarketEvent{},
		&models.AnalysisLog{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	if err := fixTradePnLColumn(db); err != nil {
		return nil, fmt.Errorf("fix pnl column: %w", err)
	}

	if err := purgeCanceledTrades(db); err != nil {
		return nil, fmt.Errorf("purge canceled trades: %w", err)
	}
	if err := reopenPrematureLosses(db); err != nil {
		return nil, fmt.Errorf("reopen premature losses: %w", err)
	}

	if err := ensurePortfolio(db, cfg.InitialBalance); err != nil {
		return nil, err
	}

	_ = UpdatePortfolioStats(db, !cfg.PaperTrading)

	return db, nil
}

// purgeCanceledTrades removes failed-fill rows — they are not shown or kept.
func purgeCanceledTrades(db *gorm.DB) error {
	res := db.Where("status = ?", models.TradeStatusCanceled).Delete(&models.Trade{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("purged %d canceled trade(s) (failed fills are not kept)", res.RowsAffected)
	}
	return nil
}

// reopenPrematureLosses clears losses that were marked without a real settle or stop-loss sell.
func reopenPrematureLosses(db *gorm.DB) error {
	// NOTE: SQL LIKE treats "_" as a wildcard — use position() for literal checks.
	// Do NOT reopen rows already finalized via stop_loss or chain_settle closed-positions.
	res := db.Exec(`
		UPDATE trades
		SET status = ?, pnl = 0, resolved_at = NULL
		WHERE status = ?
		  AND order_error ILIKE ?
		  AND position(? in order_error) = 0
		  AND position(? in order_error) = 0
	`, models.TradeStatusOpen, models.TradeStatusLost, "%reopened%", "stop_loss", "chain_settle")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Printf("reopened %d premature loss trade(s) (await settle or stop-loss)", res.RowsAffected)
	}
	return nil
}

func fixTradePnLColumn(db *gorm.DB) error {
	var pnL, pnl int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'trades' AND column_name = 'pn_l'
	`).Scan(&pnL).Error; err != nil {
		return err
	}
	if err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'trades' AND column_name = 'pnl'
	`).Scan(&pnl).Error; err != nil {
		return err
	}

	switch {
	case pnL > 0 && pnl > 0:
		if err := db.Exec(`UPDATE trades SET pnl = pn_l WHERE pnl = 0 AND pn_l <> 0`).Error; err != nil {
			return err
		}
		return db.Exec(`ALTER TABLE trades DROP COLUMN pn_l`).Error
	case pnL > 0:
		return db.Exec(`ALTER TABLE trades RENAME COLUMN pn_l TO pnl`).Error
	default:
		return nil
	}
}

func ensurePortfolio(db *gorm.DB, initialBalance float64) error {
	var count int64
	if err := db.Model(&models.Portfolio{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	portfolio := models.Portfolio{
		ID:             uuid.New(),
		Balance:        initialBalance,
		InitialBalance: initialBalance,
	}
	return db.Create(&portfolio).Error
}

func GetPortfolio(db *gorm.DB) (*models.Portfolio, error) {
	var p models.Portfolio
	if err := db.First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func SyncPortfolioBalance(db *gorm.DB, clobBalance float64, liveTrading bool) error {
	if !liveTrading || clobBalance <= 0 {
		return nil
	}

	var portfolio models.Portfolio
	if err := db.First(&portfolio).Error; err != nil {
		return err
	}

	// CLOB balance is free USDC; open positions are held as tokens (not in CLOB cash).
	portfolio.InitialBalance = clobBalance
	portfolio.Balance = clobBalance
	return db.Save(&portfolio).Error
}

func UpdatePortfolioStats(db *gorm.DB, liveTrading bool) error {
	var won, lost, total int64
	db.Model(&models.Trade{}).Where("status = ?", models.TradeStatusWon).Count(&won)
	db.Model(&models.Trade{}).Where("status = ?", models.TradeStatusLost).Count(&lost)
	total = won + lost

	var totalPnL float64
	db.Model(&models.Trade{}).Where("status IN ?", []models.TradeStatus{
		models.TradeStatusWon, models.TradeStatusLost,
	}).Select("COALESCE(SUM(pnl), 0)").Scan(&totalPnL)

	var portfolio models.Portfolio
	if err := db.First(&portfolio).Error; err != nil {
		return err
	}

	portfolio.TotalTrades = int(total)
	portfolio.WonTrades = int(won)
	portfolio.LostTrades = int(lost)
	portfolio.TotalProfit = totalPnL
	if !liveTrading {
		portfolio.Balance = portfolio.InitialBalance + totalPnL
		var openAmount float64
		db.Model(&models.Trade{}).Where("status = ?", models.TradeStatusOpen).
			Select("COALESCE(SUM(amount), 0)").Scan(&openAmount)
		portfolio.Balance -= openAmount
	}

	log.Printf("portfolio updated: balance=%.2f profit=%.2f winrate=%.1f%%",
		portfolio.Balance, portfolio.TotalProfit, portfolio.WinRate())
	return db.Save(&portfolio).Error
}
