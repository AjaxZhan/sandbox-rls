// Package server provides gRPC server implementations for sandbox and codebase services.
package server

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	pb "github.com/ajaxzhan/sandbox-rls/api/gen"
	"github.com/ajaxzhan/sandbox-rls/internal/codebase"
	sbruntime "github.com/ajaxzhan/sandbox-rls/internal/runtime"
	"github.com/ajaxzhan/sandbox-rls/pkg/types"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Config holds server configuration.
type Config struct {
	GRPCAddr string
	RESTAddr string // Optional REST gateway address
}

// Server represents the gRPC server.
type Server struct {
	config          *Config
	grpcServer      *grpc.Server
	httpServer      *http.Server
	sandboxService  *SandboxServiceServer
	codebaseService *CodebaseServiceServer
	sbRuntime       sbruntime.RuntimeWithExecutor
	codebaseManager *codebase.Manager
	mu              sync.Mutex
}

// New creates a new gRPC server.
func New(cfg *Config, rt sbruntime.RuntimeWithExecutor, cbManager *codebase.Manager) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if rt == nil {
		return nil, errors.New("runtime is required")
	}
	if cbManager == nil {
		return nil, errors.New("codebase manager is required")
	}

	grpcServer := grpc.NewServer()
	sandboxSvc := NewSandboxServiceServer(rt, cbManager)
	codebaseSvc := NewCodebaseServiceServer(cbManager)

	pb.RegisterSandboxServiceServer(grpcServer, sandboxSvc)
	pb.RegisterCodebaseServiceServer(grpcServer, codebaseSvc)

	return &Server{
		config:          cfg,
		grpcServer:      grpcServer,
		sandboxService:  sandboxSvc,
		codebaseService: codebaseSvc,
		sbRuntime:       rt,
		codebaseManager: cbManager,
	}, nil
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.config.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	return s.grpcServer.Serve(lis)
}

// Stop gracefully stops the server.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpServer != nil {
		s.httpServer.Close()
	}
	s.grpcServer.GracefulStop()
}

// StartWithGateway starts both gRPC and REST gateway servers.
func (s *Server) StartWithGateway() error {
	// Start gRPC server in background
	grpcLis, err := net.Listen("tcp", s.config.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC address: %w", err)
	}

	errCh := make(chan error, 2)

	go func() {
		if err := s.grpcServer.Serve(grpcLis); err != nil {
			errCh <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Create REST gateway mux
	ctx := context.Background()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Register sandbox service handler
	err = pb.RegisterSandboxServiceHandlerFromEndpoint(ctx, mux, s.config.GRPCAddr, opts)
	if err != nil {
		return fmt.Errorf("failed to register sandbox service gateway: %w", err)
	}

	// Register codebase service handler
	err = pb.RegisterCodebaseServiceHandlerFromEndpoint(ctx, mux, s.config.GRPCAddr, opts)
	if err != nil {
		return fmt.Errorf("failed to register codebase service gateway: %w", err)
	}

	// Create HTTP server
	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:    s.config.RESTAddr,
		Handler: mux,
	}
	s.mu.Unlock()

	// Start HTTP server
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Wait for an error from either server
	return <-errCh
}

// ============================================
// SandboxServiceServer Implementation
// ============================================

// SandboxServiceServer implements the SandboxService gRPC interface.
type SandboxServiceServer struct {
	pb.UnimplementedSandboxServiceServer
	runtime         sbruntime.RuntimeWithExecutor
	codebaseManager *codebase.Manager
}

// NewSandboxServiceServer creates a new SandboxServiceServer.
func NewSandboxServiceServer(rt sbruntime.RuntimeWithExecutor, cbManager *codebase.Manager) *SandboxServiceServer {
	return &SandboxServiceServer{
		runtime:         rt,
		codebaseManager: cbManager,
	}
}

// generateSandboxID generates a unique sandbox ID.
func generateSandboxID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "sb_" + hex.EncodeToString(bytes)
}

