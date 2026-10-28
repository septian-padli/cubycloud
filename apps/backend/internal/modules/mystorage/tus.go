package mystorage

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

// tusUploadTTL is the maximum retention for TUS session state (in-progress
// parts, metadata, digest state, and finalized-item records). Expired state
// is removed by CleanupExpiredTusUploads so abandoned uploads cannot consume
// node capacity forever.
const tusUploadTTL = 24 * time.Hour

// tusHashBuffer bounds the memory used while digesting upload bytes.
const tusHashBuffer = 128 << 10

// tusSweepInterval throttles the opportunistic sweep performed on upload
// creation so bursts of creates stay cheap; CleanupExpiredTusUploads can be
// forced at any time from the cron job.
const tusSweepInterval = time.Hour

type TusUploadInfo struct {
	ID           string    `json:"id"`
	Owner        string    `json:"owner"`
	Filename     string    `json:"filename"`
	ParentID     *string   `json:"parent_id"`
	UploadLength int64     `json:"upload_length"`
	Checksum     string    `json:"checksum,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// tusDoneRecord durably records the result of a finalized upload so a lost
// final response cannot make the client re-upload and create a duplicate file.
// It carries Owner and UploadLength so replays work after cleaning up the
// regular info/part files.
type tusDoneRecord struct {
	Owner        string    `json:"owner,omitempty"`
	UploadLength int64     `json:"upload_length,omitempty"`
	ItemID       string    `json:"item_id"`
	Item         []byte    `json:"item,omitempty"`
	FinalizedAt  time.Time `json:"finalized_at"`
}

type tusUploadLock struct {
	mu   sync.Mutex
	refs int
}

var tusLocks = struct {
	mu   sync.Mutex
	byID map[string]*tusUploadLock
}{byID: make(map[string]*tusUploadLock)}

// tusSessions keeps incremental SHA-256 state per in-flight upload in memory
// (~ a few hundred bytes each). On restart the state is rebuilt from the
// persisted digest file or the part file itself, never from buffered bytes.
var tusSessions = struct {
	mu      sync.Mutex
	hashers map[string](hash.Hash)
}{hashers: make(map[string](hash.Hash))}

var (
	tusSweepMu      sync.Mutex
	tusLastSwept    time.Time
	errTusChecksum  = errors.New("tus checksum mismatch")
	errTusCorrupted = errors.New("tus upload state corrupted")
)

func lockTusUpload(id string) *tusUploadLock {
	tusLocks.mu.Lock()
	lock := tusLocks.byID[id]
	if lock == nil {
		lock = &tusUploadLock{}
		tusLocks.byID[id] = lock
	}
	lock.refs++
	tusLocks.mu.Unlock()
	lock.mu.Lock()
	return lock
}

func unlockTusUpload(id string, lock *tusUploadLock) {
	if lock == nil {
		return
	}
	lock.mu.Unlock()
	tusLocks.mu.Lock()
	lock.refs--
	if lock.refs <= 0 {
		if tusLocks.byID[id] == lock {
			delete(tusLocks.byID, id)
		}
	}
	tusLocks.mu.Unlock()
}

func tusSessionHasher(id string) hash.Hash {
	tusSessions.mu.Lock()
	defer tusSessions.mu.Unlock()
	return tusSessions.hashers[id]
}

func setTusSessionHasher(id string, hasher hash.Hash) {
	tusSessions.mu.Lock()
	defer tusSessions.mu.Unlock()
	if hasher == nil {
		delete(tusSessions.hashers, id)
		return
	}
	tusSessions.hashers[id] = hasher
}

func (h *Handler) tusTempDir() string {
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

// tusValidID rejects anything that could escape the temp directory via a URL
// parameter: only plain ULID-ish tokens are accepted.
func tusValidID(id string) bool {
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

func tusUploadPaths(id string) (string, string, string, string) {
	tempDir := tusTempDir()
	return filepath.Join(tempDir, id+".info"),
		filepath.Join(tempDir, id+".part"),
		filepath.Join(tempDir, id+".done"),
		filepath.Join(tempDir, id+".hash")
}

// tusTempDir is a package-level variant used by the sweeper, which does not
// need a Handler instance.
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

func validateTusVersion(c echo.Context) bool {
	clientVer := c.Request().Header.Get("Tus-Resumable")
	if clientVer != "" && clientVer != "1.0.0" {
		c.Response().Header().Set("Tus-Version", "1.0.0")
		_ = c.NoContent(http.StatusPreconditionFailed)
		return false
	}
	return true
}

func (h *Handler) registerTusRoutes(g *echo.Group) {
	g.POST("/upload-tus", h.createTusUpload)
	g.HEAD("/upload-tus/:id", h.headTusUpload)
	g.PATCH("/upload-tus/:id", h.patchTusUpload)
	g.DELETE("/upload-tus/:id", h.deleteTusUpload)
	g.OPTIONS("/upload-tus", h.optionsTusUpload)
	g.OPTIONS("/upload-tus/:id", h.optionsTusUpload)
}

// RegisterTusRoutes wires the TUS endpoints onto an authenticated scope group.
// Intentionally exported so the router (handler.go is owned elsewhere) can
// attach them to /api/v1/drive/my-storage.
func (h *Handler) RegisterTusRoutes(g *echo.Group) {
	h.registerTusRoutes(g)
}

func (h *Handler) optionsTusUpload(c echo.Context) error {
	c.Response().Header().Set("Tus-Version", "1.0.0")
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Tus-Extension", "creation,termination,checksum")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Tus-Version, Tus-Extension, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) createTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
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

	userId := owner(c)
	maxBytes, limitErr, err := h.service.UploadLimit(c.Request().Context(), userId)
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
		if !validTusChecksum(given) {
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "unsupported Upload-Metadata checksum"})
		}
		checksum = given
	}

	uploadID := ulid.Make().String()
	tempDir := h.tusTempDir()

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to initialize tus temp directory"})
	}

	info := TusUploadInfo{
		ID:           uploadID,
		Owner:        userId,
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

	infoPath := filepath.Join(tempDir, uploadID+".info")
	partPath := filepath.Join(tempDir, uploadID+".part")

	// Opportunistic 24h-lifetime sweep so abandoned sessions are bounded even
	// if the dedicated cron job is not wired yet.
	sweepTusUploadsLazy(tempDir)

	lock := lockTusUpload(uploadID)
	defer unlockTusUpload(uploadID, lock)

	if err := os.WriteFile(infoPath, infoBytes, 0644); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to save upload metadata"})
	}
	partFile, err := os.Create(partPath)
	if err != nil {
		_ = os.Remove(infoPath)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to create part file"})
	}
	_ = partFile.Close()

	c.Response().Header().Set("Location", "/api/v1/drive/my-storage/upload-tus/"+uploadID)
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusCreated)
}

func (h *Handler) headTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if !tusValidID(id) {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	infoPath, partPath, donePath, _ := tusPathsWithDir(h.tusTempDir(), id)

	lock := lockTusUpload(id)
	defer unlockTusUpload(id, lock)

	// A finalized upload reports its recovered result instead of 404 so a
	// client that lost the final PATCH response can adopt the item. The done
	// record is checked first because info/part files are removed on success.
	if done, err := readTusDoneRecord(donePath); err == nil {
		if done.Owner == owner(c) {
			setTusDoneHeaders(c, done.UploadLength, done)
			c.Response().Header().Set("Tus-Resumable", "1.0.0")
			c.Response().Header().Set("Cache-Control", "no-store")
			c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
			return c.NoContent(http.StatusOK)
		}
	}

	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	var info TusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "corrupted upload metadata"})
	}
	if info.Owner != owner(c) {
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

func setTusDoneHeaders(c echo.Context, length int64, done tusDoneRecord) {
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

func (h *Handler) patchTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if !tusValidID(id) {
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

	tempDir := h.tusTempDir()
	infoPath, partPath, donePath, hashPath := tusPathsWithDir(tempDir, id)

	lock := lockTusUpload(id)
	defer unlockTusUpload(id, lock)
	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		// After successful finalization the info/part files are removed and
		// only the durable done record remains; replay its result.
		if done, err := readTusDoneRecord(donePath); err == nil {
			if done.Owner != owner(c) {
				return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
			}
			if reqOffset != done.UploadLength {
				c.Response().Header().Set("Tus-Resumable", "1.0.0")
				c.Response().Header().Set("Upload-Offset", strconv.FormatInt(done.UploadLength, 10))
				return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "upload offset mismatch"})
			}
			setTusDoneHeaders(c, done.UploadLength, done)
			c.Response().Header().Set("Tus-Resumable", "1.0.0")
			return c.NoContent(http.StatusNoContent)
		}
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	var info TusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "corrupted upload metadata"})
	}
	if info.Owner != owner(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	// Idempotent finalization: a retry of the final chunk after Upload
	// succeeded but before the info file was removed replays the record.
	if done, err := readTusDoneRecord(donePath); err == nil {
		if reqOffset != info.UploadLength {
			c.Response().Header().Set("Tus-Resumable", "1.0.0")
			c.Response().Header().Set("Upload-Offset", strconv.FormatInt(info.UploadLength, 10))
			return echo.NewHTTPError(http.StatusConflict, map[string]string{"error": "upload offset mismatch"})
		}
		setTusDoneHeaders(c, info.UploadLength, done)
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

	hasher, err := tusHasherFor(info.ID, partPath)
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
		invalidateTusHasher(info.ID, hashPath)
		if copyErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to write chunk"})
		}
	}
	newOffset := currentOffset + written

	if newOffset > info.UploadLength {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "uploaded bytes exceed Upload-Length"})
	}

	if newOffset == info.UploadLength {
		if persistErr := persistTusHasher(hashPath, hasher); persistErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to persist upload digest state"})
		}
		return h.finalizeTusUpload(c, info, partPath, infoPath, donePath, hasher)
	}

	if persistErr := persistTusHasher(hashPath, hasher); persistErr != nil {
		invalidateTusHasher(info.ID, hashPath)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to persist upload digest state"})
	}

	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

// finalizeTusUpload verifies the full-file SHA-256 digest, streams the part
// file through the existing service upload path (quota, parent validation,
// placement, dedup), then durably records the result for idempotent retries.
func (h *Handler) finalizeTusUpload(c echo.Context, info TusUploadInfo, partPath, infoPath, donePath string, hasher hash.Hash) error {
	digest := hasher.Sum(nil)

	expected := info.Checksum
	if header := c.Request().Header.Get("Upload-Checksum"); header != "" {
		expected = header
	}
	if expected != "" {
		want, err := tusChecksumDigest(expected)
		if err != nil {
			dropTusSession(info.ID, partPath, infoPath)
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "unsupported Upload-Checksum"})
		}
		if !constTimeEqual(digest, want) {
			// Wrong content cannot be finalized; drop the session so the
			// client re-selects and starts a fresh upload.
			dropTusSession(info.ID, partPath, infoPath)
			return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "checksum_mismatch"})
		}
	}

	f, openErr := os.Open(partPath)
	if openErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to open completed upload file"})
	}

	ctx := c.Request().Context()
	maxBytes, limitErr, limitCheckErr := h.service.UploadLimit(ctx, info.Owner)
	if limitCheckErr != nil {
		_ = f.Close()
		return respond(c, limitCheckErr)
	}

	item, uploadErr := h.service.Upload(
		ctx,
		info.Owner,
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
	record := tusDoneRecord{
		Owner:        info.Owner,
		UploadLength: info.UploadLength,
		ItemID:       item.ID,
		Item:         itemJson,
		FinalizedAt:  time.Now().UTC(),
	}
	if writeErr := writeTusDoneRecord(donePath, record); writeErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{"error": "failed to record finalized upload"})
	}

	dropTusSession(info.ID, partPath, infoPath)

	c.Response().Header().Set("Upload-Offset", strconv.FormatInt(info.UploadLength, 10))
	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	c.Response().Header().Set("Upload-Item-Id", item.ID)
	if itemJson != nil {
		c.Response().Header().Set("Upload-Item", base64.StdEncoding.EncodeToString(itemJson))
	}
	c.Response().Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length, Upload-Item-Id, Upload-Item")
	return c.NoContent(http.StatusNoContent)
}

func dropTusSession(id, partPath, infoPath string) {
	setTusSessionHasher(id, nil)
	_ = os.Remove(partPath)
	_ = os.Remove(infoPath)
	_ = os.Remove(filepath.Join(tusTempDir(), id+".hash"))
}

func (h *Handler) deleteTusUpload(c echo.Context) error {
	if !validateTusVersion(c) {
		return nil
	}
	id := c.Param("id")
	if !tusValidID(id) {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	infoPath, partPath, donePath, hashPath := tusPathsWithDir(h.tusTempDir(), id)

	lock := lockTusUpload(id)
	defer unlockTusUpload(id, lock)

	infoBytes, err := os.ReadFile(infoPath)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}
	var info TusUploadInfo
	if err := json.Unmarshal(infoBytes, &info); err == nil && info.Owner != owner(c) {
		return echo.NewHTTPError(http.StatusNotFound, map[string]string{"error": "upload not found"})
	}

	setTusSessionHasher(id, nil)
	_ = os.Remove(partPath)
	_ = os.Remove(infoPath)
	_ = os.Remove(donePath)
	_ = os.Remove(hashPath)

	c.Response().Header().Set("Tus-Resumable", "1.0.0")
	return c.NoContent(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Authenticated-scoped path helpers
// ---------------------------------------------------------------------------

func tusPathsWithDir(tempDir, id string) (string, string, string, string) {
	return filepath.Join(tempDir, id+".info"),
		filepath.Join(tempDir, id+".part"),
		filepath.Join(tempDir, id+".done"),
		filepath.Join(tempDir, id+".hash")
}

// ---------------------------------------------------------------------------
// Durable SHA-256 digest state (archive verification with bounded memory)
// ---------------------------------------------------------------------------

type binaryHashState interface {
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

// tusHasherFor returns the in-memory SHA-256 state for an upload id. State is
// restored from the persisted digest file when available, or rebuilt by
// streaming the part file through the hasher (never buffered in RAM).
func tusHasherFor(id, partPath string) (hash.Hash, error) {
	if cached := tusSessionHasher(id); cached != nil {
		return cached, nil
	}

	hashPath := filepath.Join(tusTempDir(), id+".hash")
	hasher, err := loadTusHasher(hashPath)
	if err != nil || hasher == nil {
		hasher, err = rebuildTusHasher(partPath)
		if err != nil {
			return nil, err
		}
		if err := persistTusHasher(hashPath, hasher); err != nil {
			return nil, err
		}
	}
	setTusSessionHasher(id, hasher)
	return hasher, nil
}

func loadTusHasher(hashPath string) (hash.Hash, error) {
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
	state, ok := hasher.(binaryHashState)
	if !ok {
		return hasher, nil
	}
	if err := state.UnmarshalBinary(blob); err != nil {
		return nil, nil // unsupported state: caller rebuilds from the part file
	}
	return hasher, nil
}

func rebuildTusHasher(partPath string) (hash.Hash, error) {
	hasher := sha256.New()
	f, err := os.Open(partPath)
	if err != nil {
		if os.IsNotExist(err) {
			return hasher, nil
		}
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, tusHashBuffer)
	if _, err := io.CopyBuffer(hasher, f, buf); err != nil {
		return nil, err
	}
	return hasher, nil
}

func persistTusHasher(hashPath string, hasher hash.Hash) error {
	state, ok := hasher.(binaryHashState)
	if !ok {
		return nil // nothing durable to store; rebuild path covers it
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

func invalidateTusHasher(id, hashPath string) {
	setTusSessionHasher(id, nil)
	_ = os.Remove(hashPath)
}

// ---------------------------------------------------------------------------
// Expected upload checksum (tus checksum extension: `sha256 <base64>`)
// ---------------------------------------------------------------------------

func validTusChecksum(value string) bool {
	_, err := tusChecksumDigest(value)
	return err == nil
}

// tusChecksumDigest accepts `sha256 <base64>` and bare base64/hex forms and
// returns the expected 32-byte digest.
func tusChecksumDigest(value string) ([]byte, error) {
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

func constTimeEqual(a, b []byte) bool {
	return hmac.Equal(a, b)
}

// ---------------------------------------------------------------------------
// Durable finalized-upload records (idempotent finalization)
// ---------------------------------------------------------------------------

func readTusDoneRecord(donePath string) (tusDoneRecord, error) {
	raw, err := os.ReadFile(donePath)
	if err != nil {
		return tusDoneRecord{}, err
	}
	var record tusDoneRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return tusDoneRecord{}, errTusCorrupted
	}
	return record, nil
}

func writeTusDoneRecord(donePath string, record tusDoneRecord) error {
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

// CleanupExpiredTusUploads removes TUS session state older than
// tusUploadTTL. It is safe to call from the cron job at any time: entries
// still being written have fresh mtimes, unknown files are never touched,
// and each removal happens under the upload lock. Finalized items live in
// the database and are never affected.
func CleanupExpiredTusUploads() error {
	_, err := sweepTusUploads(tusTempDir())
	return err
}

// sweepTusUploadsLazy runs the sweeper at most once per tusSweepInterval so
// creation bursts stay cheap.
func sweepTusUploadsLazy(tempDir string) {
	tusSweepMu.Lock()
	due := time.Since(tusLastSwept) >= tusSweepInterval
	if due {
		tusLastSwept = time.Now()
	}
	tusSweepMu.Unlock()
	if !due {
		return
	}
	_, _ = sweepTusUploads(tempDir)
}

type tusExpiredGroup struct {
	files   []string
	newest  time.Time
	hasInfo bool
	hasPart bool
}

func sweepTusUploads(tempDir string) (int, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	now := time.Now()
	deadline := now.Add(-tusUploadTTL)
	groups := make(map[string]*tusExpiredGroup)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".info" && ext != ".part" && ext != ".done" && ext != ".hash" {
			continue
		}
		id := strings.TrimSuffix(strings.TrimSuffix(name, ext), "")
		id = name[:len(name)-len(ext)]
		if !tusValidID(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		group := groups[id]
		if group == nil {
			group = &tusExpiredGroup{}
			groups[id] = group
		}
		group.files = append(group.files, filepath.Join(tempDir, name))
		if info.ModTime().After(group.newest) {
			group.newest = info.ModTime()
		}
		switch ext {
		case ".info":
			group.hasInfo = true
		case ".part":
			group.hasPart = true
		}
	}

	removed := 0
	for id, group := range groups {
		// Never delete a half-formed session that has not waited out its TTL,
		// and never a group touched within the lifetime window.
		if !sweepableTusGroup(group, deadline) {
			continue
		}
		lock := lockTusUpload(id)
		for _, path := range group.files {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		unlockTusUpload(id, lock)
		setTusSessionHasher(id, nil)
	}
	return removed, nil
}

func sweepableTusGroup(group *tusExpiredGroup, deadline time.Time) bool {
	if group.newest.After(deadline) {
		return false
	}
	if !group.hasInfo && !group.hasPart {
		// Stray .done/.hash leftovers are still bounded by their own TTL.
		return len(group.files) > 0
	}
	return true
}
