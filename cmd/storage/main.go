package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	proto "tritontube/internal/proto"
	"tritontube/internal/storage"

	// "tritontube/internal/storage"

	"google.golang.org/grpc"
)

// type storageServer struct {
// 	proto.UnimplementedVideoContentServiceServer
// 	rootDir string
// }

// // Write saves the video chunk or manifest to disk
// func (s *storageServer) Write(ctx context.Context, req *proto.WriteRequest) (*proto.WriteResponse, error) {
// 	dir := filepath.Join(s.rootDir, req.VideoId)
// 	if err := os.MkdirAll(dir, 0755); err != nil {
// 		return nil, fmt.Errorf("failed to create dir: %w", err)
// 	}
// 	path := filepath.Join(dir, req.Filename)
// 	err := os.WriteFile(path, req.Data, 0644)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to write file: %w", err)
// 	}
// 	return &proto.WriteResponse{}, nil
// }

// // Read loads a video chunk or manifest from disk
// func (s *storageServer) Read(ctx context.Context, req *proto.ReadRequest) (*proto.ReadResponse, error) {
// 	path := filepath.Join(s.rootDir, req.VideoId, req.Filename)
// 	data, err := os.ReadFile(path)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read file: %w", err)
// 	}
// 	return &proto.ReadResponse{Data: data}, nil
// }

func main() {
	host := flag.String("host", "localhost", "Host address for the server")
	port := flag.Int("port", 8090, "Port number for the server")
	flag.Parse()

	// Validate arguments
	if *port <= 0 {
		panic("Error: Port number must be positive")
	}

	if flag.NArg() < 1 {
		fmt.Println("Usage: storage [OPTIONS] <baseDir>")
		fmt.Println("Error: Base directory argument is required")
		return
	}
	baseDir := flag.Arg(0)

	fmt.Println("Starting storage server...")
	fmt.Printf("Host: %s\n", *host)
	fmt.Printf("Port: %d\n", *port)
	fmt.Printf("Base Directory: %s\n", baseDir)

	//lab8
	address := fmt.Sprintf("%s:%d", *host, *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", address, err)
	}

	// grpcServer := grpc.NewServer()
	// server := &storage.StorageServer{RootDir: baseDir}
	// // pb.RegisterVideoContentServiceServer(grpcServer, server)
	// proto.RegisterVideoContentServiceServer(grpcServer, server)

	grpcServer := grpc.NewServer()
	storageServer := storage.NewStorageServer(baseDir)
	// adminServer := &storage.AdminServer{RootDir: baseDir}

	proto.RegisterVideoContentServiceServer(grpcServer, storageServer)
	// proto.RegisterVideoContentAdminServiceServer(grpcServer, adminServer)

	log.Printf("Storage server listening on %s, storing to %s", address, baseDir)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	// panic("Lab 8: not implemented")
}