// CreateSandbox creates a new sandbox with specified configuration.
func (s *SandboxServiceServer) CreateSandbox(ctx context.Context, req *pb.CreateSandboxRequest) (*pb.Sandbox, error) {
	if req.CodebaseId == "" {
		return nil, status.Error(codes.InvalidArgument, "codebase_id is required")
	}

	// Verify codebase exists
	cb, err := s.codebaseManager.GetCodebase(ctx, req.CodebaseId)
	if err != nil {
		if errors.Is(err, types.ErrCodebaseNotFound) {
			return nil, status.Error(codes.NotFound, "codebase not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get codebase: %v", err)
	}

	// Convert proto permissions to internal types
	permissions := make([]types.PermissionRule, 0, len(req.Permissions))
	for _, p := range req.Permissions {
		permissions = append(permissions, types.PermissionRule{
			Pattern:    p.Pattern,
			Type:       convertPatternType(p.Type),
			Permission: convertPermission(p.Permission),
			Priority:   int(p.Priority),
		})
	}

	// Create sandbox config
	config := &sbruntime.SandboxConfig{
		ID:           generateSandboxID(),
		CodebaseID:   req.CodebaseId,
		CodebasePath: cb.Path,
		Permissions:  permissions,
		Labels:       req.Labels,
	}

	// Create sandbox via runtime
	sandbox, err := s.runtime.Create(ctx, config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create sandbox: %v", err)
	}

	return sandboxToProto(sandbox), nil
}

// GetSandbox retrieves information about a sandbox.
func (s *SandboxServiceServer) GetSandbox(ctx context.Context, req *pb.GetSandboxRequest) (*pb.Sandbox, error) {
	if req.SandboxId == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox_id is required")
	}

	sandbox, err := s.runtime.Get(ctx, req.SandboxId)
	if err != nil {
		if errors.Is(err, types.ErrSandboxNotFound) {
			return nil, status.Error(codes.NotFound, "sandbox not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get sandbox: %v", err)
	}

	return sandboxToProto(sandbox), nil
}

// ListSandboxes lists all sandboxes.
func (s *SandboxServiceServer) ListSandboxes(ctx context.Context, req *pb.ListSandboxesRequest) (*pb.ListSandboxesResponse, error) {
	sandboxes, err := s.runtime.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list sandboxes: %v", err)
	}

	// Filter by codebase if specified
	var filtered []*types.Sandbox
	if req.CodebaseId != "" {
		for _, sb := range sandboxes {
			if sb.CodebaseID == req.CodebaseId {
				filtered = append(filtered, sb)
			}
		}
	} else {
		filtered = sandboxes
	}

	// Convert to proto
	pbSandboxes := make([]*pb.Sandbox, 0, len(filtered))
	for _, sb := range filtered {
		pbSandboxes = append(pbSandboxes, sandboxToProto(sb))
	}

	return &pb.ListSandboxesResponse{
		Sandboxes: pbSandboxes,
	}, nil
}

// StartSandbox starts a pending sandbox.
func (s *SandboxServiceServer) StartSandbox(ctx context.Context, req *pb.StartSandboxRequest) (*pb.Sandbox, error) {
	if req.SandboxId == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox_id is required")
	}

	err := s.runtime.Start(ctx, req.SandboxId)
	if err != nil {
		if errors.Is(err, types.ErrSandboxNotFound) {
			return nil, status.Error(codes.NotFound, "sandbox not found")
		}
		if errors.Is(err, types.ErrAlreadyRunning) {
			return nil, status.Error(codes.FailedPrecondition, "sandbox is already running")
		}
		return nil, status.Errorf(codes.Internal, "failed to start sandbox: %v", err)
	}

	// Get updated sandbox state
	sandbox, err := s.runtime.Get(ctx, req.SandboxId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get sandbox after start: %v", err)
	}

	return sandboxToProto(sandbox), nil
}

