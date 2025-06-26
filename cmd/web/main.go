package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"tritontube/internal/proto"
	"tritontube/internal/web"

	"google.golang.org/grpc"
)

// printUsage prints the usage information for the application
func printUsage() {
	fmt.Println("Usage: ./program [OPTIONS] METADATA_TYPE METADATA_OPTIONS CONTENT_TYPE CONTENT_OPTIONS")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  METADATA_TYPE         Metadata service type (sqlite, etcd)")
	fmt.Println("  METADATA_OPTIONS      Options for metadata service (e.g., db path)")
	fmt.Println("  CONTENT_TYPE          Content service type (fs, nw)")
	fmt.Println("  CONTENT_OPTIONS       Options for content service (e.g., base dir, network addresses)")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Example: ./program sqlite db.db fs /path/to/videos")
}

func main() {
	// Define flags
	port := flag.Int("port", 8080, "Port number for the web server")
	host := flag.String("host", "localhost", "Host address for the web server")

	// Set custom usage message
	flag.Usage = printUsage

	// Parse flags
	flag.Parse()

	// Check if the correct number of positional arguments is provided
	if len(flag.Args()) != 4 {
		fmt.Println("Error: Incorrect number of arguments")
		printUsage()
		return
	}

	// Parse positional arguments
	metadataServiceType := flag.Arg(0)
	metadataServiceOptions := flag.Arg(1)
	contentServiceType := flag.Arg(2)
	contentServiceOptions := flag.Arg(3)

	// Validate port number (already an int from flag, check if positive)
	if *port <= 0 {
		fmt.Println("Error: Invalid port number:", *port)
		printUsage()
		return
	}

	// Construct metadata service
	var metadataService web.VideoMetadataService
	fmt.Println("Creating metadata service of type", metadataServiceType, "with options", metadataServiceOptions)
	// TODO: Implement metadata service creation logic
	var err error
	metadataService, err = web.NewSQLiteVideoMetadataService(metadataServiceOptions)

	// Construct content service
	var contentService web.VideoContentService
	fmt.Println("Creating content service of type", contentServiceType, "with options", contentServiceOptions)
	// TODO: Implement content service creation logic
	// var err2 error

	// contentService, err = web.NewFSVideoContentService(contentServiceOptions)
	switch contentServiceType {
	case "fs":
		contentService, err = web.NewFSVideoContentService(contentServiceOptions)
		if err != nil {
			fmt.Println("Error initializing FS service:", err)
			os.Exit(1)
		}
	case "nw":
		// Parse comma-separated addresses
		addresses := splitAddresses(contentServiceOptions)
		// contentService, err = web.NewNetworkVideoContentService(addresses[0], addresses[1:], metadataService)
		contentService, err = web.NewNetworkVideoContentService(addresses[1:])
		if err != nil {
			fmt.Println("Error initializing Network service:", err)
			os.Exit(1)
		}
		nwService, ok := contentService.(*web.NetworkVideoContentService)
		if !ok {
			fmt.Println("Error: contentService is not a NetworkVideoContentService")
			os.Exit(1)
		}
		go func() {
			adminAddr := addresses[0] // Use the first address as admin gRPC address
			lis, err := net.Listen("tcp", adminAddr)
			if err != nil {
				fmt.Println("Failed to listen for admin gRPC:", err)
				os.Exit(1)
			}
			grpcServer := grpc.NewServer()
			proto.RegisterVideoContentAdminServiceServer(grpcServer, nwService)
			fmt.Println("Admin gRPC server running at", adminAddr)
			if err := grpcServer.Serve(lis); err != nil {
				fmt.Println("Admin gRPC failed:", err)
				os.Exit(1)
			}
		}()
	default:
		fmt.Println("Error: Unsupported content service type:", contentServiceType)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Println("Error initializing FS service:", err)
		os.Exit(1)
	}

	// Start the server
	server := web.NewServer(metadataService, contentService)
	listenAddr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer lis.Close()

	fmt.Println("Starting web server on", listenAddr)
	err = server.Start(lis)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
}

// lab 8
func splitAddresses(option string) []string {
	var result []string
	for _, addr := range splitAndTrim(option, ",") {
		if addr != "" {
			result = append(result, addr)
		}
	}
	return result
}

func splitAndTrim(s string, sep string) []string {
	var trimmed []string
	for _, part := range splitAndClean(s, sep) {
		trimmed = append(trimmed, part)
	}
	return trimmed
}

func splitAndClean(s, sep string) []string {
	var out []string
	for _, x := range strings.Split(s, sep) {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
