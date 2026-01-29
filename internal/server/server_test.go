// Package server provides gRPC server implementations for sandbox and codebase services.
package server

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	pb "github.com/ajaxzhan/sandbox-rls/api/gen"
	"github.com/ajaxzhan/sandbox-rls/internal/codebase"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime/mock"
	"github.com/ajaxzhan/sandbox-rls/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// testServer wraps the gRPC server and its dependencies for testing.
type testServer struct {
	lis             *bufconn.Listener
	grpcServer      *grpc.Server
	sandboxService  *SandboxServiceServer
	codebaseService *CodebaseServiceServer
	mockRuntime     *mock.MockRuntime
	codebaseManager *codebase.Manager
	conn            *grpc.ClientConn
	tempDir         string
}

// setupTestServer creates a test gRPC server with mock dependencies.
func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create temp directory for codebase storage
	tempDir := t.TempDir()

	// Create mock runtime
	mockRT := mock.New()

	// Create codebase manager
	cbManager, err := codebase.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create codebase manager: %v", err)
	}

	// Create gRPC server
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()

	// Create service implementations
	sandboxSvc := NewSandboxServiceServer(mockRT, cbManager)
	codebaseSvc := NewCodebaseServiceServer(cbManager)

	// Register services
	pb.RegisterSandboxServiceServer(s, sandboxSvc)
	pb.RegisterCodebaseServiceServer(s, codebaseSvc)

	// Start server in goroutine
	go func() {
		if err := s.Serve(lis); err != nil {
			// Only log if it's not a normal shutdown
			if err != grpc.ErrServerStopped {
				t.Logf("server error: %v", err)
			}
		}
	}()

	return &testServer{
		lis:             lis,
		grpcServer:      s,
		sandboxService:  sandboxSvc,
		codebaseService: codebaseSvc,
		mockRuntime:     mockRT,
		codebaseManager: cbManager,
		tempDir:         tempDir,
	}
}

// getConn returns a gRPC client connection to the test server.
func (ts *testServer) getConn(ctx context.Context, t *testing.T) *grpc.ClientConn {
	t.Helper()

	if ts.conn != nil {
		return ts.conn
	}

	dialer := func(context.Context, string) (net.Conn, error) {
		return ts.lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}

	ts.conn = conn
	return conn
}

// close shuts down the test server.
func (ts *testServer) close() {
	if ts.conn != nil {
		ts.conn.Close()
	}
	ts.grpcServer.GracefulStop()
}

// ============================================
// SandboxService Tests
// ============================================

func TestSandboxService_CreateSandbox(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)

	// First, create a codebase
	cbClient := pb.NewCodebaseServiceClient(conn)
	cbResp, err := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})
	if err != nil {
		t.Fatalf("failed to create codebase: %v", err)
	}

	// Test creating a sandbox
	resp, err := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
		Permissions: []*pb.PermissionRule{
			{
				Pattern:    "*.go",
				Type:       pb.PatternType_PATTERN_TYPE_GLOB,
				Permission: pb.Permission_PERMISSION_READ,
				Priority:   10,
			},
		},
		Labels: map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("CreateSandbox failed: %v", err)
	}

	// Verify response
	if resp.Id == "" {
		t.Error("expected sandbox ID to be set")
	}
	if resp.CodebaseId != cbResp.Id {
		t.Errorf("expected codebase_id %s, got %s", cbResp.Id, resp.CodebaseId)
	}
	if resp.Status != pb.SandboxStatus_SANDBOX_STATUS_PENDING {
		t.Errorf("expected status PENDING, got %v", resp.Status)
	}
	if len(resp.Permissions) != 1 {
		t.Errorf("expected 1 permission rule, got %d", len(resp.Permissions))
	}
	if resp.Labels["env"] != "test" {
		t.Errorf("expected label env=test, got %v", resp.Labels)
	}
}

