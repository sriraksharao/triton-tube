//

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"tritontube/internal/proto"
)

type StorageServer struct {
	proto.UnimplementedVideoContentServiceServer
	baseDir string
}

func NewStorageServer(baseDir string) *StorageServer {
	return &StorageServer{baseDir: baseDir}
}

func (s *StorageServer) Write(ctx context.Context, req *proto.WriteRequest) (*proto.WriteResponse, error) {
	dir := filepath.Join(s.baseDir, req.VideoId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create video directory: %w", err)
	}

	path := filepath.Join(dir, req.Filename)
	err := os.WriteFile(path, req.Data, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &proto.WriteResponse{}, nil
}

func (s *StorageServer) Read(ctx context.Context, req *proto.ReadRequest) (*proto.ReadResponse, error) {
	path := filepath.Join(s.baseDir, req.VideoId, req.Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return &proto.ReadResponse{Data: data}, nil
}

func (s *StorageServer) ListKeys(ctx context.Context, _ *proto.ListKeysRequest) (*proto.ListKeysResponse, error) {
	var keys []string

	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || path == s.baseDir {
			return nil
		}

		relPath, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		keys = append(keys, relPath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return &proto.ListKeysResponse{Keys: keys}, nil
}

func (s *StorageServer) Delete(ctx context.Context, req *proto.DeleteRequest) (*proto.DeleteResponse, error) {
	path := filepath.Join(s.baseDir, req.VideoId, req.Filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	// Clean up empty video directory
	videoDir := filepath.Join(s.baseDir, req.VideoId)
	files, err := os.ReadDir(videoDir)
	if err == nil && len(files) == 0 {
		_ = os.Remove(videoDir)
	}

	return &proto.DeleteResponse{}, nil
}
