package mystorage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/modules/mystorage"
	"github.com/cubycloud/backend/internal/modules/organization"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/uptrace/bun"
)

type capacityTestEnv struct {
	db         *bun.DB
	driver     storage.StorageDriver
	nodeID     string
	nodePath   string
	myService  *mystorage.Service
	orgService *organization.Service
	myUser     string
	orgUser    string
}

// setupCapacityTestEnv creates an isolated environment with a single LOCAL storage node.
func setupCapacityTestEnv(t *testing.T, nodeCapacity int64) *capacityTestEnv {
	t.Helper()
	tempDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tempDir, "db"), 0o755); err != nil {
		t.Fatalf("failed to create db dir: %v", err)
	}
	nodePath := filepath.Join(tempDir, "storage", "default")
	if err := os.MkdirAll(nodePath, 0o755); err != nil {
		t.Fatalf("failed to create storage dir: %v", err)
	}

	db, err := database.Connect(tempDir)
	if err != nil {
		t.Fatalf("database.Connect failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	ctx := context.Background()

	// Users with unlimited personal quota (-1) so capacity rejections come strictly from physical node.
	myUser := "my-storage-user"
	orgUser := "org-storage-user"
	for _, u := range []struct {
		id, name, email string
	}{
		{myUser, "myuser", "myuser@example.test"},
		{orgUser, "orguser", "orguser@example.test"},
	} {
		_, err = db.ExecContext(ctx, `
			INSERT INTO users(id, username, email, password_hash, role, quota_bytes)
			VALUES(?, ?, ?, 'hash', 'REGULAR', -1)
		`, u.id, u.name, u.email)
		if err != nil {
			t.Fatalf("failed to insert user %s: %v", u.id, err)
		}
	}

	// Insert single LOCAL storage node with the specified capacity_bytes.
	nodeID := ulid.Make().String()
	_, err = db.ExecContext(ctx, `
		INSERT INTO storage_nodes(id, type, path, capacity_bytes, enabled, created_at)
		VALUES(?, 'LOCAL', ?, ?, 1, ?)
	`, nodeID, nodePath, nodeCapacity, time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to insert storage node: %v", err)
	}

	driver := storage.NewStorageDriver(db, filepath.Join(tempDir, "storage"))
	myService := mystorage.NewService(db, driver)
	orgService := organization.NewService(db, driver)

	return &capacityTestEnv{
		db:         db,
		driver:     driver,
		nodeID:     nodeID,
		nodePath:   nodePath,
		myService:  myService,
		orgService: orgService,
		myUser:     myUser,
		orgUser:    orgUser,
	}
}

// isStorageFullError checks if an error represents storage full or pool full.
func isStorageFullError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mystorage.ErrStorageFull) || errors.Is(err, storage.ErrPoolFull) {
	}
	msg := err.Error()
	return strings.Contains(msg, "STORAGE_POOL_FULL") || strings.Contains(msg, "storage pool full")
}

// getNodeStoredBytes calculates the combined deduplicated physical size on the node.
func getNodeStoredBytes(t *testing.T, db *bun.DB, nodeID string) int64 {
	t.Helper()
	var used int64
	err := db.QueryRowContext(context.Background(), `
		SELECT COALESCE(SUM(size_bytes),0) FROM (
			SELECT disk_key, MAX(size_bytes) AS size_bytes FROM (
				SELECT disk_key, size_bytes FROM my_storage_items WHERE kind='file' AND disk_key IS NOT NULL AND storage_node_id=?
				UNION ALL
				SELECT disk_key, size_bytes FROM organization_items WHERE kind='file' AND disk_key IS NOT NULL AND storage_node_id=?
			) GROUP BY disk_key
		)
	`, nodeID, nodeID).Scan(&used)
	if err != nil {
		t.Fatalf("failed to query node stored bytes: %v", err)
	}
	return used
}

// getPhysicalDirBytes returns the sum of file sizes present on the physical filesystem directory.
func getPhysicalDirBytes(t *testing.T, nodePath string) int64 {
	t.Helper()
	var total int64
	entries, err := os.ReadDir(nodePath)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			info, err := e.Info()
			if err == nil {
				total += info.Size()
			}
		}
	}
	return total
}

// makePayload creates deterministic byte slices of the requested size.
func makePayload(size int64, fill byte) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = fill
	}
	return b
}

