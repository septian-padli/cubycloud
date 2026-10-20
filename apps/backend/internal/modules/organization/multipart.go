package organization

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

// Direct presigned multipart upload for the organization drive, mirroring the
// My Storage flow (see internal/modules/mystorage/multipart.go). Differences
// follow the module contract: Organization skips the user quota pre-check, the
// namespace is organization-wide (any authenticated user may upload, matching
// the existing org write path), and files below 100 MiB keep the unchanged
// POST /upload flow.
const (
	orgMultipartScope = "organization"
)

// orgMultipartMu is held across provider reconcile + metadata commit so two
// complete calls cannot assemble the same session twice.
var orgMultipartMu sync.Mutex

// errInvalidMultipartSize is org-local because the module has no shared
// invalid-size sentinel; multipartRespond maps it to HTTP 400.
var errInvalidMultipartSize = errors.New("invalid multipart size")

type orgMultipartSession struct {
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

func (r *orgMultipartSession) expired() bool {
	return !r.expiresAt.After(time.Now().UTC())
}

func loadOrgMultipartSession(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, sessionID string) (*orgMultipartSession, error) {
	row := &orgMultipartSession{}
	var parentID, finalItemID sql.NullString
	var ownerID, status string
	err := q.QueryRowContext(ctx, `
		SELECT owner_id, storage_node_id, s3_key, s3_upload_id, filename, size_bytes, part_size_bytes, parent_id, unique_name, status, final_item_id, expires_at
		FROM multipart_uploads
		WHERE id=? AND scope=?
	`, sessionID, orgMultipartScope).Scan(
		&ownerID, &row.nodeID, &row.s3Key, &row.providerUpload, &row.filename, &row.sizeBytes, &row.partSize,
		&parentID, &row.uniqueName, &status, &finalItemID, &row.expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	row.id, row.ownerID, row.parentID, row.finalItemID, row.status = sessionID, ownerID, parentID, finalItemID, status
	return row, nil
}

func loadOrgMultipartSessionForOwner(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID, sessionID string) (*orgMultipartSession, error) {
	session, err := loadOrgMultipartSession(ctx, q, sessionID)
	if err != nil {
		return nil, err
	}
	if session.ownerID != userID {
		return nil, errNotFound
	}
	return session, nil
}

func loadOrgMultipartParts(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, sessionID string) ([]mystorage.MultipartPartState, error) {
	rows, err := q.QueryContext(ctx, `SELECT part_number, size_bytes, COALESCE(etag,''), status FROM multipart_parts WHERE upload_id=? ORDER BY part_number`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	parts := make([]mystorage.MultipartPartState, 0, 16)
	for rows.Next() {
		var p mystorage.MultipartPartState
		var etag string
		if err = rows.Scan(&p.PartNumber, &p.SizeBytes, &etag, &p.Status); err != nil {
			return nil, err
		}
		p.ETag = etag
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

func (s *Service) reconcileOrgMultipartParts(ctx context.Context, session *orgMultipartSession, parts []mystorage.MultipartPartState) ([]mystorage.MultipartPartState, error) {
	providerParts, err := mystorage.NewS3MultipartClient(s.db).ListParts(ctx, session.nodeID, session.s3Key, session.providerUpload)
	if err != nil {
		if mystorage.IsNoSuchUpload(err) {
			return parts, nil
		}
		return nil, fmt.Errorf("list multipart parts: %w", err)
	}
	uploaded := make(map[int32]mystorage.ProviderPart, len(providerParts))
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

// MultipartIntent decides where a large organization file goes: unchanged
// POST /upload below the single-PUT threshold, or a durable multipart session.
func (s *Service) MultipartIntent(ctx context.Context, userID, filename string, size int64, parent *string, uniqueName bool) (*mystorage.MultipartIntentResult, error) {
	parent = normalizeParent(parent)
	if !validName(filename) {
		return nil, errInvalidName
	}
	if size < 0 {
		return nil, errInvalidMultipartSize
	}

	placement, err := storage.SelectPlacement(ctx, s.db, s.storage, size)
	if err != nil {
		return nil, errStorageFull
	}

	// < 100 MiB keeps the existing server-side POST /upload path.
	if placement.Type != "S3" || size < mystorage.SinglePutMaxBytes {
		return &mystorage.MultipartIntentResult{Placement: "server", Mode: "server_put"}, nil
	}

	partSize, partCount, err := planOrgMultipartParts(size)
	if err != nil {
		return nil, err
	}

	s3multipart := mystorage.NewS3MultipartClient(s.db)
	sessionID := ulid.Make().String()
	providerUpload, err := s3multipart.CreateMultipartUpload(ctx, placement.ID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("create multipart upload: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(mystorage.MultipartIntentTTL)
	var parentVal any
	if parent != nil && *parent != "" {
		parentVal = *parent
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO multipart_uploads (id, scope, owner_id, storage_node_id, s3_key, s3_upload_id, filename, size_bytes, part_size_bytes, part_count, parent_id, unique_name, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, orgMultipartScope, userID, placement.ID, sessionID, providerUpload, filename, size, partSize, partCount, parentVal, uniqueName, expiresAt)
	if err != nil {
		_ = s3multipart.AbortMultipartUpload(ctx, placement.ID, sessionID, providerUpload)
		return nil, fmt.Errorf("record multipart session: %w", err)
	}
	parts := planOrgMultipartSizes(size, partSize, partCount)
	if err = mystorage.InsertMultipartParts(ctx, s.db, sessionID, parts); err != nil {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE id=?`, sessionID)
		_ = s3multipart.AbortMultipartUpload(ctx, placement.ID, sessionID, providerUpload)
		return nil, fmt.Errorf("record multipart parts: %w", err)
	}

	states := make([]mystorage.MultipartPartState, len(parts))
	for i, p := range parts {
		states[i] = mystorage.MultipartPartState{PartNumber: p.Number, SizeBytes: p.Size, Status: "pending"}
	}
	return &mystorage.MultipartIntentResult{
		Placement:     "s3",
		Mode:          "multipart",
		UploadID:      sessionID,
		SizeBytes:     size,
		PartSizeBytes: partSize,
		PartCount:     partCount,
		Parts:         states,
	}, nil
}

func planOrgMultipartParts(size int64) (partSize int64, partCount int32, err error) {
	if size <= 0 {
		return 0, 0, mystorage.ErrMultipartBadPart
	}
	partSize = mystorage.MultipartDefaultPartSize
	count := (size + partSize - 1) / partSize
	if count <= mystorage.MultipartMaxParts {
		return partSize, int32(count), nil
	}
	partSize = (size + mystorage.MultipartMaxParts - 1) / mystorage.MultipartMaxParts
	const mib = int64(1 << 20)
	partSize = ((partSize + mib - 1) / mib) * mib
	if partSize > mystorage.MultipartMaxPartSize || partSize < (5<<20) {
		return 0, 0, mystorage.ErrMultipartBadPart
	}
	count = (size + partSize - 1) / partSize
	if count > mystorage.MultipartMaxParts {
		return 0, 0, mystorage.ErrMultipartBadPart
	}
	return partSize, int32(count), nil
}

func planOrgMultipartSizes(size, partSize int64, partCount int32) []mystorage.PlannedPart {
	parts := make([]mystorage.PlannedPart, 0, partCount)
	for i := int32(0); i < partCount; i++ {
		p := partSize
		if int64(i+1)*partSize > size {
			p = size - int64(i)*partSize
		}
		parts = append(parts, mystorage.PlannedPart{Number: i + 1, Size: p})
	}
	return parts
}

// MultipartState returns the durable snapshot for an organization session.
func (s *Service) MultipartState(ctx context.Context, userID, sessionID string) (*mystorage.MultipartSessionSnapshot, error) {
	session, err := loadOrgMultipartSessionForOwner(ctx, s.db, userID, sessionID)
	if err != nil {
		return nil, err
	}
	parts, err := loadOrgMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return nil, err
	}
	if session.status == "active" && !session.expired() {
		parts, err = s.reconcileOrgMultipartParts(ctx, session, parts)
		if err != nil {
			return nil, err
		}
	}
	snapshot := &mystorage.MultipartSessionSnapshot{
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
		snapshot.Parts = make([]mystorage.MultipartPartState, 0)
	}
	if session.expired() && session.status == "active" {
		snapshot.Status = "expired"
	}
	return snapshot, nil
}

// MultipartPartURLs presigns UploadPart URLs; part URLs are minted per call.
func (s *Service) MultipartPartURLs(ctx context.Context, userID, sessionID string, partNumbers []int32) ([]mystorage.MultipartPartURL, error) {
	session, err := loadOrgMultipartSessionForOwner(ctx, s.db, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if session.status != "active" {
		return nil, mystorage.ErrMultipartAlreadyDone
	}
	if session.expired() {
		return nil, mystorage.ErrMultipartExpired
	}
	planned, err := loadOrgMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return nil, err
	}
	wanted := make(map[int32]struct{}, len(partNumbers))
	for _, n := range partNumbers {
		if n < 1 || int(n) > len(planned) {
			return nil, fmt.Errorf("%w: part %d", mystorage.ErrMultipartBadPart, n)
		}
		wanted[n] = struct{}{}
	}
	targets := make([]mystorage.MultipartPartState, 0, len(wanted))
	for _, p := range planned {
		_, requested := wanted[p.PartNumber]
		if len(wanted) == 0 && p.Status != "completed" || requested {
			targets = append(targets, p)
		}
	}
	if len(targets) > mystorage.MultipartPresignBatch {
		return nil, fmt.Errorf("%w: %d > %d", mystorage.ErrMultipartBadPart, len(targets), mystorage.MultipartPresignBatch)
	}
	s3multipart := mystorage.NewS3MultipartClient(s.db)
	urls := make([]mystorage.MultipartPartURL, 0, len(targets))
	expiresAt := time.Now().UTC().Add(mystorage.MultipartURLTTL).Format(time.RFC3339)
	for _, p := range targets {
		url, err := s3multipart.PresignUploadPart(ctx, session.nodeID, session.s3Key, session.providerUpload, p.PartNumber, mystorage.MultipartURLTTL)
		if err != nil {
			return nil, fmt.Errorf("presign part %d: %w", p.PartNumber, err)
		}
		urls = append(urls, mystorage.MultipartPartURL{PartNumber: p.PartNumber, URL: url, ExpiresAt: expiresAt})
	}
	return urls, nil
}

// MultipartCheckpointParts persists per-part ETags/completion or failure.
func (s *Service) MultipartCheckpointParts(ctx context.Context, userID, sessionID string, updates []mystorage.MultipartPartUpdate) error {
	session, err := loadOrgMultipartSessionForOwner(ctx, s.db, userID, sessionID)
	if err != nil {
		return err
	}
	if session.status != "active" {
		return mystorage.ErrMultipartAlreadyDone
	}
	if session.expired() {
		return mystorage.ErrMultipartExpired
	}
	planned, err := loadOrgMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return err
	}
	plannedSizes := make(map[int32]int64, len(planned))
	for _, p := range planned {
		plannedSizes[p.PartNumber] = p.SizeBytes
	}
	for _, u := range updates {
		if _, known := plannedSizes[u.PartNumber]; !known {
			return fmt.Errorf("%w: part %d", mystorage.ErrMultipartBadPart, u.PartNumber)
		}
		switch u.Status {
		case "completed":
			if u.ETag == "" {
				return fmt.Errorf("%w: part %d missing etag", mystorage.ErrMultipartBadPart, u.PartNumber)
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE multipart_parts SET etag=?, status='completed', updated_at=CURRENT_TIMESTAMP WHERE upload_id=? AND part_number=?`, u.ETag, sessionID, u.PartNumber); err != nil {
				return err
			}
		case "failed":
			if _, err = s.db.ExecContext(ctx, `UPDATE multipart_parts SET etag='', status='failed', updated_at=CURRENT_TIMESTAMP WHERE upload_id=? AND part_number=?`, sessionID, u.PartNumber); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: part %d status %q", mystorage.ErrMultipartBadPart, u.PartNumber, u.Status)
		}
	}
	return nil
}

// MultipartComplete reconciles provider truth, completes the object and then
// commits the org item in one transaction. A failed metadata commit preserves
// both object and session; the retry recovers the same single item.
func (s *Service) MultipartComplete(ctx context.Context, userID, sessionID string) (Item, error) {
	orgMultipartMu.Lock()
	defer orgMultipartMu.Unlock()

	session, err := loadOrgMultipartSessionForOwner(ctx, s.db, userID, sessionID)
	if err != nil {
		return Item{}, err
	}
	if session.status == "completed" {
		if session.finalItemID.Valid && session.finalItemID.String != "" {
			return s.Get(ctx, userID, session.finalItemID.String)
		}
		return Item{}, mystorage.ErrMultipartAlreadyDone
	}
	if session.status != "active" {
		return Item{}, mystorage.ErrMultipartUnrecoverable
	}
	if session.expired() {
		return Item{}, mystorage.ErrMultipartExpired
	}

	planned, err := loadOrgMultipartParts(ctx, s.db, sessionID)
	if err != nil {
		return Item{}, err
	}
	s3multipart := mystorage.NewS3MultipartClient(s.db)
	providerParts, err := s3multipart.ListParts(ctx, session.nodeID, session.s3Key, session.providerUpload)
	if err != nil {
		if !mystorage.IsNoSuchUpload(err) {
			return Item{}, fmt.Errorf("list multipart parts: %w", err)
		}
		objectSize, headErr := s3multipart.HeadObjectSize(ctx, session.nodeID, session.s3Key)
		if headErr != nil || objectSize != session.sizeBytes {
			return Item{}, mystorage.ErrMultipartUnrecoverable
		}
	} else {
		providerByNumber := make(map[int32]mystorage.ProviderPart, len(providerParts))
		for _, p := range providerParts {
			providerByNumber[p.PartNumber] = p
		}
		var incomplete []int32
		completed := make([]mystorage.ProviderPart, 0, len(planned))
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
			completed = append(completed, mystorage.ProviderPart{PartNumber: provider.PartNumber, ETag: etag, SizeBytes: provider.SizeBytes})
		}
		if len(incomplete) > 0 {
			return Item{}, &mystorage.IncompletePartsError{Parts: incomplete}
		}
		if err = s3multipart.CompleteMultipartUpload(ctx, session.nodeID, session.s3Key, session.providerUpload, completed); err != nil {
			if mystorage.IsNoSuchUpload(err) {
				return Item{}, mystorage.ErrMultipartUnrecoverable
			}
			return Item{}, fmt.Errorf("complete multipart upload: %w", err)
		}
	}

	return s.commitOrgMultipartItem(ctx, userID, session, orgParentPointer(session.parentID))
}

func orgParentPointer(null sql.NullString) *string {
	if null.Valid && null.String != "" {
		p := null.String
		return &p
	}
	return nil
}

func (s *Service) commitOrgMultipartItem(ctx context.Context, userID string, session *orgMultipartSession, parent *string) (Item, error) {
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

	if err = s.checkParent(ctx, tx, parent); err != nil {
		return Item{}, err
	}

	var existingID string
	lookupErr := tx.QueryRowContext(
		ctx,
		`SELECT id FROM organization_items WHERE deleted_at IS NULL AND name=? AND COALESCE(parent_id,'')=COALESCE(?,'')`,
		session.filename,
		parent,
	).Scan(&existingID)
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return Item{}, lookupErr
	}

	candidate := session.filename
	if lookupErr == nil {
		if !session.uniqueName {
			suggestion, nameErr := uniqueName(ctx, tx, parent, session.filename, "")
			if nameErr != nil {
				return Item{}, nameErr
			}
			return Item{}, fmt.Errorf("%w: %s", errConflict, suggestion)
		}
		candidate, err = uniqueName(ctx, tx, parent, session.filename, "")
		if err != nil {
			return Item{}, err
		}
	}

	if err = checkOrgMultipartNodeCapacity(ctx, s, tx, session); err != nil {
		return Item{}, err
	}

	now := time.Now().UTC()
	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO organization_items (id, created_by, name, kind, size_bytes, parent_id, disk_key, storage_node_id, modified_at)
		 VALUES (?, ?, ?, 'file', ?, ?, ?, ?, ?)`,
		session.id,
		userID,
		candidate,
		session.sizeBytes,
		parent,
		session.s3Key,
		session.nodeID,
		now,
	); err != nil {
		return Item{}, err
	}
	_ = database.RecordActivity(ctx, tx, userID, "upload", session.id, candidate, "file", "organization")
	if _, err = tx.ExecContext(ctx, `UPDATE multipart_uploads SET status='completed', final_item_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, session.id, session.id); err != nil {
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
		CreatedBy:  userID,
	}, nil
}

// checkOrgMultipartNodeCapacity mirrors the single-PUT node enforcement for S3
// nodes so multipart finalize cannot overcommit the node.
func checkOrgMultipartNodeCapacity(ctx context.Context, s *Service, tx storage.QueryRunner, session *orgMultipartSession) error {
	available, err := storage.AvailableOnNodeTx(ctx, tx, s.storage, session.nodeID)
	if err != nil || session.sizeBytes > available {
		return errStorageFull
	}
	return nil
}

// MultipartAbort cancels an organization multipart session explicitly.
func (s *Service) MultipartAbort(ctx context.Context, userID, sessionID string) (bool, error) {
	orgMultipartMu.Lock()
	defer orgMultipartMu.Unlock()

	session, err := loadOrgMultipartSessionForOwner(ctx, s.db, userID, sessionID)
	if err != nil {
		return false, err
	}
	if session.status == "completed" {
		return false, mystorage.ErrMultipartAlreadyDone
	}
	s3multipart := mystorage.NewS3MultipartClient(s.db)
	_, listErr := s3multipart.ListParts(ctx, session.nodeID, session.s3Key, session.providerUpload)
	if listErr == nil {
		if err = s3multipart.AbortMultipartUpload(ctx, session.nodeID, session.s3Key, session.providerUpload); err != nil && !mystorage.IsNoSuchUpload(err) {
			return false, fmt.Errorf("abort multipart upload: %w", err)
		}
	} else if mystorage.IsNoSuchUpload(listErr) {
		if referencedOrErr := deleteUnreferencedOrgMultipartObject(ctx, s, session.nodeID, session.s3Key); referencedOrErr != nil {
			return false, referencedOrErr
		}
	} else {
		return false, fmt.Errorf("list multipart parts: %w", listErr)
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE id=? AND scope=?`, sessionID, orgMultipartScope)
	if err != nil {
		return false, err
	}
	deleted, err := result.RowsAffected()
	return deleted > 0, err
}

func deleteUnreferencedOrgMultipartObject(ctx context.Context, s *Service, nodeID, key string) error {
	var referenced bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM my_storage_items WHERE disk_key=? AND storage_node_id=?
			UNION ALL
			SELECT 1 FROM organization_items WHERE disk_key=? AND storage_node_id=?
		)
	`, key, nodeID, key, nodeID).Scan(&referenced)
	if err != nil || referenced {
		return err
	}
	return mystorage.NewS3MultipartClient(s.db).DeleteObject(ctx, nodeID, key)
}

// CleanupExpiredMultipartSessions aborts expired provider sessions and removes
// expired session bookkeeping while leaving finalized objects intact.
func (s *Service) CleanupExpiredMultipartSessions(ctx context.Context) (int64, error) {
	orgMultipartMu.Lock()
	defer orgMultipartMu.Unlock()

	rows, err := s.db.QueryContext(ctx, `SELECT id, storage_node_id, s3_key FROM multipart_uploads WHERE scope=? AND status='active' AND expires_at <= CURRENT_TIMESTAMP`, orgMultipartScope)
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

	s3multipart := mystorage.NewS3MultipartClient(s.db)
	var removed int64
	var firstErr error
	result, completedErr := s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE scope=? AND status='completed' AND expires_at <= CURRENT_TIMESTAMP`, orgMultipartScope)
	if completedErr != nil {
		firstErr = completedErr
	} else if n, countErr := result.RowsAffected(); countErr != nil {
		firstErr = countErr
	} else {
		removed += n
	}

	for _, session := range sessions {
		_, listErr := s3multipart.ListParts(ctx, session.nodeID, session.key, session.id)
		var cleanupErr error
		switch {
		case listErr == nil:
			if err = s3multipart.AbortMultipartUpload(ctx, session.nodeID, session.key, session.id); err != nil && !mystorage.IsNoSuchUpload(err) {
				cleanupErr = fmt.Errorf("abort expired multipart upload %s: %w", session.id, err)
			}
		case mystorage.IsNoSuchUpload(listErr):
			if delErr := deleteUnreferencedOrgMultipartObject(ctx, s, session.nodeID, session.key); delErr != nil {
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
		result, deleteErr := s.db.ExecContext(ctx, `DELETE FROM multipart_uploads WHERE id=? AND scope=? AND status='active' AND expires_at <= CURRENT_TIMESTAMP`, session.id, orgMultipartScope)
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

func respondMultipart(c echo.Context, err error) error {
	incomplete := &mystorage.IncompletePartsError{}
	switch {
	case errors.Is(err, mystorage.ErrMultipartExpired):
		return echo.NewHTTPError(http.StatusGone, map[string]string{"error": "multipart_session_expired"})
	case errors.Is(err, mystorage.ErrMultipartUnrecoverable):
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "multipart_unrecoverable"})
	case errors.Is(err, mystorage.ErrMultipartAlreadyDone):
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "multipart_session_completed"})
	case errors.As(err, &incomplete):
		return echo.NewHTTPError(http.StatusConflict, map[string]any{"error": "multipart_parts_incomplete", "parts": incomplete.Parts})
	case errors.Is(err, mystorage.ErrMultipartBadPart):
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid multipart part"})
	case errors.Is(err, errInvalidMultipartSize):
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid multipart size"})
	}
	return respond(c, err)
}

// RegisterMultipartRoutes is the single wiring point for the organization
// multipart endpoints; the lead calls h.RegisterMultipartRoutes(g).
func (h *Handler) RegisterMultipartRoutes(g *echo.Group) {
	g.POST("/multipart-intent", h.multipartIntent)
	g.GET("/multipart/:id", h.multipartState)
	g.POST("/multipart/:id/parts/urls", h.multipartPartURLs)
	g.PUT("/multipart/:id/parts", h.multipartCheckpointParts)
	g.POST("/multipart/:id/complete", h.multipartComplete)
	g.DELETE("/multipart/:id", h.multipartAbort)
}

func (h *Handler) multipartIntent(c echo.Context) error {
	var req struct {
		Filename   string  `json:"filename"`
		Size       int64   `json:"size"`
		ParentID   *string `json:"parent_id"`
		UniqueName bool    `json:"unique_name"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	res, err := h.service.MultipartIntent(c.Request().Context(), currentUserID(c), req.Filename, req.Size, req.ParentID, req.UniqueName)
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) multipartState(c echo.Context) error {
	snapshot, err := h.service.MultipartState(c.Request().Context(), currentUserID(c), c.Param("id"))
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
	urls, err := h.service.MultipartPartURLs(c.Request().Context(), currentUserID(c), c.Param("id"), req.PartNumbers)
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"parts": urls})
}

func (h *Handler) multipartCheckpointParts(c echo.Context) error {
	var req struct {
		Parts []mystorage.MultipartPartUpdate `json:"parts"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := h.service.MultipartCheckpointParts(c.Request().Context(), currentUserID(c), c.Param("id"), req.Parts); err != nil {
		return respondMultipart(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) multipartComplete(c echo.Context) error {
	item, err := h.service.MultipartComplete(c.Request().Context(), currentUserID(c), c.Param("id"))
	if err != nil {
		return respondMultipart(c, err)
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) multipartAbort(c echo.Context) error {
	deleted, err := h.service.MultipartAbort(c.Request().Context(), currentUserID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, mystorage.ErrMultipartAlreadyDone) {
			return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "multipart_session_completed"})
		}
		return respondMultipart(c, err)
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "not_found"})
	}
	return c.NoContent(http.StatusNoContent)
}
