package mystorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/uptrace/bun"
)

// fakeS3 implements the provider surface the flow needs and records provider
// truth so ListParts reconciliation is exercised against real responses.
type fakeS3 struct {
	mu           sync.Mutex
	uploadID     string
	created      int
	completed    int
	aborted      int
	parts        map[int32]fakePart
	partOrder    []int32
	objectExists bool
	objectSize   int64
}

type fakePart struct {
	size int64
	etag string
}

func newFakeS3() *fakeS3 {
	return &fakeS3{parts: make(map[int32]fakePart)}
}

func (f *fakeS3) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		switch {
		case r.Method == "POST" && query.Has("uploads"):
			f.mu.Lock()
			f.created++
			f.uploadID = fmt.Sprintf("mp-%d", f.created)
			id := f.uploadID
			f.mu.Unlock()
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, key, id)
		case r.Method == "PUT" && query.Get("uploadId") != "" && query.Get("partNumber") != "":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var partNumber int32
			fmt.Sscanf(query.Get("partNumber"), "%d", &partNumber)
			etag := fmt.Sprintf(`"etag-%d"`, partNumber)
			f.mu.Lock()
			if _, seen := f.parts[partNumber]; !seen {
				f.partOrder = append(f.partOrder, partNumber)
			}
			f.parts[partNumber] = fakePart{size: int64(len(body)), etag: etag}
			f.mu.Unlock()
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusOK)
		case r.Method == "GET" && query.Get("uploadId") != "":
			f.mu.Lock()
			defer f.mu.Unlock()
			order := append([]int32(nil), f.partOrder...)
			sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
			var parts strings.Builder
			for _, n := range order {
				p := f.parts[n]
				parts.WriteString(fmt.Sprintf("<Part><PartNumber>%d</PartNumber><ETag>s%s</ETag><Size>%d</Size></Part>", n, p.etag, p.size))
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<ListPartsResult><Bucket>bucket</Bucket><Key>%s</Key><UploadId>%s</UploadId><IsTruncated>false</IsTruncated>%s</ListPartsResult>`, key, f.uploadID, parts.String())
		case r.Method == "POST" && query.Get("uploadId") != "":
			f.mu.Lock()
			defer f.mu.Unlock()
			f.completed++
			f.objectExists = len(f.parts) > 0
			f.objectSize = 0
			for _, p := range f.parts {
				f.objectSize += p.size
			}
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<CompleteMultipartUploadResult><Location>x</Location><Bucket>bucket</Bucket><Key>object</Key><ETag>"whole"</ETag></CompleteMultipartUploadResult>`)
		case r.Method == "DELETE" && query.Get("uploadId") != "":
			f.mu.Lock()
			f.aborted++
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "HEAD":
			f.mu.Lock()
			defer f.mu.Unlock()
			if !f.objectExists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", f.objectSize))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})
}

func (f *fakeS3) counter() (created, completed, aborted int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.created, f.completed, f.aborted
}

// uploadPlannedParts pushes raw bytes straight to the presigned part URLs the
// server minted, exactly like the browser transport does.
func uploadPlannedParts(t *testing.T, ctx context.Context, svc *Service, owner, sessionID string, intent *MultipartIntentResult, onlyPartsFirst int) error {
	t.Helper()
	urls, err := svc.MultipartPartURLs(ctx, owner, sessionID, nil)
	if err != nil {
		return err
	}
	if onlyPartsFirst < len(intent.Parts) {
		planned := intent.Parts[:onlyPartsFirst]
		wanted := make(map[int32]struct{}, len(planned))
		for _, p := range planned {
			wanted[p.PartNumber] = struct{}{}
		}
		filtered := make([]MultipartPartURL, 0, len(wanted))
		for _, u := range urls {
			if _, ok := wanted[u.PartNumber]; ok {
				filtered = append(filtered, u)
			}
		}
		urls = filtered
	}
	updates := make([]MultipartPartUpdate, 0, len(urls))
	for _, part := range intent.Parts {
		var url string
		for _, u := range urls {
			if u.PartNumber == part.PartNumber {
				url = u.URL
			}
		}
		if url == "" {
			continue
		}
		body := bytes.Repeat([]byte{byte(part.PartNumber)}, int(part.SizeBytes))
		req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		drain, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("part %d: status %d body %q", part.PartNumber, resp.StatusCode, string(drain))
		}
		etag := resp.Header.Get("ETag")
		if etag == "" {
			return fmt.Errorf("part %d: no ETag header", part.PartNumber)
		}
		updates = append(updates, MultipartPartUpdate{PartNumber: part.PartNumber, ETag: etag, SizeBytes: part.SizeBytes, Status: "completed"})
	}
	return svc.MultipartCheckpointParts(ctx, owner, sessionID, updates)
}

