package organization

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cubycloud/backend/internal/config"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
)

// orgTusUploadTTL bounds how long Organization TUS session state (in-progress
// parts, metadata, digest state, finalized-item record) is retained before the
// sweeper removes it; abandoned uploads must not consume capacity forever.
const orgTusUploadTTL = 24 * time.Hour

// orgTusHashBuffer bounds the memory used while digesting upload bytes.
const orgTusHashBuffer = 128 << 10

const orgTusSweepInterval = time.Hour

type OrgTusUploadInfo struct {
	ID           string    `json:"id"`
	CreatedBy    string    `json:"created_by"`
	Filename     string    `json:"filename"`
	ParentID     *string   `json:"parent_id"`
	UploadLength int64     `json:"upload_length"`
	Checksum     string    `json:"checksum,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// orgTusDoneRecord durably records the finalized item so a lost final PATCH
// response cannot make the client re-upload and create a duplicate file. It
// carries creator and length so replays work after the regular info/part
// files are removed.
type orgTusDoneRecord struct {
	CreatedBy    string    `json:"created_by,omitempty"`
	UploadLength int64     `json:"upload_length,omitempty"`
	ItemID       string    `json:"item_id"`
	Item         []byte    `json:"item,omitempty"`
	FinalizedAt  time.Time `json:"finalized_at"`
}

type orgTusLock struct {
	mu   sync.Mutex
	refs int
}

var orgTusLocks = struct {
	mu   sync.Mutex
	byID map[string]*orgTusLock
}{byID: make(map[string]*orgTusLock)}

// orgTusSessions keeps incremental SHA-256 state per in-flight upload in
// memory; on restart it is rebuilt from the persisted digest file or the part
// file itself, never from buffered bytes.
var orgTusSessions = struct {
	mu      sync.Mutex
	hashers map[string]hash.Hash
}{hashers: make(map[string]hash.Hash)}

var (
	orgTusSweepMu      sync.Mutex
	orgTusLastSweptLoc time.Time
)

func lockOrgTusUpload(id string) *orgTusLock {
	orgTusLocks.mu.Lock()
	lock := orgTusLocks.byID[id]
	if lock == nil {
		lock = &orgTusLock{}
		orgTusLocks.byID[id] = lock
	}
	lock.refs++
	orgTusLocks.mu.Unlock()
	lock.mu.Lock()
	return lock
}

func unlockOrgTusUpload(id string, lock *orgTusLock) {
	if lock == nil {
		return
	}
	lock.mu.Unlock()
	orgTusLocks.mu.Lock()
	lock.refs--
	if lock.refs <= 0 {
		if orgTusLocks.byID[id] == lock {
			delete(orgTusLocks.byID, id)
		}
	}
	orgTusLocks.mu.Unlock()
}

func orgTusSessionHasher(id string) hash.Hash {
	orgTusSessions.mu.Lock()
	defer orgTusSessions.mu.Unlock()
	return orgTusSessions.hashers[id]
}

func setOrgTusSessionHasher(id string, hasher hash.Hash) {
	orgTusSessions.mu.Lock()
	defer orgTusSessions.mu.Unlock()
	if hasher == nil {
		delete(orgTusSessions.hashers, id)
		return
	}
	orgTusSessions.hashers[id] = hasher
}

// tusTempDir returns the base TUS temp directory; Organization state lives in
// a scoped subdirectory so the two modules never reclaim each other's files.
func tusTempDir() string {
	if dir := os.Getenv("TEMP_TUS_PATH"); dir != "" {
		return dir
	}
	if dir := os.Getenv("TUS_TEMP_PATH"); dir != "" {
		return dir
	}
	dataDir := os.Getenv("DATA_DIR_PATH")
	if dataDir == "" {
		dataDir = config.Load().DataDirPath
	}
	return filepath.Join(dataDir, "temp", "tus")
}

func (h *Handler) tusTempDir() string {
	return filepath.Join(tusTempDir(), "organization")
}

func parseTusMetadata(header string) map[string]string {
	meta := make(map[string]string)
	if strings.TrimSpace(header) == "" {
		return meta
	}
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, " ", 2)
		key := strings.TrimSpace(pair[0])
		if len(pair) == 2 {
			valBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pair[1]))
			if err == nil {
				meta[key] = string(valBytes)
			}
		} else {
			meta[key] = ""
		}
	}
	return meta
}

// orgTusValidID rejects anything that could escape the temp directory via a
// URL parameter: only plain ULID-ish tokens are accepted.
func orgTusValidID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func orgTusPaths(id string) (string, string, string, string) {
	tempDir := tusTempDir()
	return filepath.Join(tempDir, "organization", id+".info"),
		filepath.Join(tempDir, "organization", id+".part"),
		filepath.Join(tempDir, "organization", id+".done"),
		filepath.Join(tempDir, "organization", id+".hash")
}

func validateOrgTusVersion(c echo.Context) bool {
	clientVer := c.Request().Header.Get("Tus-Resumable")
	if clientVer != "" && clientVer != "1.0.0" {
		c.Response().Header().Set("Tus-Version", "1.0.0")
		_ = c.NoContent(http.StatusPreconditionFailed)
		return false
	}
	return true
}

// RegisterTusRoutes wires the Organization TUS endpoints (creation, HEAD
// offset recovery, PATCH append, DELETE termination) onto the authenticated
// /api/v1/drive/organization scope group. Intentionally exported so the
// router can attach them without editing handler.go.
func (h *Handler) RegisterTusRoutes(g *echo.Group) {
	g.POST("/upload-tus", h.createOrgTusUpload)
	g.HEAD("/upload-tus/:id", h.headOrgTusUpload)
	g.PATCH("/upload-tus/:id", h.patchOrgTusUpload)
	g.DELETE("/upload-tus/:id", h.deleteOrgTusUpload)
	g.OPTIONS("/upload-tus", h.optionsOrgTusUpload)
	g.OPTIONS("/upload-tus/:id", h.optionsOrgTusUpload)
}

/* Echo v4 keeps HEAD/PATCH exactly like the My Storage TUS endpoints;
responses always carry Tus-Resumable plus the exposed headers above. */

func (h *Handler) optionsOrgTusUpload(c echo.Context) error {
	c.Response().Header().Set("Tus-Version", "1.0.0")
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Tus-Extension", "creation,termination,checksum")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Tus-Version, Tus-Extension, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) createOrgTusUpload(c echo.Context) error {
	if !validateOrgTusVersion(c) {
		return nil
	}
	lengthHeader := c.Request().Header.Get("Upload-Length")
	if lengthHeader == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "Upload-Length header is required"})
	}
	uploadLength, err := strconv.ParseInt(lengthHeader, 10, 64)
	if err != nil || uploadLength < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid Upload-Length header"})
	}

	// Organization uploads skip the user quota gate by contract; the node
	// capacity gate lives inside the service.
	maxBytes, limitErr, err := h.service.UploadLimit(c.Request().Context())
	if err != nil {
		return respond(c, err)
	}
	if maxBytes == 0 || uploadLength > maxBytes {
		return respond(c, limitErr)
	}

	meta := parseTusMetadata(c.Request().Header.Get("Upload-Metadata"))
	filename := meta["filename"]
	if filename == "" {
		filename = c.QueryParam("filename")
	}
	if filename == "" {
		filename = "unnamed"
	}
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))

	var parentID *string
	if pid, ok := meta["parent_id"]; ok && pid != "" {
		parentID = &pid
	} else if qpid := c.QueryParam("parent_id"); qpid != "" {
		parentID = &qpid
	}

	checksum := ""
	if given, ok := meta["checksum"]; ok && given != "" {
		if !validOrgTusChecksum(given) {
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "unsupported Upload-Metadata checksum"})
		}
		checksum = given
	}

	uploadID := ulid.Make().String()
	tempDir := h.tusTempDir()

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to initialize tus temp directory"})
	}

	info := OrgTusUploadInfo{
		ID:           uploadID,
		CreatedBy:    currentUserID(c),
		Filename:     filename,
		ParentID:     parentID,
		UploadLength: uploadLength,
		Checksum:     checksum,
		CreatedAt:    time.Now().UTC(),
	}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to encode metadata"})
	}

	infoPath, partPath, _, _ := orgTusPaths(uploadID)

	// Opportunistic 24h-lifetime sweep so abandoned sessions are bounded even
	// if the dedicated cron job is not wired yet.
	sweepOrgTusLazy(tempDir)

	lock := lockOrgTusUpload(uploadID)
	defer unlockOrgTusUpload(uploadID, lock)

	if err := os.WriteFile(infoPath, infoBytes, 0644); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to save upload metadata"})
	}
	partFile, err := os.Create(partPath)
	if err != nil {
		_ = os.Remove(infoPath)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to create part file"})
	}
	_ = partFile.Close()

	c.Response().Header().Set("Location", "/api/v1/drive/organization/upload-tus/"+uploadID)
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusCreated)
}

func (h *Handler) headOrgTusUpload(c echo.Context) error {
	if !validateOrgTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if !orgTusValidID(id) {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	infoPath, partPath, donePath, _ := orgTusPaths(id)

	lock := lockOrgTusUpload(id)
	defer unlockOrgTusUpload(id, lock)

	// A finalized upload reports its recovered result instead of 404. The
	// done record is checked first because info/part files are removed on
	// success.
	if done, err := readOrgTusDoneRecord(donePath); err == nil {
		if done.CreatedBy == currentUserID(c) {
			setOrgTusDoneHeaders(c, done.UploadLength, done)
			c.Response().Header().Set("Tus-Resumable", "1.0.0")
			c.Response().Header().Set("Cache-Control", "no-store")
			return c.NoContent(http.StatusOK)
		}
	}

	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	var info OrgTusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "corrupted upload metadata"})
	}
	if info.CreatedBy != currentUserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	stat, err := os.Stat(partPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload file not found"})
	}

	offset := stat.Size()
	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	c.Response().Header().Set("Upload-Length", strconv.FormatInt(info.UploadLength, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusOK)
}

func setOrgTusDoneHeaders(c echo.Context, length int64, done orgTusDoneRecord) {
	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(length, 10))
	c.Response().Header().Set("Upload-Length", strconv.FormatInt(length, 10))
	if done.ItemID != "" {
		c.Response().Header().Set("Upload-Item-Id", done.ItemID)
	}
	if len(done.Item) > 0 {
		c.Response().Header().Set("Upload-Item", base64.StdEncoding.EncodeToString(done.Item))
	}
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
}

func (h *Handler) patchOrgTusUpload(c echo.Context) error {
	if !validateOrgTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if !orgTusValidID(id) {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	offsetHeader := c.Request().Header.Get("Upload-Offset")
	if offsetHeader == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "Upload-Offset header is required"})
	}
	reqOffset, err := strconv.ParseInt(offsetHeader, 10, 64)
	if err != nil || reqOffset < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "invalid Upload-Offset header"})
	}

	infoPath, partPath, donePath, hashPath := orgTusPaths(id)

	lock := lockOrgTusUpload(id)
	defer unlockOrgTusUpload(id, lock)
	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		// After successful finalization the info/part files are removed and
		// only the durable done record remains; replay its result.
		if done, err := readOrgTusDoneRecord(donePath); err == nil {
			if done.CreatedBy != currentUserID(c) {
				return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
			}
			if reqOffset != done.UploadLength {
				c.Response().Header().Set("Tus-Resumable", "1.0.0")
				c.Response().Header().Set("Upload-Offset", strconv.FormatInt(done.UploadLength, 10))
				return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "upload offset mismatch"})
			}
			setOrgTusDoneHeaders(c, done.UploadLength, done)
			c.Response().Header().Set("Tus-Resumable", "1.0.0")
			return c.NoContent(http.StatusNoContent)
		}
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	var info OrgTusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "corrupted upload metadata"})
	}
	if info.CreatedBy != currentUserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	// Idempotent finalization: a retry of the final chunk after Upload
	// succeeded but before the info file was removed replays the record.
	if done, err := readOrgTusDoneRecord(donePath); err == nil {
		if reqOffset != info.UploadLength {
			c.Response().Header().Set("Tus-Resumable", "1.0.0")
			c.Response().Header().Set("Upload-Offset", strconv.FormatInt(info.UploadLength, 10))
			return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "upload offset mismatch"})
		}
		setOrgTusDoneHeaders(c, info.UploadLength, done)
		c.Response().Header().Set("Tus-Resumable", "1.0.0")
		return c.NoContent(http.StatusNoContent)
	}

	stat, err := os.Stat(partPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload file not found"})
	}
	currentOffset := stat.Size()
	if reqOffset != currentOffset {
		c.Response().Header().Set("Tus-Resumable", "1.0.0")
		c.Response().Header().Set("Upload-Offset", strconv.FormatInt(currentOffset, 10))
		return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "upload offset mismatch"})
	}

	hasher, err := orgTusHasherFor(info.ID, partPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to restore upload digest state"})
	}

	file, err := os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to open upload file"})
	}

	remaining := info.UploadLength - currentOffset
	tee := io.MultiWriter(file, hasher)
	written, copyErr := io.Copy(tee, io.LimitReader(c.Request().Body, remaining))
	_ = file.Sync()
	_ = file.Close()
	if copyErr != nil || written < remaining {
		// The digest state can no longer be trusted; rebuild it from the part
		// file on the next successful patch.
		invalidateOrgTusHasher(info.ID, hashPath)
		if copyErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to write chunk"})
		}
	}
	newOffset := currentOffset + written

	if newOffset > info.UploadLength {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "uploaded bytes exceed Upload-Length"})
	}

	if newOffset == info.UploadLength {
		if persistErr := persistOrgTusHasher(hashPath, hasher); persistErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to persist upload digest state"})
		}
		return h.finalizeOrgTusUpload(c, info, partPath, infoPath, donePath, hasher)
	}

	if persistErr := persistOrgTusHasher(hashPath, hasher); persistErr != nil {
		invalidateOrgTusHasher(info.ID, hashPath)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to persist upload digest state"})
	}

	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

// finalizeOrgTusUpload verifies the full-file SHA-256 digest, streams the part
// file through the existing service upload path (folder authorization,
// placement, dedup), then durably records the result for idempotent retries.
func (h *Handler) finalizeOrgTusUpload(c echo.Context, info OrgTusUploadInfo, partPath, infoPath, donePath string, hasher hash.Hash) error {
	digest := hasher.Sum(nil)

	expected := info.Checksum
	if header := c.Request().Header.Get("Upload-Checksum"); header != "" {
		expected = header
	}
	if expected != "" {
		want, err := orgTusChecksumDigest(expected)
		if err != nil {
			dropOrgTusSession(info.ID, partPath, infoPath)
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "unsupported Upload-Checksum"})
		}
		if !orgConstTimeEqual(digest, want) {
			// Wrong content cannot be finalized; drop the session so the
			// client re-selects and starts a fresh upload.
			dropOrgTusSession(info.ID, partPath, infoPath)
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "checksum_mismatch"})
		}
	}

	f, openErr := os.Open(partPath)
	if openErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to open completed upload file"})
	}

	ctx := c.Request().Context()
	maxBytes, limitErr, limitCheckErr := h.service.UploadLimit(ctx)
	if limitCheckErr != nil {
		_ = f.Close()
		return respond(c, limitCheckErr)
	}

	item, uploadErr := h.service.Upload(
		ctx,
		info.CreatedBy,
		info.Filename,
		info.ParentID,
		f,
		true,
		maxBytes,
		limitErr,
	)
	_ = f.Close()
	if uploadErr != nil {
		return respond(c, uploadErr)
	}

	itemJson, err := json.Marshal(item)
	if err != nil {
		itemJson = nil
	}
	record := orgTusDoneRecord{
		CreatedBy:    info.CreatedBy,
		UploadLength: info.UploadLength,
		ItemID:       item.ID,
		Item:         itemJson,
		FinalizedAt:  time.Now().UTC(),
	}
	if writeErr := writeOrgTusDoneRecord(donePath, record); writeErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to record finalized upload"})
	}

	dropOrgTusSession(info.ID, partPath, infoPath)

	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(info.UploadLength, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Upload-Item-Id", item.ID)
	if itemJson != nil {
		c.Response().Header().Set("Upload-Item", base64.StdEncoding.EncodeToString(itemJson))
	}
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

func dropOrgTusSession(id, partPath, infoPath string) {
	setOrgTusSessionHasher(id, nil)
	_ = os.Remove(partPath)
	_ = os.Remove(infoPath)
	_ = os.Remove(filepath.Join(tusTempDir(), "organization", id+".hash"))
}

func (h *Handler) deleteOrgTusUpload(c echo.Context) error {
	if !validateOrgTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if !orgTusValidID(id) {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	infoPath, partPath, donePath, hashPath := orgTusPaths(id)

	lock := lockOrgTusUpload(id)
	defer unlockOrgTusUpload(id, lock)

	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	var info OrgTusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err == nil && info.CreatedBy != currentUserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	setOrgTusSessionHasher(id, nil)
	_ = os.Remove(partPath)
	_ = os.Remove(infoPath)
	_ = os.Remove(donePath)
	_ = os.Remove(hashPath)

	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	return c.NoContent(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Durable SHA-256 digest state (verification with bounded memory)
// ---------------------------------------------------------------------------

type orgBinaryHashState interface {
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

func orgTusHasherFor(id, partPath string) (hash.Hash, error) {
	if cached := orgTusSessionHasher(id); cached != nil {
		return cached, nil
	}

	hashPath := filepath.Join(tusTempDir(), "organization", id+".hash")
	hasher, err := loadOrgTusHasher(hashPath)
	if err != nil || hasher == nil {
		hasher, err = rebuildOrgTusHasher(partPath)
		if err != nil {
			return nil, err
		}
		if err := persistOrgTusHasher(hashPath, hasher); err != nil {
			return nil, err
		}
	}
	setOrgTusSessionHasher(id, hasher)
	return hasher, nil
}

func loadOrgTusHasher(hashPath string) (hash.Hash, error) {
	raw, err := os.ReadFile(hashPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	blob, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return nil, nil
	}
	hasher := sha256.New()
	state, ok := hasher.(orgBinaryHashState)
	if !ok {
		return hasher, nil
	}
	if err := state.UnmarshalBinary(blob); err != nil {
		return nil, nil
	}
	return hasher, nil
}

func rebuildOrgTusHasher(partPath string) (hash.Hash, error) {
	hasher := sha256.New()
	f, err := os.Open(partPath)
	if err != nil {
		if os.IsNotExist(err) {
			return hasher, nil
		}
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, orgTusHashBuffer)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return nil, err
	}
	return hasher, nil
}

func persistOrgTusHasher(hashPath string, hasher hash.Hash) error {
	state, ok := hasher.(orgBinaryHashState)
	if !ok {
		return nil
	}
	blob, err := state.MarshalBinary()
	if err != nil {
		return err
	}
	tmp := hashPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(base64.StdEncoding.EncodeToString(blob)), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, hashPath)
}

func invalidateOrgTusHasher(id, hashPath string) {
	setOrgTusSessionHasher(id, nil)
	_ = os.Remove(hashPath)
}

// ---------------------------------------------------------------------------
// Expected upload checksum (tus checksum extension: `sha256 <base64>`)
// ---------------------------------------------------------------------------

func validOrgTusChecksum(value string) bool {
	_, err := orgTusChecksumDigest(value)
	return err == nil
}

func orgTusChecksumDigest(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	parts := strings.SplitN(value, " ", 2)
	var encoded string
	if len(parts) == 2 && strings.ToLower(parts[0]) == "sha256" {
		encoded = parts[1]
	} else {
		encoded = value
	}
	encoded = strings.TrimSpace(encoded)
	if raw, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(raw) == sha256.Size {
		return raw, nil
	}
	if raw, err := hex.DecodeString(encoded); err == nil && len(raw) == sha256.Size {
		return raw, nil
	}
	return nil, errors.New("unsupported checksum encoding")
}

func orgConstTimeEqual(a, b []byte) bool {
	return hmac.Equal(a, b)
}

// ---------------------------------------------------------------------------
// Durable finalized-upload records (idempotent finalization)
// ---------------------------------------------------------------------------

func readOrgTusDoneRecord(donePath string) (orgTusDoneRecord, error) {
	raw, err := os.ReadFile(donePath)
	if err != nil {
		return orgTusDoneRecord{}, err
	}
	var record orgTusDoneRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return orgTusDoneRecord{}, errors.New("tus upload state corrupted")
	}
	return record, nil
}

func writeOrgTusDoneRecord(donePath string, record orgTusDoneRecord) error {
	blob, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp := donePath + ".tmp"
	if err := os.WriteFile(tmp, blob, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, donePath)
}

// ---------------------------------------------------------------------------
// 24-hour lifetime cleanup
// ---------------------------------------------------------------------------

// CleanupExpiredTusUploads removes Organization TUS session state older than
// orgTusUploadTTL. Safe to call from the cron job at any time: entries still
// being written have fresh mtimes, unknown files are never touched, and each
// removal happens under the upload lock. Finalized items live in the
// database and are never affected.
func CleanupExpiredTusUploads() error {
	_, err := sweepOrgTus(filepath.Join(tusTempDir(), "organization"))
	return err
}

func sweepOrgTusLazy(tempDir string) {
	orgTusSweepMu.Lock()
	due := time.Since(orgTusLastSweptLoc) >= orgTusSweepInterval
	if due {
		orgTusLastSweptLoc = time.Now()
	}
	orgTusSweepMu.Unlock()
	if !due {
		return
	}
	_, _ = sweepOrgTus(tempDir)
}

type orgTusExpiredGroup struct {
	files   []string
	newest  time.Time
	hasInfo bool
}

func sweepOrgTus(tempDir string) (int, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	deadline := time.Now().Add(-orgTusUploadTTL)
	groups := make(map[string]*orgTusExpiredGroup)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".info" && ext != ".part" && ext != ".done" && ext != ".hash" {
			continue
		}
		id := name[:len(name)-len(ext)]
		if !orgTusValidID(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		group := groups[id]
		if group == nil {
			group = &orgTusExpiredGroup{}
			groups[id] = group
		}
		group.files = append(group.files, filepath.Join(tempDir, name))
		if info.ModTime().After(group.newest) {
			group.newest = info.ModTime()
		}
		if ext == ".info" {
			group.hasInfo = true
		}
	}

	removed := 0
	for id, group := range groups {
		if !orgSweepableTusGroup(group, deadline) {
			continue
		}
		lock := lockOrgTusUpload(id)
		for _, path := range group.files {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		unlockOrgTusUpload(id, lock)
		setOrgTusSessionHasher(id, nil)
	}
	return removed, nil
}

func orgSweepableTusGroup(group *orgTusExpiredGroup, deadline time.Time) bool {
	if group.newest.After(deadline) {
		return false
	}
	if !group.hasInfo {
		return len(group.files) > 0
	}
	return true
}
