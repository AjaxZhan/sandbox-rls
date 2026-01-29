// Package integration provides end-to-end integration tests for the sandbox service.
package integration

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/ajaxzhan/sandbox-rls/api/gen"
	"github.com/ajaxzhan/sandbox-rls/internal/codebase"
	"github.com/ajaxzhan/sandbox-rls/internal/runtime/mock"
	"github.com/ajaxzhan/sandbox-rls/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// testEnv holds the test environment.
type testEnv struct {
	server     *server.Server
	grpcConn   *grpc.ClientConn
	sandboxCli pb.SandboxServiceClient
	codebaseCli pb.CodebaseServiceClient
	cleanup    func()
}

// setupTestEnv creates a test environment with a running server.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create temp directory for codebase storage
	tempDir := t.TempDir()

	// Create mock runtime
	rt := mock.New()

	// Create codebase manager
	cbManager, err := codebase.NewManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create codebase manager: %v", err)
	}

	// Find available port
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := lis.Addr().String()
	lis.Close()

	// Create server
	cfg := &server.Config{
		GRPCAddr: addr,
	}
	srv, err := server.New(cfg, rt, cbManager)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Start server in background
	go func() {
		if err := srv.Start(); err != nil {
			// Server stopped, ignore
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create client connection
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	return &testEnv{
		server:     srv,
		grpcConn:   conn,
		sandboxCli: pb.NewSandboxServiceClient(conn),
		codebaseCli: pb.NewCodebaseServiceClient(conn),
		cleanup: func() {
			conn.Close()
			srv.Stop()
		},
	}
}

// TestFullWorkflow tests the complete sandbox workflow:
// Create Codebase -> Upload Files -> Create Sandbox -> Start -> Exec -> Stop -> Destroy
func TestFullWorkflow(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Step 1: Create a codebase
	t.Log("Step 1: Creating codebase...")
	codebase, err := env.codebaseCli.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "test-project",
		OwnerId: "user_123",
	})
	if err != nil {
		t.Fatalf("failed to create codebase: %v", err)
	}
	t.Logf("Created codebase: %s", codebase.Id)

	// Step 2: Upload a file
	t.Log("Step 2: Uploading file...")
	uploadStream, err := env.codebaseCli.UploadFiles(ctx)
	if err != nil {
		t.Fatalf("failed to start upload: %v", err)
	}

	// Send metadata
	err = uploadStream.Send(&pb.UploadChunk{
		Content: &pb.UploadChunk_Metadata_{
			Metadata: &pb.UploadChunk_Metadata{
				CodebaseId: codebase.Id,
				FilePath:   "hello.txt",
				TotalSize:  13,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to send metadata: %v", err)
	}

	// Send data
	err = uploadStream.Send(&pb.UploadChunk{
		Content: &pb.UploadChunk_Data{
			Data: []byte("Hello, World!"),
		},
	})
	if err != nil {
		t.Fatalf("failed to send data: %v", err)
	}

	uploadResult, err := uploadStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("failed to complete upload: %v", err)
	}
	t.Logf("Uploaded file: %s (%d bytes)", uploadResult.FilePath, uploadResult.Size)

	// Step 3: List files to verify
	t.Log("Step 3: Listing files...")
	fileList, err := env.codebaseCli.ListFiles(ctx, &pb.ListFilesRequest{
		CodebaseId: codebase.Id,
		Recursive:  true,
	})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(fileList.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(fileList.Files))
	}
	t.Logf("Found %d file(s)", len(fileList.Files))

	// Step 4: Create a sandbox
	t.Log("Step 4: Creating sandbox...")
	sandbox, err := env.sandboxCli.CreateSandbox(ctx, &pb.CreateSandboxRequest{
		CodebaseId: codebase.Id,
		Permissions: []*pb.PermissionRule{
			{
				Pattern:    "**/*",
				Permission: pb.Permission_PERMISSION_READ,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	t.Logf("Created sandbox: %s (status: %s)", sandbox.Id, sandbox.Status)

	if sandbox.Status != pb.SandboxStatus_SANDBOX_STATUS_PENDING {
		t.Errorf("expected status PENDING, got %s", sandbox.Status)
	}

	// Step 5: Start the sandbox
	t.Log("Step 5: Starting sandbox...")
	sandbox, err = env.sandboxCli.StartSandbox(ctx, &pb.StartSandboxRequest{
		SandboxId: sandbox.Id,
	})
	if err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}
	t.Logf("Started sandbox: %s (status: %s)", sandbox.Id, sandbox.Status)

	if sandbox.Status != pb.SandboxStatus_SANDBOX_STATUS_RUNNING {
		t.Errorf("expected status RUNNING, got %s", sandbox.Status)
	}

	// Step 6: Execute a command
	t.Log("Step 6: Executing command...")
	execResult, err := env.sandboxCli.Exec(ctx, &pb.ExecRequest{
		SandboxId: sandbox.Id,
		Command:   "echo 'Hello from sandbox!'",
	})
	if err != nil {
		t.Fatalf("failed to execute command: %v", err)
	}
	t.Logf("Command output: %s (exit code: %d)", execResult.Stdout, execResult.ExitCode)

	if execResult.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", execResult.ExitCode)
	}

	// Step 7: Stop the sandbox
	t.Log("Step 7: Stopping sandbox...")
	sandbox, err = env.sandboxCli.StopSandbox(ctx, &pb.StopSandboxRequest{
		SandboxId: sandbox.Id,
	})
	if err != nil {
		t.Fatalf("failed to stop sandbox: %v", err)
	}
	t.Logf("Stopped sandbox: %s (status: %s)", sandbox.Id, sandbox.Status)

	if sandbox.Status != pb.SandboxStatus_SANDBOX_STATUS_STOPPED {
		t.Errorf("expected status STOPPED, got %s", sandbox.Status)
	}

	// Step 8: Destroy the sandbox
	t.Log("Step 8: Destroying sandbox...")
	_, err = env.sandboxCli.DestroySandbox(ctx, &pb.DestroySandboxRequest{
		SandboxId: sandbox.Id,
	})
	if err != nil {
		t.Fatalf("failed to destroy sandbox: %v", err)
	}
	t.Log("Destroyed sandbox")

	// Verify sandbox is gone
	_, err = env.sandboxCli.GetSandbox(ctx, &pb.GetSandboxRequest{
		SandboxId: sandbox.Id,
	})
	if err == nil {
		t.Error("expected error getting destroyed sandbox, got nil")
	}

	// Step 9: Delete the codebase
	t.Log("Step 9: Deleting codebase...")
	_, err = env.codebaseCli.DeleteCodebase(ctx, &pb.DeleteCodebaseRequest{
		CodebaseId: codebase.Id,
	})
	if err != nil {
		t.Fatalf("failed to delete codebase: %v", err)
	}
	t.Log("Deleted codebase")

	t.Log("Full workflow completed successfully!")
}

// TestMultipleSandboxes tests creating multiple sandboxes from the same codebase.
func TestMultipleSandboxes(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create a codebase
	codebase, err := env.codebaseCli.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "shared-project",
		OwnerId: "user_123",
	})
	if err != nil {
		t.Fatalf("failed to create codebase: %v", err)
	}

	// Create multiple sandboxes with different permissions
	sandboxes := make([]*pb.Sandbox, 3)
	permissions := []pb.Permission{
		pb.Permission_PERMISSION_READ,
		pb.Permission_PERMISSION_WRITE,
		pb.Permission_PERMISSION_VIEW,
	}

	for i := 0; i < 3; i++ {
		sandbox, err := env.sandboxCli.CreateSandbox(ctx, &pb.CreateSandboxRequest{
			CodebaseId: codebase.Id,
			Permissions: []*pb.PermissionRule{
				{
					Pattern:    "**/*",
					Permission: permissions[i],
				},
			},
			Labels: map[string]string{
				"index": fmt.Sprintf("%d", i),
			},
		})
		if err != nil {
			t.Fatalf("failed to create sandbox %d: %v", i, err)
		}
		sandboxes[i] = sandbox
		t.Logf("Created sandbox %d: %s", i, sandbox.Id)
	}

	// List sandboxes filtered by codebase
	listResp, err := env.sandboxCli.ListSandboxes(ctx, &pb.ListSandboxesRequest{
		CodebaseId: codebase.Id,
	})
	if err != nil {
		t.Fatalf("failed to list sandboxes: %v", err)
	}

	if len(listResp.Sandboxes) != 3 {
		t.Errorf("expected 3 sandboxes, got %d", len(listResp.Sandboxes))
	}

	// Clean up
	for _, sb := range sandboxes {
		env.sandboxCli.DestroySandbox(ctx, &pb.DestroySandboxRequest{
			SandboxId: sb.Id,
		})
	}
	env.codebaseCli.DeleteCodebase(ctx, &pb.DeleteCodebaseRequest{
		CodebaseId: codebase.Id,
	})
}