func TestSandboxService_CreateSandbox_InvalidCodebase(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)

	// Try to create sandbox with non-existent codebase
	_, err := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent codebase")
	}

	// Verify it's a NOT_FOUND error
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestSandboxService_GetSandbox(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create a codebase
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})

	// Create a sandbox
	createResp, _ := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
	})

	// Get the sandbox
	getResp, err := client.GetSandbox(ctx, &pb.GetSandboxRequest{
		SandboxId: createResp.Id,
	})
	if err != nil {
		t.Fatalf("GetSandbox failed: %v", err)
	}

	if getResp.Id != createResp.Id {
		t.Errorf("expected ID %s, got %s", createResp.Id, getResp.Id)
	}
}

func TestSandboxService_GetSandbox_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)

	_, err := client.GetSandbox(ctx, &pb.GetSandboxRequest{
		SandboxId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent sandbox")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestSandboxService_ListSandboxes(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create a codebase
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})

	// Create multiple sandboxes
	for i := 0; i < 3; i++ {
		_, err := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
			CodebaseId: cbResp.Id,
		})
		if err != nil {
			t.Fatalf("CreateSandbox failed: %v", err)
		}
	}

	// List all sandboxes
	listResp, err := client.ListSandboxes(ctx, &pb.ListSandboxesRequest{})
	if err != nil {
		t.Fatalf("ListSandboxes failed: %v", err)
	}

	if len(listResp.Sandboxes) != 3 {
		t.Errorf("expected 3 sandboxes, got %d", len(listResp.Sandboxes))
	}
}

func TestSandboxService_ListSandboxes_FilterByCodebase(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create two codebases
	cb1, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "codebase1",
		OwnerId: "owner1",
	})
	cb2, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "codebase2",
		OwnerId: "owner1",
	})

	// Create sandboxes for each
	for i := 0; i < 2; i++ {
		client.CreateSandbox(ctx, &pb.CreateSandboxRequest{CodebaseId: cb1.Id})
	}
	for i := 0; i < 3; i++ {
		client.CreateSandbox(ctx, &pb.CreateSandboxRequest{CodebaseId: cb2.Id})
	}

	// List sandboxes filtered by codebase
	listResp, err := client.ListSandboxes(ctx, &pb.ListSandboxesRequest{
		CodebaseId: cb1.Id,
	})
	if err != nil {
		t.Fatalf("ListSandboxes failed: %v", err)
	}

	if len(listResp.Sandboxes) != 2 {
		t.Errorf("expected 2 sandboxes for cb1, got %d", len(listResp.Sandboxes))
	}
}

func TestSandboxService_StartSandbox(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create codebase and sandbox
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})
	createResp, _ := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
	})

	// Start the sandbox
	startResp, err := client.StartSandbox(ctx, &pb.StartSandboxRequest{
		SandboxId: createResp.Id,
	})
	if err != nil {
		t.Fatalf("StartSandbox failed: %v", err)
	}

	if startResp.Status != pb.SandboxStatus_SANDBOX_STATUS_RUNNING {
		t.Errorf("expected status RUNNING, got %v", startResp.Status)
	}
	if startResp.StartedAt == nil {
		t.Error("expected started_at to be set")
	}
}

func TestSandboxService_StopSandbox(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create, start sandbox
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})
	createResp, _ := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
	})
	client.StartSandbox(ctx, &pb.StartSandboxRequest{SandboxId: createResp.Id})

	// Stop the sandbox
	stopResp, err := client.StopSandbox(ctx, &pb.StopSandboxRequest{
		SandboxId: createResp.Id,
	})
	if err != nil {
		t.Fatalf("StopSandbox failed: %v", err)
	}

	if stopResp.Status != pb.SandboxStatus_SANDBOX_STATUS_STOPPED {
		t.Errorf("expected status STOPPED, got %v", stopResp.Status)
	}
	if stopResp.StoppedAt == nil {
		t.Error("expected stopped_at to be set")
	}
}

func TestSandboxService_DestroySandbox(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create sandbox
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})
	createResp, _ := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
	})

	// Destroy the sandbox
	_, err := client.DestroySandbox(ctx, &pb.DestroySandboxRequest{
		SandboxId: createResp.Id,
	})
	if err != nil {
		t.Fatalf("DestroySandbox failed: %v", err)
	}

	// Verify it's gone
	_, err = client.GetSandbox(ctx, &pb.GetSandboxRequest{
		SandboxId: createResp.Id,
	})
	if err == nil {
		t.Fatal("expected error for destroyed sandbox")
	}
}

