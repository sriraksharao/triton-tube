// Lab 7: Implement a local filesystem video content service

package web

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// FSVideoContentService implements VideoContentService using the local filesystem.
type FSVideoContentService struct {
	rootDir string
}

// Uncomment the following line to ensure FSVideoContentService implements VideoContentService
var _ VideoContentService = (*FSVideoContentService)(nil)

func NewFSVideoContentService(rootDir string) (*FSVideoContentService, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create root directory %q: %w", rootDir, err)
	}
	return &FSVideoContentService{rootDir: rootDir}, nil
}

func (fs *FSVideoContentService) Read(videoId string, filename string) ([]byte, error) {
	filePath := filepath.Join(fs.rootDir, videoId, filename)
	return os.ReadFile(filePath)

}

func (fs *FSVideoContentService) Write(videoId string, filename string, data []byte) error {
	dirPath := filepath.Join(fs.rootDir, videoId)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Printf("error creating directory: %v", err)
		return err
	}
	filePath := filepath.Join(dirPath, filename)
	return os.WriteFile(filePath, data, 0644)
}

// func create