// StopSandbox stops a running sandbox.
func (s *SandboxServiceServer) StopSandbox(ctx context.Context, req *pb.StopSandboxRequest) (*pb.Sandbox, error) {
	if req.SandboxId == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox_id is required")
	}

	err := s.runtime.Stop(ctx, req.SandboxId)
	if err != nil {
		if errors.Is(err, types.ErrSandboxNotFound) {
			return nil, status.Error(codes.NotFound, "sandbox not found")
		}
		if errors.Is(err, types.ErrNotRunning) {
			return nil, status.Error(codes.FailedPrecondition, "sandbox is not running")
		}
		return nil, status.Errorf(codes.Internal, "failed to stop sandbox: %v", err)
	}

	// Get updated sandbox state
	sandbox, err := s.runtime.Get(ctx, req.SandboxId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get sandbox after stop: %v", err)
	}

	return sandboxToProto(sandbox), nil
}

// DestroySandbox destroys a sandbox and releases resources.
func (s *SandboxServiceServer) DestroySandbox(ctx context.Context, req *pb.DestroySandboxRequest) (*pb.Empty, error) {
	if req.SandboxId == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox_id is required")
	}

	err := s.runtime.Destroy(ctx, req.SandboxId)
	if err != nil {
		if errors.Is(err, types.ErrSandboxNotFound) {
			return nil, status.Error(codes.NotFound, "sandbox not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to destroy sandbox: %v", err)
	}

	return &pb.Empty{}, nil
}

// Exec executes a command in a sandbox.
func (s *SandboxServiceServer) Exec(ctx context.Context, req *pb.ExecRequest) (*pb.ExecResult, error) {
	if req.SandboxId == "" {
		return nil, status.Error(codes.InvalidArgument, "sandbox_id is required")
	}
	if req.Command == "" {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	// Convert request
	execReq := &types.ExecRequest{
		Command: req.Command,
		Stdin:   req.Stdin,
		Env:     req.Env,
		WorkDir: req.Workdir,
	}
	if req.Timeout != nil {
		execReq.Timeout = req.Timeout.AsDuration()
	}

	// Execute command
	result, err := s.runtime.Exec(ctx, req.SandboxId, execReq)
	if err != nil {
		if errors.Is(err, types.ErrSandboxNotFound) {
			return nil, status.Error(codes.NotFound, "sandbox not found")
		}
		if errors.Is(err, types.ErrNotRunning) {
			return nil, status.Error(codes.FailedPrecondition, "sandbox is not running")
		}
		return nil, status.Errorf(codes.Internal, "failed to execute command: %v", err)
	}

	return &pb.ExecResult{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: int32(result.ExitCode),
		Duration: durationpb.New(result.Duration),
	}, nil
}

// ExecStream executes a command and streams output.
func (s *SandboxServiceServer) ExecStream(req *pb.ExecRequest, stream grpc.ServerStreamingServer[pb.ExecOutput]) error {
	if req.SandboxId == "" {
		return status.Error(codes.InvalidArgument, "sandbox_id is required")
	}
	if req.Command == "" {
		return status.Error(codes.InvalidArgument, "command is required")
	}

	// Create output channel
	output := make(chan []byte, 100)

	// Convert request
	execReq := &types.ExecRequest{
		Command: req.Command,
		Stdin:   req.Stdin,
		Env:     req.Env,
		WorkDir: req.Workdir,
	}
	if req.Timeout != nil {
		execReq.Timeout = req.Timeout.AsDuration()
	}

	// Start execution in goroutine
	ctx := stream.Context()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.runtime.ExecStream(ctx, req.SandboxId, execReq, output)
	}()

	// Stream output to client
	for data := range output {
		if err := stream.Send(&pb.ExecOutput{
			Type: pb.ExecOutput_OUTPUT_TYPE_STDOUT,
			Data: data,
		}); err != nil {
			return err
		}
	}

	// Wait for execution to complete
	if err := <-errCh; err != nil {
		if errors.Is(err, types.ErrSandboxNotFound) {
			return status.Error(codes.NotFound, "sandbox not found")
		}
		if errors.Is(err, types.ErrNotRunning) {
			return status.Error(codes.FailedPrecondition, "sandbox is not running")
		}
		return status.Errorf(codes.Internal, "failed to execute command: %v", err)
	}

	return nil
}

