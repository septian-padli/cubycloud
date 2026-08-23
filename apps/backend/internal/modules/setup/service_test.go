package setup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/middleware"
	"github.com/cubycloud/backend/internal/storage"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

const testValidRecoveryKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRecoveryKeyGeneration(t *testing.T) {
	recoveryKeyBytes := make([]byte, 32)
	if _, err := rand.Read(recoveryKeyBytes); err != nil {
		t.Fatalf("generate recovery key: %v", err)
	}
	recoveryKey := hex.EncodeToString(recoveryKeyBytes)

	if len(recoveryKey) != 64 {
		t.Errorf("recovery key length = %d, want 64", len(recoveryKey))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(recoveryKey), bcryptCost)
	if err != nil {
		t.Fatalf("bcrypt hash: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(recoveryKey)); err != nil {
		t.Errorf("bcrypt verify failed: %v", err)
	}
}

func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid 8 chars", "password", false},
		{"valid long", "verylongpassword123", false},
		{"invalid short", "pass", true},
		{"invalid empty", "", true},
		{"edge case 7", "passwor", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlacementStrategyValidation(t *testing.T) {
	valid := []string{"FILL_FIRST", "MOST_FREE_SPACE", "ROUND_ROBIN"}
	invalid := []string{"", "INVALID", "fill_first"}

	for _, s := range valid {
		if !isValidPlacementStrategy(s) {
			t.Errorf("strategy %q should be valid", s)
		}
	}

	for _, s := range invalid {
		if isValidPlacementStrategy(s) {
			t.Errorf("strategy %q should be invalid", s)
		}
	}
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return echo.NewHTTPError(400, "password must be at least 8 characters")
	}
	return nil
}

func isValidPlacementStrategy(s string) bool {
	switch s {
	case "FILL_FIRST", "MOST_FREE_SPACE", "ROUND_ROBIN":
		return true
	default:
		return false
	}
}

// Integration tests

func TestCompleteSetup_Success_LocalOnly(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud: CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}
	if resp.Redirect != "/drive/home" {
		t.Errorf("Redirect = %q, want /drive/home", resp.Redirect)
	}

	var user database.User
	err = db.NewSelect().Model(&user).Where("username = ?", "admin").Scan(ctx)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if user.Role != "SUPER_ADMIN" {
		t.Errorf("Role = %q, want SUPER_ADMIN", user.Role)
	}
	if user.QuotaBytes != -1 {
		t.Errorf("QuotaBytes = %d, want -1", user.QuotaBytes)
	}

	if !guard.IsCompleted() {
		t.Error("setup guard not marked completed")
	}

	var setting database.Setting
	err = db.NewSelect().Model(&setting).Where("key = ?", "is_setup_completed").Scan(ctx)
	if err != nil {
		t.Fatalf("query setting: %v", err)
	}
	if setting.Value != "true" {
		t.Errorf("is_setup_completed = %q, want true", setting.Value)
	}
}

func TestCompleteSetup_Success_LocalAndCloud(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	mockS3 := mockS3Server(t, "success")
	defer mockS3.Close()

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud: CloudConfig{
			Enabled:   true,
			Provider:  "R2",
			Endpoint:  mockS3.URL,
			Bucket:    "cuby",
			Region:    "auto",
			AccessKey: "test-key",
			SecretKey: "test-secret",
		},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}

	var count int
	count, err = db.NewSelect().Model((*database.StorageNode)(nil)).Where("type = ?", "S3").Count(ctx)
	if err != nil {
		t.Fatalf("count S3 nodes: %v", err)
	}
	if count != 1 {
		t.Errorf("S3 nodes = %d, want 1", count)
	}
}

func TestCompleteSetup_AlreadyCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	ctx := context.Background()
	_, err := db.NewInsert().Model(&database.Setting{Key: "is_setup_completed", Value: "true"}).Exec(ctx)
	if err != nil {
		t.Fatalf("insert setting: %v", err)
	}

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)
	guard.MarkCompleted()

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/tmp/storage", CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	_, err = svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for already completed setup")
	}
}

