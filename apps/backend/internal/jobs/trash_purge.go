package jobs

import (
	"context"
	"log"
	"time"

	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/uptrace/bun"
)

type TrashPurgeRunner struct {
	service       *mystorage.Service
	organization *organization.Service
	cancel        context.CancelFunc
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
		service:       service,
		organization: organization.NewService(db, store),
		cancel:        cancel,
	}

	go func() {
		// Run initial check after a brief startup delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
			runner.purge(ctx, retention)
			runner.cleanupExpiredPendingCloudUploads(ctx)
			runner.cleanupExpiredTransferSessions(ctx)
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
				runner.cleanupExpiredTransferSessions(ctx)
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

func (r *TrashPurgeRunner) cleanupExpiredTransferSessions(ctx context.Context) {
	if err := mystorage.CleanupExpiredTusUploads(); err != nil {
		log.Printf("[TUS_CLEANUP_JOB] Error cleaning My Storage uploads: %v", err)
	}
	if err := organization.CleanupExpiredTusUploads(); err != nil {
		log.Printf("[TUS_CLEANUP_JOB] Error cleaning Organization uploads: %v", err)
	}
	if r.service != nil {
		if removed, err := r.service.CleanupExpiredMultipartSessions(ctx); err != nil {
			log.Printf("[MULTIPART_CLEANUP_JOB] Error cleaning My Storage sessions: %v", err)
		} else if removed > 0 {
			log.Printf("[MULTIPART_CLEANUP_JOB] Removed %d My Storage sessions", removed)
		}
	}
	if r.organization != nil {
		if removed, err := r.organization.CleanupExpiredMultipartSessions(ctx); err != nil {
			log.Printf("[MULTIPART_CLEANUP_JOB] Error cleaning Organization sessions: %v", err)
		} else if removed > 0 {
			log.Printf("[MULTIPART_CLEANUP_JOB] Removed %d Organization sessions", removed)
		}
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