// Test 1: Joint Capacity Rejection
// Memverifikasi bahwa kapasitas node fisik dihitung secara gabungan:
// - Ketika node terisi oleh organization_items, upload my_storage_items yang melebihi kapasitas ditolak dengan error storage full.
// - Sebaliknya, ketika node terisi oleh my_storage_items, upload organization_items yang melebihi kapasitas ditolak.
func TestJointCapacityRejection(t *testing.T) {
	t.Run("OrgPrefill_BlocksMyStorageOverflow", func(t *testing.T) {
		// Node capacity = 100,000 bytes.
		// 5% reserve = 5,000 bytes. Safe capacity = 95,000 bytes.
		const nodeCap int64 = 100_000
		const safeCap int64 = 95_000

		env := setupCapacityTestEnv(t, nodeCap)
		ctx := context.Background()

		// 1. Organization prefill: 70,000 bytes.
		orgPayload := makePayload(70_000, 0xAA)
		orgItem, err := env.orgService.Upload(ctx, env.orgUser, "org-prefill.bin", nil, bytes.NewReader(orgPayload), false, 1<<30, nil)
		if err != nil {
			t.Fatalf("expected org prefill upload to succeed, got: %v", err)
		}
		if orgItem.SizeBytes != 70_000 {
			t.Fatalf("expected org item size 70000, got: %d", orgItem.SizeBytes)
		}

		// Sisa kapasitas aman = 95,000 - 70,000 = 25,000 bytes.

		// 2. My Storage mencoba upload file 30,000 bytes (> 25,000 sisa aman) -> HARUS DITOLAK.
		myOverflowPayload := makePayload(30_000, 0xBB)
		_, err = env.myService.Upload(ctx, env.myUser, "my-overflow.bin", nil, bytes.NewReader(myOverflowPayload), false, 1<<30, nil)
		if err == nil {
			t.Fatalf("expected My Storage upload of 30,000 bytes to be rejected, but it succeeded")
		}
		if !isStorageFullError(err) {
			t.Fatalf("expected storage full error, got: %v", err)
		}

		// 3. My Storage mencoba upload file 20,000 bytes (<= 25,000 sisa aman) -> HARUS BERHASIL.
		myFitPayload := makePayload(20_000, 0xCC)
		myItem, err := env.myService.Upload(ctx, env.myUser, "my-fit.bin", nil, bytes.NewReader(myFitPayload), false, 1<<30, nil)
		if err != nil {
			t.Fatalf("expected My Storage upload of 20,000 bytes to succeed, got: %v", err)
		}
		if myItem.SizeBytes != 20_000 {
			t.Fatalf("expected my item size 20000, got: %d", myItem.SizeBytes)
		}

		// 4. Verifikasi total byte yang tersimpan pada node fisik gabungan.
		usedInDB := getNodeStoredBytes(t, env.db, env.nodeID)
		if usedInDB != 90_000 {
			t.Fatalf("expected combined stored bytes 90,000, got: %d", usedInDB)
		}
		if usedInDB > safeCap {
			t.Fatalf("stored bytes %d exceeded safe capacity %d", usedInDB, safeCap)
		}

		physBytes := getPhysicalDirBytes(t, env.nodePath)
		if physBytes != 90_000 {
			t.Fatalf("expected physical dir bytes 90,000, got: %d", physBytes)
		}
	})

	t.Run("MyStoragePrefill_BlocksOrgOverflow", func(t *testing.T) {
		// Node capacity = 100,000 bytes. Safe capacity = 95,000 bytes.
		const nodeCap int64 = 100_000
		const safeCap int64 = 95_000

		env := setupCapacityTestEnv(t, nodeCap)
		ctx := context.Background()

		// 1. My Storage prefill: 70,000 bytes.
		myPayload := makePayload(70_000, 0x11)
		myItem, err := env.myService.Upload(ctx, env.myUser, "my-prefill.bin", nil, bytes.NewReader(myPayload), false, 1<<30, nil)
		if err != nil {
			t.Fatalf("expected My Storage prefill upload to succeed, got: %v", err)
		}
		if myItem.SizeBytes != 70_000 {
			t.Fatalf("expected my item size 70000, got: %d", myItem.SizeBytes)
		}

		// Sisa kapasitas aman = 95,000 - 70,000 = 25,000 bytes.

		// 2. Organization mencoba upload file 30,000 bytes (> 25,000 sisa aman) -> HARUS DITOLAK.
		orgOverflowPayload := makePayload(30_000, 0x22)
		_, err = env.orgService.Upload(ctx, env.orgUser, "org-overflow.bin", nil, bytes.NewReader(orgOverflowPayload), false, 1<<30, nil)
		if err == nil {
			t.Fatalf("expected Organization upload of 30,000 bytes to be rejected, but it succeeded")
		}
		if !isStorageFullError(err) {
			t.Fatalf("expected storage full error, got: %v", err)
		}

		// 3. Organization mencoba upload file 20,000 bytes (<= 25,000 sisa aman) -> HARUS BERHASIL.
		orgFitPayload := makePayload(20_000, 0x33)
		orgItem, err := env.orgService.Upload(ctx, env.orgUser, "org-fit.bin", nil, bytes.NewReader(orgFitPayload), false, 1<<30, nil)
		if err != nil {
			t.Fatalf("expected Organization upload of 20,000 bytes to succeed, got: %v", err)
		}
		if orgItem.SizeBytes != 20_000 {
			t.Fatalf("expected org item size 20000, got: %d", orgItem.SizeBytes)
		}

		// 4. Verifikasi total byte gabungan.
		usedInDB := getNodeStoredBytes(t, env.db, env.nodeID)
		if usedInDB != 90_000 {
			t.Fatalf("expected combined stored bytes 90,000, got: %d", usedInDB)
		}
		if usedInDB > safeCap {
			t.Fatalf("stored bytes %d exceeded safe capacity %d", usedInDB, safeCap)
		}

		physBytes := getPhysicalDirBytes(t, env.nodePath)
		if physBytes != 90_000 {
			t.Fatalf("expected physical dir bytes 90,000, got: %d", physBytes)
		}
	})
}

