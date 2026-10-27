package mystorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

// Direct presigned multipart upload (browser → provider UploadPart) for files
// placed on an S3-compatible node. Files below the 100 MiB threshold
// keep the existing single-PUT intent flow untouched; larger files get a
// server-owned durable intent plus per-part short-lived URLs, so only raw file
// bytes ever cross the browser↔provider boundary and provider secrets or
// expiring part URLs never persist in the browser.
const (
	MultipartDefaultPartSize = int64(16 << 20) // 16 MiB adaptive default
	MultipartMaxParts        = 10000           // provider hard limit
	MultipartMaxPartSize     = int64(5 << 30)  // provider hard limit per part
	SinglePutMaxBytes        = int64(100 << 20) // files ≥ 100 MiB use multipart
	MultipartURLTTL          = 15 * time.Minute
	MultipartIntentTTL       = 24 * time.Hour
	MultipartPresignBatch    = int(100)
	MultipartScopeMyStorage  = "my-storage"
)

// Domain errors kept local; respondMultipart maps them to explicit statuses so
// an expired or unrecoverable session is never silently restarted.
var (
	errMultipartExpired       = errors.New("multipart session expired")
	errMultipartUnrecoverable = errors.New("multipart session unrecoverable")
	errMultipartBadPart       = errors.New("invalid multipart part")
	errMultipartAlreadyDone   = errors.New("multipart session already completed")
)

// Errors exported for sibling modules that mirror this flow (organization).
var (
	ErrMultipartExpired       = errMultipartExpired
	ErrMultipartUnrecoverable = errMultipartUnrecoverable
	ErrMultipartBadPart       = errMultipartBadPart
	ErrMultipartAlreadyDone   = errMultipartAlreadyDone
)

// IncompletePartsError carries the part numbers that still need uploading.
type IncompletePartsError struct{ Parts []int32 }

type incompletePartsError = IncompletePartsError

func (e *IncompletePartsError) Error() string {
	return fmt.Sprintf("multipart parts incomplete: %d missing", len(e.Parts))
}

// MultipartPartState is persisted per planned part so a reload can resume only
// absent/failed parts. ETags are persisted server-side (and mirrored into the
// local session snapshot); expiring URLs are minted per call only.
type MultipartPartState struct {
	PartNumber int32  `json:"part_number"`
	SizeBytes  int64  `json:"size_bytes"`
	ETag       string `json:"etag,omitempty"`
	Status     string `json:"status"` // pending | completed | failed
}

// MultipartPartURL is one short-lived presigned UploadPart URL.
type MultipartPartURL struct {
	PartNumber int32  `json:"part_number"`
	URL        string `json:"url"`
	ExpiresAt  string `json:"expires_at"`
}

// MultipartSessionSnapshot is the durable intent state handed to the browser.
type MultipartSessionSnapshot struct {
	UploadID      string               `json:"upload_id"`
	Filename      string               `json:"filename"`
	SizeBytes     int64                `json:"size_bytes"`
	PartSizeBytes int64                `json:"part_size_bytes"`
	PartCount     int32                `json:"part_count"`
	Status        string               `json:"status"` // active | completed | expired | aborted
	ExpiresAt     string               `json:"expires_at,omitempty"`
	FinalItemID   string               `json:"final_item_id,omitempty"`
	Parts         []MultipartPartState `json:"parts"`
}

// MultipartIntentResult decides where the file goes: "local" (unchanged local
// flow), "single_put" (existing UploadIntent fields, ≤ 100 MiB), or "multipart".
type MultipartIntentResult struct {
	Placement string `json:"placement"` // "local" | "s3"
	Mode      string `json:"mode"`      // "local" | "single_put" | "multipart"
	UploadID  string `json:"upload_id,omitempty"`
	// single-PUT passthrough (unchanged ≤ 100 MiB flow)
	URL       string `json:"url,omitempty"`
	Method    string `json:"method,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	// multipart session
	SizeBytes     int64                `json:"size_bytes,omitempty"`
	PartSizeBytes int64                `json:"part_size_bytes,omitempty"`
	PartCount     int32                `json:"part_count,omitempty"`
	Parts         []MultipartPartState `json:"parts,omitempty"`
}

// MultipartPartUpdate checkpoints one part from the browser.
type MultipartPartUpdate struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag,omitempty"`
	SizeBytes  int64  `json:"size_bytes"`
	Status     string `json:"status"` // completed | failed
}