// ============================================
// CodebaseServiceServer Implementation
// ============================================

// CodebaseServiceServer implements the CodebaseService gRPC interface.
type CodebaseServiceServer struct {
	pb.UnimplementedCodebaseServiceServer
	manager *codebase.Manager
}

// NewCodebaseServiceServer creates a new CodebaseServiceServer.
func NewCodebaseServiceServer(manager *codebase.Manager) *CodebaseServiceServer {
	return &CodebaseServiceServer{
		manager: manager,
	}
}

// CreateCodebase creates a new codebase.
func (s *CodebaseServiceServer) CreateCodebase(ctx context.Context, req *pb.CreateCodebaseRequest) (*pb.Codebase, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.OwnerId == "" {
		return nil, status.Error(codes.InvalidArgument, "owner_id is required")
	}

	cb, err := s.manager.CreateCodebase(ctx, &types.CreateCodebaseRequest{
		Name:    req.Name,
		OwnerID: req.OwnerId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create codebase: %v", err)
	}

	return codebaseToProto(cb), nil
}

// GetCodebase retrieves information about a codebase.
func (s *CodebaseServiceServer) GetCodebase(ctx context.Context, req *pb.GetCodebaseRequest) (*pb.Codebase, error) {
	if req.CodebaseId == "" {
		return nil, status.Error(codes.InvalidArgument, "codebase_id is required")
	}

	cb, err := s.manager.GetCodebase(ctx, req.CodebaseId)
	if err != nil {
		if errors.Is(err, types.ErrCodebaseNotFound) {
			return nil, status.Error(codes.NotFound, "codebase not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get codebase: %v", err)
	}

	return codebaseToProto(cb), nil
}

// ListCodebases lists all codebases for an owner.
func (s *CodebaseServiceServer) ListCodebases(ctx context.Context, req *pb.ListCodebasesRequest) (*pb.ListCodebasesResponse, error) {
	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 100 // Default page size
	}

	// TODO: Implement proper pagination with page_token
	codebases, err := s.manager.ListCodebases(ctx, req.OwnerId, limit, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list codebases: %v", err)
	}

	pbCodebases := make([]*pb.Codebase, 0, len(codebases))
	for _, cb := range codebases {
		pbCodebases = append(pbCodebases, codebaseToProto(cb))
	}

	return &pb.ListCodebasesResponse{
		Codebases: pbCodebases,
	}, nil
}

// DeleteCodebase deletes a codebase.
func (s *CodebaseServiceServer) DeleteCodebase(ctx context.Context, req *pb.DeleteCodebaseRequest) (*pb.Empty, error) {
	if req.CodebaseId == "" {
		return nil, status.Error(codes.InvalidArgument, "codebase_id is required")
	}

	err := s.manager.DeleteCodebase(ctx, req.CodebaseId)
	if err != nil {
		if errors.Is(err, types.ErrCodebaseNotFound) {
			return nil, status.Error(codes.NotFound, "codebase not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete codebase: %v", err)
	}

	return &pb.Empty{}, nil
}

// UploadFiles uploads files to a codebase via streaming.
func (s *CodebaseServiceServer) UploadFiles(stream grpc.ClientStreamingServer[pb.UploadChunk, pb.UploadResult]) error {
	var metadata *pb.UploadChunk_Metadata
	var buffer bytes.Buffer
	hasher := md5.New()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		switch content := chunk.Content.(type) {
		case *pb.UploadChunk_Metadata_:
			if metadata != nil {
				return status.Error(codes.InvalidArgument, "metadata already received")
			}
			metadata = content.Metadata
			if metadata.CodebaseId == "" {
				return status.Error(codes.InvalidArgument, "codebase_id is required in metadata")
			}
			if metadata.FilePath == "" {
				return status.Error(codes.InvalidArgument, "file_path is required in metadata")
			}

		case *pb.UploadChunk_Data:
			if metadata == nil {
				return status.Error(codes.InvalidArgument, "metadata must be sent first")
			}
			buffer.Write(content.Data)
			hasher.Write(content.Data)
		}
	}

	if metadata == nil {
		return status.Error(codes.InvalidArgument, "no metadata received")
	}

	// Write file to codebase
	ctx := stream.Context()
	err := s.manager.WriteFile(ctx, metadata.CodebaseId, metadata.FilePath, bytes.NewReader(buffer.Bytes()))
	if err != nil {
		if errors.Is(err, types.ErrCodebaseNotFound) {
			return status.Error(codes.NotFound, "codebase not found")
		}
		return status.Errorf(codes.Internal, "failed to write file: %v", err)
	}

	// Send result
	return stream.SendAndClose(&pb.UploadResult{
		CodebaseId: metadata.CodebaseId,
		FilePath:   metadata.FilePath,
		Size:       int64(buffer.Len()),
		Checksum:   hex.EncodeToString(hasher.Sum(nil)),
	})
}

// DownloadFile downloads a file from a codebase.
func (s *CodebaseServiceServer) DownloadFile(req *pb.DownloadFileRequest, stream grpc.ServerStreamingServer[pb.FileChunk]) error {
	if req.CodebaseId == "" {
		return status.Error(codes.InvalidArgument, "codebase_id is required")
	}
	if req.FilePath == "" {
		return status.Error(codes.InvalidArgument, "file_path is required")
	}

	ctx := stream.Context()
	reader, err := s.manager.ReadFile(ctx, req.CodebaseId, req.FilePath)
	if err != nil {
		if errors.Is(err, types.ErrCodebaseNotFound) {
			return status.Error(codes.NotFound, "codebase not found")
		}
		return status.Errorf(codes.NotFound, "file not found: %v", err)
	}
	defer reader.Close()

	// Stream file in chunks
	chunkSize := 64 * 1024 // 64KB chunks
	buf := make([]byte, chunkSize)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&pb.FileChunk{Data: buf[:n]}); sendErr != nil {
				return sendErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read file: %v", err)
		}
	}

	return nil
}