// Test 2: Race Condition / Atomic Reservation
// Sisa kapasitas aman node diset hanya cukup untuk 1 file (misalnya 95,000 bytes sisa aman).
// Dua upload konkuren dijalankan secara simultan:
// - Satu upload My Storage (80,000 bytes)
// - Satu upload Organization (80,000 bytes)
//
// Pembuktian yang dituntut:
// 1. Tepat 1 upload berhasil dan tepat 1 upload gagal dengan error storage full.
// 2. Total ukuran byte yang tersimpan pada node fisik tidak pernah melebihi kapasitas aman node.
// 3. File yang gagal upload dibersihkan (cleanup) dari disk fisik sehingga tidak ada file yatim.
func TestConcurrentUploadRaceCondition(t *testing.T) {
	const nodeCap int64 = 100_000
	const safeCap int64 = 95_000
	const fileSize int64 = 80_000 // 80 KB; 2 * 80 KB = 160 KB > 95 KB safe capacity.

	// Jalankan beberapa iterasi untuk memverifikasi reliabilitas pencegahan race condition.
	const iterations = 5

	for iter := 1; iter <= iterations; iter++ {
		t.Run(fmt.Sprintf("Iteration_%d", iter), func(t *testing.T) {
			env := setupCapacityTestEnv(t, nodeCap)
			ctx := context.Background()

			myPayload := makePayload(fileSize, byte(0x10+iter))
			orgPayload := makePayload(fileSize, byte(0x20+iter))

			var wg sync.WaitGroup
			startBarrier := make(chan struct{})

			var myErr error
			var myItem mystorage.Item
			var orgErr error
			var orgItem organization.Item

			wg.Add(2)

			// Goroutine 1: My Storage Upload
			go func() {
				defer wg.Done()
				<-startBarrier
				myItem, myErr = env.myService.Upload(
					ctx,
					env.myUser,
					fmt.Sprintf("race-mystorage-%d.bin", iter),
					nil,
					bytes.NewReader(myPayload),
					false,
					1<<30,
					nil,
				)
			}()

			// Goroutine 2: Organization Upload
			go func() {
				defer wg.Done()
				<-startBarrier
				orgItem, orgErr = env.orgService.Upload(
					ctx,
					env.orgUser,
					fmt.Sprintf("race-org-%d.bin", iter),
					nil,
					bytes.NewReader(orgPayload),
					false,
					1<<30,
					nil,
				)
			}()

			// Lepaskan kedua goroutine secara serentak
			close(startBarrier)
			wg.Wait()

			// Evaluasi hasil
			successCount := 0
			failCount := 0

			if myErr == nil {
				successCount++
			} else {
				failCount++
				if !isStorageFullError(myErr) {
					t.Errorf("myStorage upload failed with unexpected error: %v", myErr)
				}
			}

			if orgErr == nil {
				successCount++
			} else {
				failCount++
				if !isStorageFullError(orgErr) {
					t.Errorf("org upload failed with unexpected error: %v", orgErr)
				}
			}

			// Pembuktian 1: Tepat 1 upload berhasil dan tepat 1 upload gagal
			if successCount != 1 || failCount != 1 {
				t.Fatalf("expected exactly 1 success and 1 failure; got %d successes, %d failures (myErr=%v, orgErr=%v)",
					successCount, failCount, myErr, orgErr)
			}

			// Pembuktian 2: Total ukuran byte yang tersimpan pada node fisik TIDAK PERNAH melebihi safe capacity
			usedInDB := getNodeStoredBytes(t, env.db, env.nodeID)
			if usedInDB > safeCap {
				t.Fatalf("race condition violated: stored bytes in DB (%d) exceeded safe capacity (%d)",
					usedInDB, safeCap)
			}
			if usedInDB != fileSize {
				t.Fatalf("expected exactly one file (%d bytes) in DB, got: %d", fileSize, usedInDB)
			}

			// Pembuktian 3: File fisik pada direktori node tidak melebihi safe capacity dan file gagal dibersihkan
			physBytes := getPhysicalDirBytes(t, env.nodePath)
			if physBytes > safeCap {
				t.Fatalf("race condition violated: physical files on disk (%d bytes) exceeded safe capacity (%d)",
					physBytes, safeCap)
			}
			if physBytes != fileSize {
				t.Fatalf("expected physical directory to contain exactly %d bytes, got: %d (cleanup failed)",
					fileSize, physBytes)
			}

			_ = myItem
			_ = orgItem
		})
	}
}