func TestSandboxService_Exec(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Setup: create and start sandbox
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})
	createResp, _ := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
	})
	client.StartSandbox(ctx, &pb.StartSandboxRequest{SandboxId: createResp.Id})

	// Configure mock exec result
	ts.mockRuntime.OnExec = func(ctx context.Context, sandboxID string, req *types.ExecRequest) (*types.ExecResult, error) {
		return &types.ExecResult{
			Stdout:   "hello world",
			Stderr:   "",
			ExitCode: 0,
			Duration: time.Millisecond * 100,
		}, nil
	}

	// Execute command
	execResp, err := client.Exec(ctx, &pb.ExecRequest{
		SandboxId: createResp.Id,
		Command:   "echo hello world",
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if execResp.Stdout != "hello world" {
		t.Errorf("expected stdout 'hello world', got '%s'", execResp.Stdout)
	}
	if execResp.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", execResp.ExitCode)
	}
}

func TestSandboxService_Exec_NotRunning(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewSandboxServiceClient(conn)
	cbClient := pb.NewCodebaseServiceClient(conn)

	// Create sandbox but don't start it
	cbResp, _ := cbClient.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-codebase",
		OwnerId: "owner1",
	})
	createResp, _ := client.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: cbResp.Id,
	})

	// Try to exec on non-running sandbox
	_, err := client.Exec(ctx, &pb.ExecRequest{
		SandboxId: createResp.Id,
		Command:   "echo hello",
	})
	if err == nil {
		t.Fatal("expected error when exec on non-running sandbox")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("expected FAILED_PRECONDITION, got %v", st.Code())
	}
}

// ============================================
// CodebaseService Tests
// ============================================

func TestCodebaseService_CreateCodebase(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	resp, err := client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "my-project",
		OwnerId: "user123",
	})
	if err != nil {
		t.Fatalf("CreateCodebase failed: %v", err)
	}

	if resp.Id == "" {
		t.Error("expected codebase ID to be set")
	}
	if resp.Name != "my-project" {
		t.Errorf("expected name 'my-project', got '%s'", resp.Name)
	}
	if resp.OwnerId != "user123" {
		t.Errorf("expected owner_id 'user123', got '%s'", resp.OwnerId)
	}
}

func TestCodebaseService_CreateCodebase_InvalidRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	// Empty name
	_, err := client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "",
		OwnerId: "user123",
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected INVALID_ARGUMENT, got %v", st.Code())
	}
}

func TestCodebaseService_GetCodebase(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	// Create a codebase
	createResp, _ := client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "my-project",
		OwnerId: "user123",
	})

	// Get it back
	getResp, err := client.GetCodebase(ctx, &pb.GetCodebaseRequest{
		CodebaseId: createResp.Id,
	})
	if err != nil {
		t.Fatalf("GetCodebase failed: %v", err)
	}

	if getResp.Id != createResp.Id {
		t.Errorf("expected ID %s, got %s", createResp.Id, getResp.Id)
	}
	if getResp.Name != "my-project" {
		t.Errorf("expected name 'my-project', got '%s'", getResp.Name)
	}
}

func TestCodebaseService_GetCodebase_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	_, err := client.GetCodebase(ctx, &pb.GetCodebaseRequest{
		CodebaseId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent codebase")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestCodebaseService_ListCodebases(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	// Create multiple codebases
	for i := 0; i < 3; i++ {
		client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
			Name:    "project",
			OwnerId: "user123",
		})
	}
	client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "other-project",
		OwnerId: "user456",
	})

	// List for specific owner
	listResp, err := client.ListCodebases(ctx, &pb.ListCodebasesRequest{
		OwnerId: "user123",
	})
	if err != nil {
		t.Fatalf("ListCodebases failed: %v", err)
	}

	if len(listResp.Codebases) != 3 {
		t.Errorf("expected 3 codebases for user123, got %d", len(listResp.Codebases))
	}
}