// TestFileOperations tests file upload, download, and listing.
func TestFileOperations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Create a codebase
	codebase, err := env.codebaseCli.CreateCodebase(ctx, &pb.CreateCodebaseRequest{
		Name:    "file-test",
		OwnerId: "user_123",
	})
	if err != nil {
		t.Fatalf("failed to create codebase: %v", err)
	}

	// Upload multiple files
	files := map[string][]byte{
		"readme.md":     []byte("# Test Project\n\nThis is a test."),
		"src/main.py":   []byte("print('Hello, World!')"),
		"src/utils.py":  []byte("def helper(): pass"),
		"config.yaml":   []byte("debug: true\nport: 8080"),
	}

	for path, content := range files {
		stream, err := env.codebaseCli.UploadFiles(ctx)
		if err != nil {
			t.Fatalf("failed to start upload for %s: %v", path, err)
		}

		err = stream.Send(&pb.UploadChunk{
			Content: &pb.UploadChunk_Metadata_{
				Metadata: &pb.UploadChunk_Metadata{
					CodebaseId: codebase.Id,
					FilePath:   path,
					TotalSize:  int64(len(content)),
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to send metadata for %s: %v", path, err)
		}

		err = stream.Send(&pb.UploadChunk{
			Content: &pb.UploadChunk_Data{
				Data: content,
			},
		})
		if err != nil {
			t.Fatalf("failed to send data for %s: %v", path, err)
		}

		_, err = stream.CloseAndRecv()
		if err != nil {
			t.Fatalf("failed to complete upload for %s: %v", path, err)
		}
	}

	// List all files
	listResp, err := env.codebaseCli.ListFiles(ctx, &pb.ListFilesRequest{
		CodebaseId: codebase.Id,
		Recursive:  true,
	})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	// Should have 4 files (plus potentially directories)
	fileCount := 0
	for _, f := range listResp.Files {
		if !f.IsDir {
			fileCount++
		}
	}
	if fileCount != 4 {
		t.Errorf("expected 4 files, got %d", fileCount)
	}

	// Download a file and verify content
	downloadStream, err := env.codebaseCli.DownloadFile(ctx, &pb.DownloadFileRequest{
		CodebaseId: codebase.Id,
		FilePath:   "src/main.py",
	})
	if err != nil {
		t.Fatalf("failed to start download: %v", err)
	}

	var downloadedContent []byte
	for {
		chunk, err := downloadStream.Recv()
		if err != nil {
			break
		}
		downloadedContent = append(downloadedContent, chunk.Data...)
	}

	expectedContent := files["src/main.py"]
	if string(downloadedContent) != string(expectedContent) {
		t.Errorf("downloaded content mismatch: got %q, want %q", downloadedContent, expectedContent)
	}

	// Clean up
	env.codebaseCli.DeleteCodebase(ctx, &pb.DeleteCodebaseRequest{
		CodebaseId: codebase.Id,
	})
}