// newMultipartHarness wires a fake S3 endpoint, an S3-only storage node and a
// user with the given quota bytes into a fresh database-backed service.
func newMultipartHarness(t *testing.T, quotaBytes int64) (*Service, *bun.DB, *fakeS3, context.Context) {
	t.Helper()
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
 t.Setenv("STORAGE_MASTER_KEY", "multipart-storage-key")
	fake := newFakeS3()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	db, err := database.Connect(dataDir)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	keyMaterial, err := storage.InitMasterKey(dataDir, "", "", db)
	if err != nil {
		t.Fatalf("init master key: %v", err)
	}
	access, err := storage.EncryptCredential([]byte("access"), keyMaterial)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := storage.EncryptCredential([]byte("secret"), keyMaterial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO storage_nodes(id,type,endpoint,bucket,region,access_key_encrypted,secret_key_encrypted,capacity_bytes,enabled)
		VALUES('s3-node','S3',?,'bucket','us-east-1',?,?,1000000000000,1)`, server.URL, access, secret); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users(id,username,email,password_hash,role,quota_bytes) VALUES('u1','u1','u1@x','x','REGULAR',?)`, quotaBytes); err != nil {
		t.Fatal(err)
	}
	return NewService(db, stubDriver{}), db, fake, context.Background()
}

// stubDriver is unused for S3-only placement but keeps the Service contract.
type stubDriver struct{ storage.StorageDriver }

type singlePutTestDriver struct{ stubDriver }

func (singlePutTestDriver) ReadObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, errors.New("not used")
}

func (singlePutTestDriver) WriteObject(context.Context, string, string, io.Reader, int64) error {
	return errors.New("not used")
}

func (singlePutTestDriver) DeleteObject(context.Context, string, string) error {
	return errors.New("not used")
}

func (singlePutTestDriver) HeadObject(context.Context, string, string) (int64, error) {
	return 0, errors.New("not used")
}

func (singlePutTestDriver) PresignUpload(context.Context, string, string, time.Duration) (string, error) {
	return "https://bucket.test/upload", nil
}

func (singlePutTestDriver) PresignDownload(context.Context, string, string, string, time.Duration) (string, error) {
	return "", errors.New("not used")
}