type multipartSessionRow struct {
	id             string
	ownerID        string
	nodeID         string
	s3Key          string
	providerUpload string
	filename       string
	sizeBytes      int64
	partSize       int64
	parentID       sql.NullString
	uniqueName     bool
	status         string
	finalItemID    sql.NullString
	expiresAt      time.Time
}

func (r *multipartSessionRow) expired() bool {
	return !r.expiresAt.After(time.Now().UTC())
}

// MultipartIntent decides and (for S3) opens the durable multipart session.
func (s *Service) MultipartIntent(ctx context.Context, owner, filename string, size int64, parent *string, uniqueName bool) (*MultipartIntentResult, error) {
	parent = normalizeParent(parent)
	if !validName(filename) {
		return nil, errInvalidName
	}
	if size < 0 {
		return nil, errInvalidSize
	}

	if size < SinglePutMaxBytes {
		intent, err := s.UploadIntent(ctx, owner, filename, size, parent, uniqueName)
		if err != nil {
			return nil, err
		}
		if intent.Placement != "s3" {
			return &MultipartIntentResult{Placement: "local", Mode: "local"}, nil
		}
		return &MultipartIntentResult{
			Placement: intent.Placement,
			Mode:      "single_put",
			UploadID:  intent.UploadID,
			URL:       intent.URL,
			Method:    intent.Method,
			ExpiresAt: intent.ExpiresAt,
		}, nil
	}

	// Check user quota before placement so quota exhaustion keeps its API error.
	quotaRemaining, err := s.quotaRemaining(ctx, s.db.DB, owner)
	if err != nil {
		return nil, err
	}
	if quotaRemaining == 0 || (quotaRemaining > 0 && quotaRemaining < size) {
		return nil, errQuota
	}

	placement, err := storage.SelectPlacement(ctx, s.db, s.storage, size)
	if err != nil {
		return nil, errStorageFull
	}
	if placement.Type != "S3" {
		return &MultipartIntentResult{Placement: "local", Mode: "local"}, nil
	}

	partSize, partCount, err := planMultipartParts(size)
	if err != nil {
		return nil, err
	}

	s3multipart := NewS3MultipartClient(s.db)
	sessionID := ulid.Make().String()
	instanceID, err := storage.InstanceID(ctx, s.db)
	if err != nil {
		return nil, fmt.Errorf("load storage instance ID: %w", err)
	}
	s3Key := storage.BuildBlobKey(instanceID, placement.ID, sessionID)
	providerUpload, err := s3multipart.CreateMultipartUpload(ctx, placement.ID, s3Key)
	if err != nil {
		return nil, fmt.Errorf("create multipart upload: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(MultipartIntentTTL)
	var parentVal any
	if parent != nil && *parent != "" {
		parentVal = *parent
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO multipart_uploads (id, scope, owner_id, storage_node_id, s3_key, s3_upload_id, filename, size_bytes, part_size_bytes, part_count, parent_id, unique_name, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, MultipartScopeMyStorage, owner, placement.ID, s3Key, providerUpload, filename, size, partSize, partCount, parentVal, uniqueName, expiresAt)
	if err != nil {
		_ = s3multipart.AbortMultipartUpload(ctx, placement.ID, s3Key, providerUpload)
		return nil, fmt.Errorf("record multipart session: %w", err)
	}
	parts := planMultipartSizes(size, partSize, partCount)
	if err = InsertMultipartParts(ctx, s.db, sessionID, parts); err != nil {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE id=?`, sessionID)
		_ = s3multipart.AbortMultipartUpload(ctx, placement.ID, s3Key, providerUpload)
		return nil, fmt.Errorf("record multipart parts: %w", err)
	}

	states := make([]MultipartPartState, len(parts))
	for i, p := range parts {
		states[i] = MultipartPartState{PartNumber: p.Number, SizeBytes: p.Size, Status: "pending"}
	}
	return &MultipartIntentResult{
		Placement:     "s3",
		Mode:          "multipart",
		UploadID:      sessionID,
		SizeBytes:     size,
		PartSizeBytes: partSize,
		PartCount:     partCount,
		Parts:         states,
	}, nil
}

type PlannedPart struct {
	Number int32
	Size   int64
}

// planMultipartParts keys an adaptive part size: 16 MiB until the provider
// 10000-part limit forces fewer, larger parts (never below 5 MiB).
func planMultipartParts(size int64) (partSize int64, partCount int32, err error) {
	if size <= 0 {
		return 0, 0, errInvalidSize
	}
	partSize = MultipartDefaultPartSize
	count := (size + partSize - 1) / partSize
	if count <= MultipartMaxParts {
		return partSize, int32(count), nil
	}
	partSize = (size + MultipartMaxParts - 1) / MultipartMaxParts
	const mib = int64(1 << 20)
	partSize = ((partSize + mib - 1) / mib) * mib
	if partSize > MultipartMaxPartSize || partSize < (5<<20) {
		return 0, 0, errInvalidSize
	}
	count = (size + partSize - 1) / partSize
	if count > MultipartMaxParts {
		return 0, 0, errInvalidSize
	}
	return partSize, int32(count), nil
}

func planMultipartSizes(size, partSize int64, partCount int32) []PlannedPart {
	parts := make([]PlannedPart, 0, partCount)
	for i := int32(0); i < partCount; i++ {
		p := partSize
		if int64(i+1)*partSize > size {
			p = size - int64(i)*partSize
		}
		parts = append(parts, PlannedPart{Number: i + 1, Size: p})
	}
	return parts
}

func InsertMultipartParts(ctx context.Context, q interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, sessionID string, parts []PlannedPart) error {
	const chunkSize = 400
	for start := 0; start < len(parts); start += chunkSize {
		end := start + chunkSize
		if end > len(parts) {
			end = len(parts)
		}
		chunk := parts[start:end]
		var values strings.Builder
		args := make([]any, 0, len(chunk)*3)
		for i, p := range chunk {
			if i > 0 {
				values.WriteByte(',')
			}
			values.WriteString("(?,?,?,NULL,'pending',CURRENT_TIMESTAMP)")
			args = append(args, sessionID, p.Number, p.Size)
		}
		query := fmt.Sprintf(`INSERT INTO multipart_parts (upload_id, part_number, size_bytes, etag, status, updated_at) VALUES %s`, values.String())
		if _, err := q.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

func loadMultipartSession(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, scope, owner, sessionID string, enforceOwner bool) (*multipartSessionRow, error) {
	row := &multipartSessionRow{}
	var parentID, finalItemID sql.NullString
	var ownerID, status string
	err := q.QueryRowContext(ctx, `
		SELECT owner_id, storage_node_id, s3_key, s3_upload_id, filename, size_bytes, part_size_bytes, parent_id, unique_name, status, final_item_id, expires_at
		FROM multipart_uploads
		WHERE id=? AND scope=?
	`, sessionID, scope).Scan(
		&ownerID, &row.nodeID, &row.s3Key, &row.providerUpload, &row.filename, &row.sizeBytes, &row.partSize,
		&parentID, &row.uniqueName, &status, &finalItemID, &row.expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if enforceOwner && ownerID != owner {
		return nil, errNotFound
	}
	row.id, row.ownerID, row.parentID, row.finalItemID, row.status = sessionID, ownerID, parentID, finalItemID, status
	return row, nil
}

func loadMultipartParts(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, sessionID string) ([]MultipartPartState, error) {
	rows, err := q.QueryContext(ctx, `SELECT part_number, size_bytes, COALESCE(etag,''), status FROM multipart_parts WHERE upload_id=? ORDER BY part_number`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := make([]MultipartPartState, 0, 16)
	for rows.Next() {
		var p MultipartPartState
		var etag string
		if err = rows.Scan(&p.PartNumber, &p.SizeBytes, &etag, &p.Status); err != nil {
			return nil, err
		}
		p.ETag = etag
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

func (s *Service) reconcileMultipartParts(ctx context.Context, session *multipartSessionRow, parts []MultipartPartState) ([]MultipartPartState, error) {
	providerParts, err := NewS3MultipartClient(s.db).ListParts(ctx, session.nodeID, session.s3Key, session.providerUpload)
	if err != nil {
		if IsNoSuchUpload(err) {
			return parts, nil
		}
		return nil, fmt.Errorf("list multipart parts: %w", err)
	}
	uploaded := make(map[int32]ProviderPart, len(providerParts))
	for _, part := range providerParts {
		uploaded[part.PartNumber] = part
	}
	for i := range parts {
		status, etag := "pending", ""
		if provider, ok := uploaded[parts[i].PartNumber]; ok && provider.SizeBytes == parts[i].SizeBytes && provider.ETag != "" {
			status, etag = "completed", provider.ETag
		}
		if status != parts[i].Status || etag != parts[i].ETag {
			if _, err = s.db.ExecContext(ctx, `UPDATE multipart_parts SET etag=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE upload_id=? AND part_number=?`, etag, status, session.id, parts[i].PartNumber); err != nil {
				return nil, err
			}
		}
		parts[i].Status, parts[i].ETag = status, etag
	}
	return parts, nil
}

// MultipartState exposes the durable session so the transport helper can hand
// back its last snapshot; expired/completed states are explicit, never silent.
func (s *Service) MultipartState(ctx context.Context, owner, sessionID string) (*MultipartSessionSnapshot, error) {
	session, err := loadMultipartSession(ctx, s.db, MultipartScopeMyStorage, owner, sessionID, true)
	if err != nil {
		return nil, err
	}
	parts, err := loadMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return nil, err
	}
	if session.status == "active" && !session.expired() {
		parts, err = s.reconcileMultipartParts(ctx, session, parts)
		if err != nil {
			return nil, err
		}
	}
	snapshot := &MultipartSessionSnapshot{
		UploadID:      session.id,
		Filename:      session.filename,
		SizeBytes:     session.sizeBytes,
		PartSizeBytes: session.partSize,
		PartCount:     int32(len(parts)),
		Status:        session.status,
		ExpiresAt:     session.expiresAt.Format(time.RFC3339),
		Parts:         parts,
	}
	if session.finalItemID.Valid {
		snapshot.FinalItemID = session.finalItemID.String
	}

	if snapshot.Parts == nil {
		snapshot.Parts = make([]MultipartPartState, 0)
	}
	if session.expired() && session.status == "active" {
		snapshot.Status = "expired"
	}
	return snapshot, nil
}

// MultipartPartURLs presigns UploadPart URLs for the requested part numbers
// (empty = every not-yet-completed part). URLs are minted per call and never
// persisted, so nothing in the browser reloads into a still-valid grant.
func (s *Service) MultipartPartURLs(ctx context.Context, owner, sessionID string, partNumbers []int32) ([]MultipartPartURL, error) {
	session, err := loadMultipartSession(ctx, s.db, MultipartScopeMyStorage, owner, sessionID, true)
	if err != nil {
		return nil, err
	}
	if session.status != "active" {
		return nil, errMultipartAlreadyDone
	}
	if session.expired() {
		return nil, errMultipartExpired
	}
	planned, err := loadMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return nil, err
	}
	wanted := make(map[int32]struct{}, len(partNumbers))
	for _, n := range partNumbers {
		if n < 1 || int(n) > len(planned) {
			return nil, fmt.Errorf("%w: part %d", errMultipartBadPart, n)
		}
		wanted[n] = struct{}{}
	}
	targets := make([]MultipartPartState, 0, len(wanted))
	for _, p := range planned {
		_, requested := wanted[p.PartNumber]
		if (len(wanted) == 0 && p.Status != "completed") || requested {
			targets = append(targets, p)
		}
	}
	if len(targets) > MultipartPresignBatch {
		return nil, fmt.Errorf("%w: %d > %d", errMultipartBadPart, len(targets), MultipartPresignBatch)
	}
	s3multipart := NewS3MultipartClient(s.db)
	urls := make([]MultipartPartURL, 0, len(targets))
	expiresAt := time.Now().UTC().Add(MultipartURLTTL).Format(time.RFC3339)
	for _, p := range targets {
		url, err := s3multipart.PresignUploadPart(ctx, session.nodeID, session.s3Key, session.providerUpload, p.PartNumber, MultipartURLTTL)
		if err != nil {
			return nil, fmt.Errorf("presign part %d: %w", p.PartNumber, err)
		}
		urls = append(urls, MultipartPartURL{PartNumber: p.PartNumber, URL: url, ExpiresAt: expiresAt})
	}
	return urls, nil
}

// MultipartCheckpointParts persists per-part ETags/completion (or failure) so a
// reload resumes only the absent/failed parts.
func (s *Service) MultipartCheckpointParts(ctx context.Context, owner, sessionID string, updates []MultipartPartUpdate) error {
	session, err := loadMultipartSession(ctx, s.db, MultipartScopeMyStorage, owner, sessionID, true)
	if err != nil {
		return err
	}
	if session.status != "active" {
		return errMultipartAlreadyDone
	}
	if session.expired() {
		return errMultipartExpired
	}
	planned, err := loadMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return err
	}
	plannedSizes := make(map[int32]int64, len(planned))
	for _, p := range planned {
		plannedSizes[p.PartNumber] = p.SizeBytes
	}
	for _, u := range updates {
		if _, known := plannedSizes[u.PartNumber]; !known {
			return fmt.Errorf("%w: part %d", errMultipartBadPart, u.PartNumber)
		}
		switch u.Status {
		case "completed":
			if u.ETag == "" {
				return fmt.Errorf("%w: part %d missing etag", errMultipartBadPart, u.PartNumber)
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE multipart_parts SET etag=?, status='completed', updated_at=CURRENT_TIMESTAMP WHERE upload_id=? AND part_number=?`, u.ETag, sessionID, u.PartNumber); err != nil {
				return err
			}
		case "failed":
			if _, err = s.db.ExecContext(ctx, `UPDATE multipart_parts SET etag='', status='failed', updated_at=CURRENT_TIMESTAMP WHERE upload_id=? AND part_number=?`, sessionID, u.PartNumber); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: part %d status %q", errMultipartBadPart, u.PartNumber, u.Status)
		}
	}
	return nil
}

// MultipartComplete reconciles provider truth (ListParts), completes the
// object, then commits the drive item, session completion and part cleanup in
// one transaction, so object and metadata cannot disagree permanently: if the
// metadata commit fails, the object and the active session survive and a retry
// recovers into the same single item.
func (s *Service) MultipartComplete(ctx context.Context, owner, sessionID string) (Item, error) {
	cloudUploadMu.Lock()
	defer cloudUploadMu.Unlock()

	session, err := loadMultipartSession(ctx, s.db, MultipartScopeMyStorage, owner, sessionID, true)
	if err != nil {
		return Item{}, err
	}
	if session.status == "completed" {
		if session.finalItemID.Valid && session.finalItemID.String != "" {
			return s.Get(ctx, owner, session.finalItemID.String)
		}
		return Item{}, errMultipartAlreadyDone
	}
	if session.status != "active" {
		return Item{}, errMultipartUnrecoverable
	}
	if session.expired() {
		return Item{}, errMultipartExpired
	}

	planned, err := loadMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return Item{}, err
	}
	s3multipart := NewS3MultipartClient(s.db)
	providerParts, err := s3multipart.ListParts(ctx, session.nodeID, session.s3Key, session.providerUpload)
	if err != nil {
		if !IsNoSuchUpload(err) {
			return Item{}, fmt.Errorf("list multipart parts: %w", err)
		}
		// The provider session is gone: the object either already exists
		// (previous Complete succeeded but the metadata commit failed) or
		// the upload cannot continue. Reconcile against the object itself.
		objectSize, headErr := s3multipart.HeadObjectSize(ctx, session.nodeID, session.s3Key)
		if headErr != nil || objectSize != session.sizeBytes {
			return Item{}, errMultipartUnrecoverable
		}
	} else {
		providerByNumber := make(map[int32]ProviderPart, len(providerParts))
		for _, p := range providerParts {
			providerByNumber[p.PartNumber] = p
		}
		var incomplete []int32
		completed := make([]ProviderPart, 0, len(planned))
		for _, p := range planned {
			provider, uploaded := providerByNumber[p.PartNumber]
			if !uploaded || provider.SizeBytes != p.SizeBytes {
				incomplete = append(incomplete, p.PartNumber)
				continue
			}
			etag := provider.ETag
			if etag == "" {
				etag = p.ETag
			}
			completed = append(completed, ProviderPart{PartNumber: provider.PartNumber, ETag: etag, SizeBytes: provider.SizeBytes})
		}
		if len(incomplete) > 0 {
			return Item{}, &incompletePartsError{Parts: incomplete}
		}
		if err = s3multipart.CompleteMultipartUpload(ctx, session.nodeID, session.s3Key, session.providerUpload, completed); err != nil {
			if IsNoSuchUpload(err) {
				return Item{}, errMultipartUnrecoverable
			}
			return Item{}, fmt.Errorf("complete multipart upload: %w", err)
		}
	}

	return s.commitMultipartItem(ctx, session, parentPointer(session.parentID))
}