func TestCompleteSetup_UsernameDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	ctx := context.Background()
	existingUser := &database.User{
		ID:           "01J5X5X5X5X5X5X5X5X5X5X5X5",
		Username:     "admin",
		Email:        "other@local",
		PasswordHash: "hash",
		Role:         "REGULAR",
		QuotaBytes:   -1,
	}
	_, err := db.NewInsert().Model(existingUser).Exec(ctx)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/tmp/storage", CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	_, err = svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestCompleteSetup_EmailDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	ctx := context.Background()
	existingUser := &database.User{
		ID:           "01J5X5X5X5X5X5X5X5X5X5X5X5",
		Username:     "other",
		Email:        "admin@local",
		PasswordHash: "hash",
		Role:         "REGULAR",
		QuotaBytes:   -1,
	}
	_, err := db.NewInsert().Model(existingUser).Exec(ctx)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/tmp/storage", CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	_, err = svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestCompleteSetup_PasswordTooShort(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "short",
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/tmp/storage", CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	_, err := svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for password too short")
	}
}

func TestCompleteSetup_NoEnabledNodes(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/tmp/storage", CapacityBytes: 500000000000, Enabled: false},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	_, err := svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for no enabled nodes")
	}
}

func TestCompleteSetup_LocalNodePathNotExist(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	driver := storage.NewLocalDriver(tempDir)
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/nonexistent/path", CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	_, err := svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestCompleteSetup_LocalNodePathNotWritable(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "readonly")
	os.MkdirAll(nodePath, 0555)
	defer os.Chmod(nodePath, 0755)

	driver := storage.NewLocalDriver(tempDir)
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	_, err := svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for non-writable path")
	}
}

func TestCompleteSetup_CloudCredentialsInvalid(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	mockS3 := mockS3Server(t, "auth_fail")
	defer mockS3.Close()

	driver := &mockStorageDriver{validateErr: errors.New("403 Forbidden")}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud: CloudConfig{
			Enabled:   true,
			Provider:  "R2",
			Endpoint:  mockS3.URL,
			Bucket:    "cuby",
			Region:    "auto",
			AccessKey: "bad-key",
			SecretKey: "bad-secret",
		},
	}

	ctx := context.Background()
	_, err := svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error for invalid cloud credentials")
	}
}

func TestCompleteSetup_ConcurrentRequests(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()

	var wg sync.WaitGroup
	results := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CompleteSetup(ctx, req)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	errorCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	if successCount != 1 {
		t.Errorf("success count = %d, want 1", successCount)
	}
	if errorCount != 4 {
		t.Errorf("error count = %d, want 4", errorCount)
	}
}

func TestCompleteSetup_RecoveryKeyBcrypt(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if len(resp.RecoveryKey) != 64 {
		t.Errorf("RecoveryKey length = %d, want 64", len(resp.RecoveryKey))
	}

	var user database.User
	err = db.NewSelect().Model(&user).Where("username = ?", "admin").Scan(ctx)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}

	if user.RecoveryKeyHash == nil {
		t.Fatal("RecoveryKeyHash is nil")
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.RecoveryKeyHash), []byte(resp.RecoveryKey))
	if err != nil {
		t.Errorf("bcrypt verify failed: %v", err)
	}
}

func TestCompleteSetup_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	// Insert duplicate username to force rollback during transaction
	ctx := context.Background()
	existingUser := &database.User{
		ID:           "01J5X5X5X5X5X5X5X5X5X5X5X5",
		Username:     "admin",
		Email:        "other@local",
		PasswordHash: "hash",
		Role:         "REGULAR",
		QuotaBytes:   -1,
	}
	_, err := db.NewInsert().Model(existingUser).Exec(ctx)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username: "admin",
			Email:    "admin@local",
			Password: "password123",
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud:             CloudConfig{Enabled: false},
	}

	_, err = svc.CompleteSetup(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify rollback: no settings inserted
	var setting database.Setting
	err = db.NewSelect().Model(&setting).Where("key = ?", "is_setup_completed").Scan(ctx)
	if err == nil {
		t.Error("expected no setting after rollback, but found one")
	}

	// Verify guard not marked
	if guard.IsCompleted() {
		t.Error("guard should not be marked after rollback")
	}
}