// ListFiles lists files in a codebase directory.
func (s *CodebaseServiceServer) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	if req.CodebaseId == "" {
		return nil, status.Error(codes.InvalidArgument, "codebase_id is required")
	}

	files, err := s.manager.ListFiles(ctx, req.CodebaseId, req.Path, req.Recursive)
	if err != nil {
		if errors.Is(err, types.ErrCodebaseNotFound) {
			return nil, status.Error(codes.NotFound, "codebase not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to list files: %v", err)
	}

	pbFiles := make([]*pb.FileInfo, 0, len(files))
	for _, f := range files {
		pbFiles = append(pbFiles, &pb.FileInfo{
			Path:       f.Path,
			Name:       f.Name,
			IsDir:      f.IsDir,
			Size:       f.Size,
			ModifiedAt: timestamppb.New(f.ModifiedAt),
		})
	}

	return &pb.ListFilesResponse{
		Files: pbFiles,
	}, nil
}

// ============================================
// Type Conversion Helpers
// ============================================

// convertPatternType converts proto PatternType to internal PatternType.
func convertPatternType(pt pb.PatternType) types.PatternType {
	switch pt {
	case pb.PatternType_PATTERN_TYPE_GLOB:
		return types.PatternGlob
	case pb.PatternType_PATTERN_TYPE_DIRECTORY:
		return types.PatternDirectory
	case pb.PatternType_PATTERN_TYPE_FILE:
		return types.PatternFile
	default:
		return types.PatternGlob
	}
}