func TestCodebaseService_DeleteCodebase(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	// Create a codebase
	createResp, _ := client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "my-project",
		OwnerId: "user123",
	})

	// Delete it
	_, err := client.DeleteCodebase(ctx, &pb.DeleteCodebaseRequest{
		CodebaseId: createResp.Id,
	})
	if err != nil {
		t.Fatalf("DeleteCodebase failed: %v", err)
	}

	// Verify it's gone
	_, err = client.GetCodebase(ctx, &pb.GetCodebaseRequest{
		CodebaseId: createResp.Id,
	})
	if err == nil {
		t.Fatal("expected error for deleted codebase")
	}
}

func TestCodebaseService_ListFiles(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	// Create a codebase
	createResp, _ := client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "my-project",
		OwnerId: "user123",
	})

	// Write some files using the codebase manager directly
	ts.codebaseManager.WriteFile(ctx, createResp.Id, "main.go", bytes.NewReader([]byte("package main")))
	ts.codebaseManager.WriteFile(ctx, createResp.Id, "README.md", bytes.NewReader([]byte("# README")))

	// List files
	listResp, err := client.ListFiles(ctx, &pb.ListFilesRequest{
		CodebaseId: createResp.Id,
		Path:       "",
		Recursive:  false,
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(listResp.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(listResp.Files))
	}
}

func TestCodebaseService_UploadAndDownload(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.close()

	ctx := context.Background()
	conn := ts.getConn(ctx, t)
	client := pb.NewCodebaseServiceClient(conn)

	// Create a codebase
	createResp, _ := client.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "my-project",
		OwnerId: "user123",
	})

	// Upload a file using streaming
	uploadStream, err := client.UploadFiles(ctx)
	if err != nil {
		t.Fatalf("failed to create upload stream: %v", err)
	}

	// Send metadata
	err = uploadStream.Send(&pb.UploadChunk{
		Content: &pb.UploadChunk_Metadata_{
			Metadata: &pb.UploadChunk_Metadata{
				CodebaseId: createResp.Id,
				FilePath:   "test.txt",
				TotalSize:  12,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to send metadata: %v", err)
	}

	// Send data
	err = uploadStream.Send(&pb.UploadChunk{
		Content: &pb.UploadChunk_Data{
			Data: []byte("hello world!"),
		},
	})
	if err != nil {
		t.Fatalf("failed to send data: %v", err)
	}

	// Close and receive result
	uploadResult, err := uploadStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("failed to close upload stream: %v", err)
	}

	if uploadResult.FilePath != "test.txt" {
		t.Errorf("expected file_path 'test.txt', got '%s'", uploadResult.FilePath)
	}

	// Download the file
	downloadStream, err := client.DownloadFile(ctx, &pb.DownloadFileRequest{
		CodebaseId: createResp.Id,
		FilePath:   "test.txt",
	})
	if err != nil {
		t.Fatalf("failed to create download stream: %v", err)
	}

	var downloadedData []byte
	for {
		chunk, err := downloadStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to receive download chunk: %v", err)
		}
		downloadedData = append(downloadedData, chunk.Data...)
	}

	if string(downloadedData) != "hello world!" {
		t.Errorf("expected 'hello world!', got '%s'", string(downloadedData))
	}
}

// ============================================
// Server Lifecycle Tests
// ============================================

func TestServer_New(t *testing.T) {
	tempDir := t.TempDir()
	mockRT := mock.New()

	cbManager, err := codebase.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create codebase manager: %v", err)
	}

	// Create server config
	cfg := &Config{
		GRPCAddr: ":0", // Use any available port
	}

	server, err := New(cfg, mockRT, cbManager)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be non-nil")
	}
}

// Helper function to verify runtime interface
func verifyRuntimeInterface(t *testing.T, rt runtime.RuntimeWithExecutor) {
	t.Helper()
	if rt == nil {
		t.Fatal("runtime is nil")
	}
	if rt.Name() == "" {
		t.Error("runtime name should not be empty")
	}
}

func TestMockRuntimeInterface(t *testing.T) {
	mockRT := mock.New()
	verifyRuntimeInterface(t, mockRT)
}