func parentPointer(null sql.NullString) *string {
	if null.Valid && null.String != "" {
		p := null.String
		return &p
	}
	return nil
}

// commitMultipartItem inserts the drive item and marks the session completed in
// one transaction; on failure everything rolls back and the caller retries.
func (s *Service) commitMultipartItem(ctx context.Context, session *multipartSessionRow, parent *string) (Item, error) {
	// Keep the SQLite connection after a failed COMMIT so deferred constraint
	// errors can be rolled back before database/sql returns it to the pool.
	tx, err := s.db.DB.Conn(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Close()
	if _, err = tx.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return Item{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = tx.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	if err = s.checkParent(ctx, tx, session.ownerID, parent); err != nil {
		return Item{}, err
	}

	var existingID string
	lookupErr := tx.QueryRowContext(
		ctx,
		`SELECT id FROM my_storage_items WHERE owner_id=? AND deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`,
		session.ownerID,
		session.filename,
		parent,
	).Scan(&existingID)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return Item{}, lookupErr
	}

	candidate := session.filename
	if session.uniqueName || lookupErr == nil {
		candidate, err = uniqueName(ctx, tx, session.ownerID, parent, session.filename, "")
		if err != nil {
			return Item{}, err
		}
	}

	if err = s.checkQuota(ctx, tx, session.ownerID, session.sizeBytes); err != nil {
		return Item{}, err
	}
	if err = s.checkMultipartNodeCapacity(ctx, tx, session); err != nil {
		return Item{}, err
	}

	now := time.Now().UTC()
	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO my_storage_items (id, owner_id, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		 VALUES (?, ?, ?, 'file', ?, ?, ?, ?, ?)`,
		session.id,
		session.ownerID,
		candidate,
		session.sizeBytes,
		parent,
		session.s3Key,
		session.nodeID,
		now,
	); err != nil {
		return Item{}, err
	}
	_ = database.RecordActivity(ctx, tx, session.ownerID, "upload", session.id, candidate, "file", "my-storage")
	if _, err = tx.ExecContext(ctx, `UPDATE multipart_uploads SET status='completed', final_item_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, session.id, session.id); err != nil {
		return Item{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM multipart_parts WHERE upload_id=?`, session.id); err != nil {
		return Item{}, err
	}
	if _, err = tx.ExecContext(ctx, `COMMIT`); err != nil {
		return Item{}, err
	}
	committed = true

	return Item{
		ID:         session.id,
		Name:       candidate,
		Kind:       "file",
		SizeBytes:  session.sizeBytes,
		ParentID:   parent,
		ModifiedAt: now,
	}, nil
}

// checkMultipartNodeCapacity mirrors the SyncCloud enforcement so a multipart
// finalize cannot overcommit a node more than a single PUT could.
func (s *Service) checkMultipartNodeCapacity(ctx context.Context, tx storage.QueryRunner, session *multipartSessionRow) error {
	available, err := storage.AvailableOnNodeTx(ctx, tx, s.storage, session.nodeID)
	if err != nil || session.sizeBytes > available {
		return errStorageFull
	}
	return nil
}

// deleteUnreferencedMultipartBucketObject keeps a blob still referenced by
// either drive; only orphan uploads belong to this intent's cleanup.
func (s *Service) deleteUnreferencedMultipartBucketObject(ctx context.Context, nodeID, key string) (bool, error) {
	var referenced bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM my_storage_items WHERE disk_key=? AND storage_node_id=?
			UNION ALL
			SELECT 1 FROM organization_items WHERE disk_key=? AND storage_node_id=?
		)
	`, key, nodeID, key, nodeID).Scan(&referenced)
	if err != nil || referenced {
		return referenced, err
	}
	if err = NewS3MultipartClient(s.db).DeleteObject(ctx, nodeID, key); err != nil {
		return false, err
	}
	return false, nil
}

// MultipartAbort cancels the session explicitly: the provider session is
// aborted (dropping every uploaded part) and the durable intent is removed.
func (s *Service) MultipartAbort(ctx context.Context, owner, sessionID string) (bool, error) {
	cloudUploadMu.Lock()
	defer cloudUploadMu.Unlock()

	session, err := loadMultipartSession(ctx, s.db, MultipartScopeMyStorage, owner, sessionID, true)
	if err != nil {
		return false, err
	}
	if session.status == "completed" {
		return false, errMultipartAlreadyDone
	}
	s3multipart := NewS3MultipartClient(s.db)
	_, listErr := s3multipart.ListParts(ctx, session.nodeID, session.s3Key, session.providerUpload)
	if listErr == nil {
		// Uploaded (even zero) parts reach the provider under this session;
		// abort removes every one of them without touching live objects.
		if err = s3multipart.AbortMultipartUpload(ctx, session.nodeID, session.s3Key, session.providerUpload); err != nil && !IsNoSuchUpload(err) {
			return false, fmt.Errorf("abort multipart upload: %w", err)
		}
	} else if IsNoSuchUpload(listErr) {
		// No provider session: drop the object only if it is an orphan left
		// behind by a Complete whose metadata commit failed.
		if _, delErr := s.deleteUnreferencedMultipartBucketObject(ctx, session.nodeID, session.s3Key); delErr != nil {
			return false, delErr
		}
	} else {
		return false, fmt.Errorf("list multipart parts: %w", listErr)
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE id=? AND owner_id=?`, sessionID, owner)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	return deleted > 0, err
}

// CleanupExpiredMultipartSessions aborts expired provider sessions, removes
// expired session bookkeeping and orphan objects, and leaves finalized objects
// intact. It is a job hook the caller can wire into the existing cron dispatch.
func (s *Service) CleanupExpiredMultipartSessions(ctx context.Context) (int64, error) {
	cloudUploadMu.Lock()
	defer cloudUploadMu.Unlock()

	rows, err := s.db.QueryContext(ctx, `SELECT id, storage_node_id, s3_key FROM multipart_uploads WHERE status='active' AND expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, err
	}
	type expiredSession struct{ id, nodeID, key string }
	var sessions []expiredSession
	for rows.Next() {
		var session expiredSession
		if err = rows.Scan(&session.id, &session.nodeID, &session.key); err != nil {
			break
		}
		sessions = append(sessions, session)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return 0, err
	}


	var removed int64
	var firstErr error
	completed, completedErr := s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE scope=? AND status='completed' AND expires_at <= CURRENT_TIMESTAMP`, MultipartScopeMyStorage)
	if completedErr != nil {
		firstErr = completedErr
	} else if n, countErr := completed.RowsAffected(); countErr != nil {
		firstErr = countErr
	} else {
		removed += n
	}

	for _, session := range sessions {
		s3multipart := NewS3MultipartClient(s.db)
		_, listErr := s3multipart.ListParts(ctx, session.nodeID, session.key, session.id)
		var cleanupErr error
		switch {
		case listErr == nil:
			if err = s3multipart.AbortMultipartUpload(ctx, session.nodeID, session.key, session.id); err != nil && !IsNoSuchUpload(err) {
				cleanupErr = fmt.Errorf("abort expired multipart upload %s: %w", session.id, err)
			}
		case IsNoSuchUpload(listErr):
			if _, delErr := s.deleteUnreferencedMultipartBucketObject(ctx, session.nodeID, session.key); delErr != nil {
				cleanupErr = fmt.Errorf("delete expired multipart object %s: %w", session.id, delErr)
			}
		default:
			cleanupErr = fmt.Errorf("list expired multipart upload %s: %w", session.id, listErr)
		}
		if cleanupErr != nil {
			if firstErr == nil {
				firstErr = cleanupErr
			}
			continue
		}
		result, deleteErr := s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE id=? AND scope=? AND status='active' AND expires_at <= CURRENT_TIMESTAMP`, session.id, MultipartScopeMyStorage)
		if deleteErr != nil {
			if firstErr == nil {
				firstErr = deleteErr
			}
			continue
		}
		n, countErr := result.RowsAffected()
		if countErr != nil {
			if firstErr == nil {
				firstErr = countErr
			}
			continue
		}
		removed += n
	}
	return removed, firstErr
}

