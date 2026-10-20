package jobs

import (
	"context"
	"log"
	"time"

	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/uptrace/bun"
)

type TrashPurgeRunner struct {
	service *mystorage.Service
	cancel  context.CancelFunc
}

// StartTrashPurgeJob initializes and starts a periodic background worker
// that purges items from the trash bin older than retention (default 30 days) across My Storage and Organization.
func StartTrashPurgeJob(parentCtx context.Context, db *bun.DB, store storage.StorageDriver, interval, retention time.Duration) *TrashPurgeRunner {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	ctx, cancel := context.WithCancel(parentCtx)
	service := mystorage.NewService(db, store)

	runner := &TrashPurgeRunner{
		service: service,
		cancel:  cancel,
	}

	go func() {
		// Run initial check after a brief startup delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
			runner.purge(ctx, retention)
			runner.cleanupExpiredPendingCloudUploads(ctx)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runner.purge(ctx, retention)
				runner.cleanupExpiredPendingCloudUploads(ctx)
			}
		}
	}()

	return runner
}

func (r *TrashPurgeRunner) purge(ctx context.Context, retention time.Duration) {
	purged, err := r.service.PurgeExpired(ctx, retention)
	if err != nil {
		log.Printf("[TRASH_PURGE_JOB] Error purging expired trash: %v", err)
	} else if purged > 0 {
		log.Printf("[TRASH_PURGE_JOB] Purged %d expired trash items (> %v)", purged, retention)
	}
}

func (r *TrashPurgeRunner) cleanupExpiredPendingCloudUploads(ctx context.Context) {
	deleted, err := r.service.CleanupExpiredPendingCloudUploads(ctx)
	if err != nil {
		log.Printf("[CLOUD_UPLOAD_CLEANUP_JOB] Error cleaning expired upload intents: %v", err)
	}
	if deleted > 0 {
		log.Printf("[CLOUD_UPLOAD_CLEANUP_JOB] Deleted %d expired upload intents", deleted)
	}
}

// RunNow executes the purge immediately synchronously.
func (r *TrashPurgeRunner) RunNow(ctx context.Context, retention time.Duration) (int64, error) {
	return r.service.PurgeExpired(ctx, retention)
}

// Stop stops the background worker.
func (r *TrashPurgeRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}
