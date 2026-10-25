package jobs

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

func PurgeExpiredAuditLogs(ctx context.Context, db *bun.DB) (int64, error) {
	retentionDays := 90
	var value string
	if err := db.NewSelect().Table("settings").Column("value").Where("key = ?", "audit_retention_days").Limit(1).Scan(ctx, &value); err == nil {
		if parsed, parseErr := strconv.Atoi(value); parseErr == nil && parsed > 0 {
			retentionDays = parsed
		}
	} else if err != sql.ErrNoRows {
		return 0, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result, err := db.NewDelete().Table("audit_logs").Where("created_at < ?", cutoff).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type AuditCleanupRunner struct {
	cancel context.CancelFunc
}

func StartAuditCleanupJob(parentCtx context.Context, db *bun.DB, interval time.Duration) *AuditCleanupRunner {
	if interval <= 0 {
		interval = time.Hour
	}
	ctx, cancel := context.WithCancel(parentCtx)
	runner := &AuditCleanupRunner{cancel: cancel}
	go func() {
		cleanup := func() {
			if _, err := PurgeExpiredAuditLogs(ctx, db); err != nil && ctx.Err() == nil {
				log.Printf("audit log cleanup failed: %v", err)
			}
		}
		cleanup()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
	return runner
}

func (r *AuditCleanupRunner) Stop() {
	r.cancel()
}
