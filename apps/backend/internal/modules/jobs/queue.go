package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

type Queue struct {
	db       *bun.DB
	wakeChan chan struct{}
}

func NewQueue(db *bun.DB) *Queue {
	return &Queue{
		db:       db,
		wakeChan: make(chan struct{}, 64),
	}
}

func (q *Queue) WakeChan() <-chan struct{} {
	return q.wakeChan
}

func (q *Queue) notify() {
	select {
	case q.wakeChan <- struct{}{}:
	default:
	}
}

// Enqueue adds a new job with status PENDING to the SQLite database.
func (q *Queue) Enqueue(ctx context.Context, jobType, userID string, payload any) (*database.Job, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal job payload: %w", err)
	}

	job := &database.Job{
		ID:        ulid.Make().String(),
		Type:      jobType,
		Status:    StatusPending,
		UserID:    userID,
		Payload:   string(payloadBytes),
		Progress:  0,
		CreatedAt: time.Now().UTC(),
	}

	_, err = q.db.NewInsert().Model(job).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert job: %w", err)
	}

	q.notify()
	return job, nil
}

// Get fetches a job by ID, verifying user ownership unless userID is empty.
func (q *Queue) Get(ctx context.Context, jobID, userID string) (*database.Job, error) {
	job := new(database.Job)
	query := q.db.NewSelect().Model(job).Where("id = ?", jobID)
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	err := query.Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

// Claim atomically claims the oldest PENDING job and marks it PROCESSING using SQLite RETURNING.
func (q *Queue) Claim(ctx context.Context) (*database.Job, error) {
	job := new(database.Job)
	query := `
		UPDATE jobs
		SET status = 'PROCESSING', started_at = CURRENT_TIMESTAMP
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = 'PENDING'
			ORDER BY created_at ASC
			LIMIT 1
		)
		RETURNING id, type, status, user_id, payload, progress, error_message, result, created_at, started_at, completed_at
	`
	row := q.db.QueryRowContext(ctx, query)
	var startedAt, completedAt sql.NullTime
	var errMsg, resStr sql.NullString

	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.UserID,
		&job.Payload,
		&job.Progress,
		&errMsg,
		&resStr,
		&job.CreatedAt,
		&startedAt,
		&completedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if errMsg.Valid {
		job.ErrorMessage = &errMsg.String
	}
	if resStr.Valid {
		job.Result = &resStr.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return job, nil
}

// UpdateProgress updates the job's completion percentage (0-100).
func (q *Queue) UpdateProgress(ctx context.Context, jobID string, progress int) error {
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	_, err := q.db.NewUpdate().
		Model((*database.Job)(nil)).
		Set("progress = ?", progress).
		Where("id = ?", jobID).
		Exec(ctx)
	return err
}

// Complete marks a job as COMPLETED with 100% progress and optional result JSON.
func (q *Queue) Complete(ctx context.Context, jobID string, result any) error {
	now := time.Now().UTC()
	update := q.db.NewUpdate().
		Model((*database.Job)(nil)).
		Set("status = ?", StatusCompleted).
		Set("progress = 100").
		Set("completed_at = ?", now).
		Where("id = ?", jobID)

	if result != nil {
		resBytes, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
		update = update.Set("result = ?", string(resBytes))
	}

	_, err := update.Exec(ctx)
	return err
}

// Fail marks a job as FAILED with an error message and completed_at timestamp.
func (q *Queue) Fail(ctx context.Context, jobID string, errMsg string) error {
	now := time.Now().UTC()
	_, err := q.db.NewUpdate().
		Model((*database.Job)(nil)).
		Set("status = ?", StatusFailed).
		Set("error_message = ?", errMsg).
		Set("completed_at = ?", now).
		Where("id = ?", jobID).
		Exec(ctx)
	return err
}

// RecoverInterrupted transitions any lingering PROCESSING jobs to FAILED on server restart.
func (q *Queue) RecoverInterrupted(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	res, err := q.db.NewUpdate().
		Model((*database.Job)(nil)).
		Set("status = ?", StatusFailed).
		Set("error_message = 'Interrupted by server restart'").
		Set("completed_at = ?", now).
		Where("status = ?", StatusProcessing).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