// respondMultipart maps multipart domain errors to explicit statuses, then
// falls back to the module mapper for shared errors.
func respondMultipart(c echo.Context, err error) error {
	incomplete := &incompletePartsError{}
	switch {
	case errors.Is(err, errMultipartExpired):
		return echo.NewHTTPError(http.StatusGone, map[string]string{"error": "multipart_session_expired"})
	case errors.Is(err, errMultipartUnrecoverable):
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "multipart_unrecoverable"})
	case errors.Is(err, errMultipartAlreadyDone):
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "multipart_session_completed"})
	case errors.As(err, &incomplete):
		return echo.NewHTTPError(http.StatusConflict, map[string]any{"error": "multipart_parts_incomplete", "parts": incomplete.Parts})
	case errors.Is(err, errMultipartBadPart):
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid multipart part"})
	}
	return respond(c, err)
}

// RegisterMultipartRoutes is the single wiring point for the multipart
// endpoints. Route registration stays out of handler.go per work split; the
// lead calls h.RegisterMultipartRoutes(g) right after the existing routes.
func (h *Handler) RegisterMultipartRoutes(g *echo.Group) {
	g.POST("/multipart-intent", h.multipartIntent)
	g.GET("/multipart/:id", h.multipartState)
	g.POST("/multipart/:id/parts/urls", h.multipartPartURLs)
	g.PUT("/multipart/:id/parts", h.multipartCheckpointParts)
	g.POST("/multipart/:id/complete", h.multipartComplete)
	g.DELETE("/multipart/:id", h.multipartAbort)
}