func TestCompleteSetup_MultipleStorageNodes(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	node1 := filepath.Join(tempDir, "storage1")
	node2 := filepath.Join(tempDir, "storage2")
	node3 := filepath.Join(tempDir, "storage3")
	os.MkdirAll(node1, 0755)
	os.MkdirAll(node2, 0755)
	os.MkdirAll(node3, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: node1, CapacityBytes: 500000000000, Enabled: true},
			{Path: node2, CapacityBytes: 1000000000000, Enabled: true},
			{Path: node3, CapacityBytes: 250000000000, Enabled: false}, // disabled
		},
		PlacementStrategy: "ROUND_ROBIN",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}

	// Verify only enabled nodes stored
	var count int
	count, err = db.NewSelect().Model((*database.StorageNode)(nil)).Where("enabled = ?", true).Count(ctx)
	if err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if count != 2 {
		t.Errorf("enabled nodes = %d, want 2", count)
	}

	// Verify placement strategy
	var setting database.Setting
	err = db.NewSelect().Model(&setting).Where("key = ?", "placement_strategy").Scan(ctx)
	if err != nil {
		t.Fatalf("query placement_strategy: %v", err)
	}
	if setting.Value != "ROUND_ROBIN" {
		t.Errorf("placement_strategy = %q, want ROUND_ROBIN", setting.Value)
	}
}

func TestCompleteSetup_CloudOnlyNoLocalNodes(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	mockS3 := mockS3Server(t, "success")
	defer mockS3.Close()

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes:      []StorageNodeInput{}, // no local nodes
		PlacementStrategy: "FILL_FIRST",
		Cloud: CloudConfig{
			Enabled:   true,
			Provider:  "R2",
			Endpoint:  mockS3.URL,
			Bucket:    "cuby",
			Region:    "auto",
			AccessKey: "test-key",
			SecretKey: "test-secret",
		},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}

	// Verify only S3 node
	var localCount, s3Count int
	localCount, err = db.NewSelect().Model((*database.StorageNode)(nil)).Where("type = ?", "LOCAL").Count(ctx)
	if err != nil {
		t.Fatalf("count local: %v", err)
	}
	s3Count, err = db.NewSelect().Model((*database.StorageNode)(nil)).Where("type = ?", "S3").Count(ctx)
	if err != nil {
		t.Fatalf("count s3: %v", err)
	}

	if localCount != 0 {
		t.Errorf("local nodes = %d, want 0", localCount)
	}
	if s3Count != 1 {
		t.Errorf("s3 nodes = %d, want 1", s3Count)
	}
}

func TestCompleteSetup_EmptyStorageNodesWithCloud(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	mockS3 := mockS3Server(t, "success")
	defer mockS3.Close()

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes:      []StorageNodeInput{},
		PlacementStrategy: "FILL_FIRST",
		Cloud: CloudConfig{
			Enabled:   true,
			Provider:  "R2",
			Endpoint:  mockS3.URL,
			Bucket:    "cuby",
			Region:    "auto",
			AccessKey: "test-key",
			SecretKey: "test-secret",
		},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}

	var count int
	count, err = db.NewSelect().Model((*database.StorageNode)(nil)).Count(ctx)
	if err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if count != 1 {
		t.Errorf("total nodes = %d, want 1 (S3 only)", count)
	}
}