func (singlePutTestDriver) ListObjects(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func TestMultipartDirectUploadFlow(t *testing.T) {
	svc, db, fake, ctx := newMultipartHarness(t, int64(1<<40))
	size := int64(100<<20) + 1 // just past the single-PUT threshold

	intent, err := svc.MultipartIntent(ctx, "u1", "big.bin", size, nil, true)
	if err != nil {
		t.Fatalf("intent: %v", err)
	}
	if intent.Placement != "s3" || intent.Mode != "multipart" || intent.UploadID == "" {
		t.Fatalf("unexpected intent %+v", intent)
	}
	if intent.PartCount != 7 || intent.PartSizeBytes != 16<<20 {
		t.Fatalf("unexpected planning %+v", intent)
	}
	if created, _, _ := fake.counter(); created != 1 {
		t.Fatalf("provider sessions = %d", created)
	}

	if err = uploadPlannedParts(t, ctx, svc, "u1", intent.UploadID, intent, len(intent.Parts)); err != nil {
		t.Fatalf("part uploads: %v", err)
	}

	// Reload paths recover from the durable state, not from expiring URLs.
	snapshot, err := svc.MultipartState(ctx, "u1", intent.UploadID)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if snapshot.Status != "active" || snapshot.PartCount != 7 {
		t.Fatalf("unexpected snapshot %+v", snapshot)
	}
	for _, p := range snapshot.Parts {
		if p.Status != "completed" || p.ETag == "" {
			t.Fatalf("part %d not persisted as completed: %+v", p.PartNumber, p)
		}
	}

	item, err := svc.MultipartComplete(ctx, "u1", intent.UploadID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if item.ID != intent.UploadID || item.SizeBytes != size {
		t.Fatalf("unexpected item %+v", item)
	}
	if _, completed, _ := fake.counter(); completed != 1 {
		t.Fatalf("provider complete calls = %d", completed)
	}
	var items int
	if err = db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE owner_id='u1'`).Scan(&items); err != nil {
		t.Fatal(err)
	}
	if items != 1 {
		t.Fatalf("files recorded = %d", items)
	}

	// A retried complete must return the same item, never a duplicate file.
	retried, err := svc.MultipartComplete(ctx, "u1", intent.UploadID)
	if err != nil {
		t.Fatalf("complete retry: %v", err)
	}
	if retried.ID != item.ID || retried.Name != item.Name {
		t.Fatalf("retry mismatch: %+v vs %+v", retried, item)
	}

	// Aborting a completed session is an explicit conflict, not a restart.
	if _, err = svc.MultipartAbort(ctx, "u1", intent.UploadID); err == nil {
		t.Fatal("abort after complete must be explicit conflict")
	}

	// Expired completed metadata can be removed without touching the finalized object.
	if _, err = svc.db.Exec(`UPDATE multipart_uploads SET expires_at=CURRENT_TIMESTAMP WHERE id=?`, intent.UploadID); err != nil {
		t.Fatal(err)
	}
	removed, cleanupErr := svc.CleanupExpiredMultipartSessions(ctx)
	if cleanupErr != nil || removed != 1 {
		t.Fatalf("completed session cleanup removed %d, err=%v", removed, cleanupErr)
	}
	var sessionCount, itemCount int
	if err = svc.db.QueryRow(`SELECT COUNT(*) FROM multipart_uploads WHERE id=?`, intent.UploadID).Scan(&sessionCount); err != nil {
		t.Fatal(err)
	}
	if err = svc.db.QueryRow(`SELECT COUNT(*) FROM my_storage_items WHERE id=?`, intent.UploadID).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	objectExists := fake.objectExists
	fake.mu.Unlock()
	if sessionCount != 0 || itemCount != 1 || !objectExists {
		t.Fatalf("cleanup removed durable file: session=%d item=%d object=%v", sessionCount, itemCount, objectExists)
	}
}

func TestMultipartThresholdUsesSinglePUTBelow100MiB(t *testing.T) {
	svc, _, _, ctx := newMultipartHarness(t, int64(1<<40))
	svc.storage = singlePutTestDriver{}
	for _, size := range []int64{0, SinglePutMaxBytes - 1} {
		intent, err := svc.MultipartIntent(ctx, "u1", "small.bin", size, nil, true)
		if err != nil {
			t.Fatalf("single-PUT intent for %d bytes: %v", size, err)
		}
		if intent.Mode != "single_put" {
			t.Fatalf("size %d: mode = %q, want single_put", size, intent.Mode)
		}
	}

	intent, err := svc.MultipartIntent(ctx, "u1", "threshold.bin", SinglePutMaxBytes, nil, true)
	if err != nil {
		t.Fatalf("multipart intent at threshold: %v", err)
	}
	if intent.Mode != "multipart" {
		t.Fatalf("size %d: mode = %q, want multipart", SinglePutMaxBytes, intent.Mode)
	}
	if _, err = svc.MultipartAbort(ctx, "u1", intent.UploadID); err != nil {
		t.Fatalf("abort threshold session: %v", err)
	}
}

func TestMultipartIncompleteRejectsWithMissingParts(t *testing.T) {
	svc, db, fake, ctx := newMultipartHarness(t, int64(1<<40))
	size := int64(100<<20) + 1 // just past the single-PUT threshold, seven 16 MiB parts

	intent, err := svc.MultipartIntent(ctx, "u1", "gap.bin", size, nil, true)
	if err != nil {
		t.Fatalf("intent: %v", err)
	}
	if err = uploadPlannedParts(t, ctx, svc, "u1", intent.UploadID, intent, 1); err != nil {
		t.Fatalf("partial uploads: %v", err)
	}
	if _, err := svc.db.Exec(`UPDATE multipart_parts SET etag=NULL, status='pending' WHERE upload_id=? AND part_number=1`, intent.UploadID); err != nil {
		t.Fatal(err)
	}
	// Recreate the service after the provider accepted the part, as on a backend restart.
	svc = NewService(db, stubDriver{})
	snapshot, err := svc.MultipartState(ctx, "u1", intent.UploadID)
	if err != nil {
		t.Fatalf("reconcile state: %v", err)
	}
	if snapshot.Parts[0].Status != "completed" || snapshot.Parts[0].ETag == "" {
		t.Fatalf("provider part was not restored from ListParts: %+v", snapshot.Parts[0])
	}

	_, err = svc.MultipartComplete(ctx, "u1", intent.UploadID)
	if err == nil {
		t.Fatal("complete must fail while parts are missing")
	}
	incomplete := &incompletePartsError{}
	if !errors.As(err, &incomplete) {
		t.Fatalf("want incomplete parts error, got %v", err)
	}
	if len(incomplete.Parts) != 6 || incomplete.Parts[0] != 2 || incomplete.Parts[5] != 7 {
		t.Fatalf("unexpected missing parts %v", incomplete.Parts)
	}

	// Owner boundary: another account cannot act on the session.
	if _, err = svc.MultipartState(ctx, "intruder", intent.UploadID); err == nil {
		t.Fatal("cross-owner state access must not resolve the session")
	}
	if _, err = svc.MultipartPartURLs(ctx, "intruder", intent.UploadID, nil); err == nil {
		t.Fatal("cross-owner presigning must not resolve the session")
	}
	if _, err = svc.MultipartComplete(ctx, "intruder", intent.UploadID); err == nil {
		t.Fatal("cross-owner complete must not resolve the session")
	}

	// Explicit abort terminates the session and drops provider parts.
	deleted, err := svc.MultipartAbort(ctx, "u1", intent.UploadID)
	if err != nil || !deleted {
		t.Fatalf("abort: deleted=%v err=%v", deleted, err)
	}
	if _, _, aborted := fake.counter(); aborted != 1 {
		t.Fatalf("provider aborts = %d", aborted)
	}
	if _, err = svc.MultipartState(ctx, "u1", intent.UploadID); err == nil {
		t.Fatal("aborted session must be gone")
	}
}

func TestMultipartQuotaAndExpiryAreExplicit(t *testing.T) {
	svc, _, fake, ctx := newMultipartHarness(t, 1000)
	size := int64(100<<20) + 1

	// Quota exhaustion takes precedence over unavailable storage.
	if _, err := svc.db.Exec(`UPDATE storage_nodes SET enabled=0`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MultipartIntent(ctx, "u1", "noquota.bin", size, nil, true); !errors.Is(err, errQuota) {
		t.Fatalf("want quota error, got %v", err)
	}

	// Grant enough quota, restore S3 placement, then force-expire the session.
	if _, err := svc.db.Exec(`UPDATE users SET quota_bytes=? WHERE id='u1'`, int64(200<<20)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.db.Exec(`UPDATE storage_nodes SET enabled=1`); err != nil {
		t.Fatal(err)
	}
	intent, err := svc.MultipartIntent(ctx, "u1", "lapse.bin", size, nil, true)
	if err != nil {
		t.Fatalf("intent: %v", err)
	}
	if _, err = svc.db.Exec(`UPDATE multipart_uploads SET expires_at=CURRENT_TIMESTAMP`); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.MultipartPartURLs(ctx, "u1", intent.UploadID, nil); !errors.Is(err, errMultipartExpired) {
		t.Fatalf("presigning an expired session must fail explicitly, got %v", err)
	}
	if _, err = svc.MultipartComplete(ctx, "u1", intent.UploadID); !errors.Is(err, errMultipartExpired) {
		t.Fatalf("completing an expired session must fail explicitly, got %v", err)
	}
	if err = svc.MultipartCheckpointParts(ctx, "u1", intent.UploadID, nil); err == nil {
		t.Fatal("checkpointing an expired session must fail explicitly")
	}
	removed, cleanupErr := svc.CleanupExpiredMultipartSessions(ctx)
	if cleanupErr != nil {
		t.Fatalf("cleanup: %v", cleanupErr)
	}
	if removed != 1 {
		t.Fatalf("cleanup removed %d sessions", removed)
	}
	if _, _, aborted := fake.counter(); aborted != 1 {
		t.Fatalf("provider aborts after expiry = %d", aborted)
	}
}