// convertPermission converts proto Permission to internal Permission.
func convertPermission(p pb.Permission) types.Permission {
	switch p {
	case pb.Permission_PERMISSION_NONE:
		return types.PermNone
	case pb.Permission_PERMISSION_VIEW:
		return types.PermView
	case pb.Permission_PERMISSION_READ:
		return types.PermRead
	case pb.Permission_PERMISSION_WRITE:
		return types.PermWrite
	default:
		return types.PermNone
	}
}

// convertProtoPatternType converts internal PatternType to proto PatternType.
func convertProtoPatternType(pt types.PatternType) pb.PatternType {
	switch pt {
	case types.PatternGlob:
		return pb.PatternType_PATTERN_TYPE_GLOB
	case types.PatternDirectory:
		return pb.PatternType_PATTERN_TYPE_DIRECTORY
	case types.PatternFile:
		return pb.PatternType_PATTERN_TYPE_FILE
	default:
		return pb.PatternType_PATTERN_TYPE_UNSPECIFIED
	}
}

// convertProtoPermission converts internal Permission to proto Permission.
func convertProtoPermission(p types.Permission) pb.Permission {
	switch p {
	case types.PermNone:
		return pb.Permission_PERMISSION_NONE
	case types.PermView:
		return pb.Permission_PERMISSION_VIEW
	case types.PermRead:
		return pb.Permission_PERMISSION_READ
	case types.PermWrite:
		return pb.Permission_PERMISSION_WRITE
	default:
		return pb.Permission_PERMISSION_UNSPECIFIED
	}
}

// convertSandboxStatus converts internal SandboxStatus to proto SandboxStatus.
func convertSandboxStatus(s types.SandboxStatus) pb.SandboxStatus {
	switch s {
	case types.StatusPending:
		return pb.SandboxStatus_SANDBOX_STATUS_PENDING
	case types.StatusRunning:
		return pb.SandboxStatus_SANDBOX_STATUS_RUNNING
	case types.StatusStopped:
		return pb.SandboxStatus_SANDBOX_STATUS_STOPPED
	case types.StatusError:
		return pb.SandboxStatus_SANDBOX_STATUS_ERROR
	default:
		return pb.SandboxStatus_SANDBOX_STATUS_UNSPECIFIED
	}
}

// sandboxToProto converts internal Sandbox to proto Sandbox.
func sandboxToProto(sb *types.Sandbox) *pb.Sandbox {
	if sb == nil {
		return nil
	}

	pbSandbox := &pb.Sandbox{
		Id:         sb.ID,
		CodebaseId: sb.CodebaseID,
		Status:     convertSandboxStatus(sb.Status),
		Labels:     sb.Labels,
		CreatedAt:  timestamppb.New(sb.CreatedAt),
	}

	// Convert permissions
	for _, p := range sb.Permissions {
		pbSandbox.Permissions = append(pbSandbox.Permissions, &pb.PermissionRule{
			Pattern:    p.Pattern,
			Type:       convertProtoPatternType(p.Type),
			Permission: convertProtoPermission(p.Permission),
			Priority:   int32(p.Priority),
		})
	}

	// Set optional timestamps
	if sb.StartedAt != nil {
		pbSandbox.StartedAt = timestamppb.New(*sb.StartedAt)
	}
	if sb.StoppedAt != nil {
		pbSandbox.StoppedAt = timestamppb.New(*sb.StoppedAt)
	}
	if sb.ExpiresAt != nil {
		pbSandbox.ExpiresAt = timestamppb.New(*sb.ExpiresAt)
	}

	return pbSandbox
}

// codebaseToProto converts internal Codebase to proto Codebase.
func codebaseToProto(cb *types.Codebase) *pb.Codebase {
	if cb == nil {
		return nil
	}

	return &pb.Codebase{
		Id:        cb.ID,
		Name:      cb.Name,
		OwnerId:   cb.OwnerID,
		Size:      cb.Size,
		FileCount: int32(cb.FileCount),
		CreatedAt: timestamppb.New(cb.CreatedAt),
		UpdatedAt: timestamppb.New(cb.UpdatedAt),
	}
}