func TestCompleteSetup_AllLocalNodesDisabled(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	mockS3 := mockS3Server(t, "success")
	defer mockS3.Close()

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: "/tmp/disabled1", CapacityBytes: 500000000000, Enabled: false},
			{Path: "/tmp/disabled2", CapacityBytes: 500000000000, Enabled: false},
		},
		PlacementStrategy: "FILL_FIRST",
		Cloud: CloudConfig{
			Enabled:   true,
			Provider:  "R2",
			Endpoint:  mockS3.URL,
			Bucket:    "cuby",
			Region:    "auto",
			AccessKey: "test-key",
			SecretKey: "test-secret",
		},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}

	var localCount, s3Count int
	localCount, err = db.NewSelect().Model((*database.StorageNode)(nil)).Where("type = ?", "LOCAL").Count(ctx)
	if err != nil {
		t.Fatalf("count local: %v", err)
	}
	s3Count, err = db.NewSelect().Model((*database.StorageNode)(nil)).Where("type = ?", "S3").Count(ctx)
	if err != nil {
		t.Fatalf("count s3: %v", err)
	}

	if localCount != 0 {
		t.Errorf("local nodes = %d, want 0", localCount)
	}
	if s3Count != 1 {
		t.Errorf("s3 nodes = %d, want 1", s3Count)
	}
}

func TestHandler_CompleteSetup_HTTP(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	handler := NewHandler(db, driver, guard, tempDir, "test-jwt-secret")

	e := echo.New()
	reqBody := `{
		"admin": {
			"username": "admin",
			"email": "admin@local",
			"password": "password123",
			"recovery_key": "` + testValidRecoveryKey + `"
		},
		"storage_nodes": [
			{"path": "` + nodePath + `", "capacity_bytes": 500000000000, "enabled": true}
		],
		"placement_strategy": "FILL_FIRST",
		"cloud": {"enabled": false}
	}`

	req := httptest.NewRequest("POST", "/api/v1/setup/complete", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CompleteSetup(c)
	if err != nil {
		t.Fatalf("handler.CompleteSetup: %v", err)
	}

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	// Verify cookie set
	cookies := rec.Result().Cookies()
	var found bool
	for _, cookie := range cookies {
		if cookie.Name == "cuby_token" {
			found = true
			if !cookie.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Error("cookie should be SameSite=Lax")
			}
			if cookie.MaxAge != 7*24*3600 {
				t.Errorf("cookie MaxAge = %d, want %d", cookie.MaxAge, 7*24*3600)
			}
			break
		}
	}
	if !found {
		t.Error("cuby_token cookie not set")
	}
}

func TestHandler_CompleteSetup_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	handler := NewHandler(db, driver, guard, tempDir, "test-jwt-secret")

	e := echo.New()
	req := httptest.NewRequest("POST", "/api/v1/setup/complete", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CompleteSetup(c)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRegisterRoutes(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	e := echo.New()
	RegisterRoutes(e, db, driver, guard, tempDir, "test-jwt-secret")

	// Verify route registered
	routes := e.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/v1/setup/complete" && route.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("POST /api/v1/setup/complete route not registered")
	}
}

func TestCompleteSetup_MostFreeSpaceStrategy(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	tempDir := createTempDir(t)
	nodePath := filepath.Join(tempDir, "storage1")
	os.MkdirAll(nodePath, 0755)

	driver := &mockStorageDriver{}
	guard := middleware.NewSetupGuard(false)

	svc := NewService(db, driver, guard, tempDir)

	req := &CompleteSetupRequest{
		Admin: AdminUser{
			Username:    "admin",
			Email:       "admin@local",
			Password:    "password123",
			RecoveryKey: testValidRecoveryKey,
		},
		StorageNodes: []StorageNodeInput{
			{Path: nodePath, CapacityBytes: 500000000000, Enabled: true},
		},
		PlacementStrategy: "MOST_FREE_SPACE",
		Cloud:             CloudConfig{Enabled: false},
	}

	ctx := context.Background()
	resp, err := svc.CompleteSetup(ctx, req)
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}

	if resp.RecoveryKey == "" {
		t.Error("RecoveryKey empty")
	}

	var setting database.Setting
	err = db.NewSelect().Model(&setting).Where("key = ?", "placement_strategy").Scan(ctx)
	if err != nil {
		t.Fatalf("query placement_strategy: %v", err)
	}
	if setting.Value != "MOST_FREE_SPACE" {
		t.Errorf("placement_strategy = %q, want MOST_FREE_SPACE", setting.Value)
	}
}