type multipartIntentRequest struct {
	Filename   string  `json:"filename"`
	Size       int64   `json:"size"`
	ParentID   *string `json:"parent_id"`
	UniqueName bool    `json:"unique_name"`
}

func (h *Handler) multipartIntent(c echo.Context) error {
	var req multipartIntentRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	res, err := h.service.MultipartIntent(c.Request().Context(), owner(c), req.Filename, req.Size, req.ParentID, req.UniqueName)
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) multipartState(c echo.Context) error {
	snapshot, err := h.service.MultipartState(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) multipartPartURLs(c echo.Context) error {
	var req struct {
		PartNumbers []int32 `json:"part_numbers"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	urls, err := h.service.MultipartPartURLs(c.Request().Context(), owner(c), c.Param("id"), req.PartNumbers)
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"parts": urls})
}

func (h *Handler) multipartCheckpointParts(c echo.Context) error {
	var req struct {
		Parts []MultipartPartUpdate `json:"parts"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := h.service.MultipartCheckpointParts(c.Request().Context(), owner(c), c.Param("id"), req.Parts); err != nil {
		return respondMultipart(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) multipartComplete(c echo.Context) error {
	item, err := h.service.MultipartComplete(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) multipartAbort(c echo.Context) error {
	deleted, err := h.service.MultipartAbort(c.Request().Context(), owner(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, errMultipartAlreadyDone) {
			return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "multipart_session_completed"})
		}
		return respondMultipart(c, err)
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	return c.NoContent(http.StatusNoContent)
}