// Test 3: Multi-Goroutine Concurrent Race Condition
// Menjalankan beberapa upload sekaligus (misal 6 goroutine: 3 My Storage dan 3 Organization).
// Masing-masing file berukuran 60,000 bytes pada node dengan safe capacity 200,000 bytes.
// Maksimal hanya 3 file (180,000 bytes) yang muat.
// Pembuktian: jumlah yang berhasil maksimal 3, sisanya gagal dengan error storage full,
// dan total pemakaian node tidak pernah melebihi 200,000 bytes.
func TestConcurrentMultiUploadRaceCondition(t *testing.T) {
	const nodeCap int64 = 210_527 // capacity - 5% reserve approx 200,000 bytes
	ctx := context.Background()
	env := setupCapacityTestEnv(t, nodeCap)

	safeCap, err := storage.AvailableOnNode(ctx, env.db, env.driver, env.nodeID)
	if err != nil {
		t.Fatalf("failed to calculate initial available capacity: %v", err)
	}

	const fileSize int64 = 60_000
	const numMyUploads = 3
	const numOrgUploads = 3
	const totalUploads = numMyUploads + numOrgUploads

	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	var successCounter int64
	var failureCounter int64
	var unexpectedErrors []error
	var errMu sync.Mutex

	for i := range numMyUploads {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			payload := make([]byte, fileSize)
			_, _ = rand.Read(payload)

			<-startBarrier
			_, uploadErr := env.myService.Upload(
				ctx,
				env.myUser,
				fmt.Sprintf("multi-my-%d.bin", idx),
				nil,
				bytes.NewReader(payload),
				false,
				1<<30,
				nil,
			)

			if uploadErr == nil {
				atomic.AddInt64(&successCounter, 1)
			} else {
				atomic.AddInt64(&failureCounter, 1)
				if !isStorageFullError(uploadErr) {
					errMu.Lock()
					unexpectedErrors = append(unexpectedErrors, fmt.Errorf("my-upload-%d: %w", idx, uploadErr))
					errMu.Unlock()
				}
			}
		}()
	}

	for i := range numOrgUploads {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			payload := make([]byte, fileSize)
			_, _ = rand.Read(payload)

			<-startBarrier
			_, uploadErr := env.orgService.Upload(
				ctx,
				env.orgUser,
				fmt.Sprintf("multi-org-%d.bin", idx),
				nil,
				bytes.NewReader(payload),
				false,
				1<<30,
				nil,
			)

			if uploadErr == nil {
				atomic.AddInt64(&successCounter, 1)
			} else {
				atomic.AddInt64(&failureCounter, 1)
				if !isStorageFullError(uploadErr) {
					errMu.Lock()
					unexpectedErrors = append(unexpectedErrors, fmt.Errorf("org-upload-%d: %w", idx, uploadErr))
					errMu.Unlock()
				}
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if len(unexpectedErrors) > 0 {
		t.Fatalf("unexpected upload errors occurred: %v", unexpectedErrors)
	}

	successes := atomic.LoadInt64(&successCounter)
	failures := atomic.LoadInt64(&failureCounter)

	if successes+failures != totalUploads {
		t.Fatalf("total completed (%d) != expected total (%d)", successes+failures, totalUploads)
	}

	// Kapasitas aman hanya cukup untuk floor(safeCap / fileSize) file.
	maxAllowedSuccess := safeCap / fileSize
	if successes > maxAllowedSuccess {
		t.Fatalf("success count %d exceeded max allowed %d based on safe capacity %d",
			successes, maxAllowedSuccess, safeCap)
	}

	usedInDB := getNodeStoredBytes(t, env.db, env.nodeID)
	if usedInDB > safeCap {
		t.Fatalf("stored bytes in DB (%d) exceeded safe capacity (%d)", usedInDB, safeCap)
	}

	physBytes := getPhysicalDirBytes(t, env.nodePath)
	if physBytes > safeCap {
		t.Fatalf("physical bytes on disk (%d) exceeded safe capacity (%d)", physBytes, safeCap)
	}

	if physBytes != usedInDB {
		t.Fatalf("physical bytes on disk (%d) != DB stored bytes (%d), orphaned files detected",
			physBytes, usedInDB)
	}
}
