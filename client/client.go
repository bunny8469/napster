package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	pb "napster"
)

// PeerServer handles incoming file requests
type PeerServer struct {
	pb.UnimplementedPeerServiceServer
	peerAddress string
	files       map[string]string // fileName -> filePath
}

// Serve files upon request
func (p *PeerServer) RequestFile(ctx context.Context, req *pb.FileRequest) (*pb.FileResponse, error) {
	filePath, exists := p.files[req.FileName]
	if !exists {
		return nil, fmt.Errorf("file not found")
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file")
	}

	return &pb.FileResponse{FileData: data}, nil
}

// Respond to health checks
func (p *PeerServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Alive: true}, nil
}


// Start peer's gRPC server
func startPeerServer(peerAddress string, files map[string]string) {
	listener, err := net.Listen("tcp", peerAddress)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	pb.RegisterPeerServiceServer(server, &PeerServer{peerAddress: peerAddress, files: files})

	log.Printf("Peer listening on %s...", peerAddress)
	server.Serve(listener)
}

func registerFilesWithServer(serverAddr, peerAddress string, files map[string]string) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewCentralServerClient(conn)

	var filePaths []string
	for _, filePath := range files {
		filePaths = append(filePaths, filePath)
	}

	res, err := client.RegisterPeer(context.Background(), &pb.RegisterRequest{
		PeerAddress: peerAddress,
		FilePaths:   filePaths,
	})

	if err != nil {
		log.Fatalf("Failed to register: %v", err)
	}

	if !res.Success {
		fmt.Println("Registration failed:", res.Message)
		return
	}

	fmt.Println("Chunks registered successfully!")
}

// Search for a file on the central server
func searchFileOnServer(serverAddr string) {
	fmt.Print("Enter file name to search: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	fileName := scanner.Text()

	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewCentralServerClient(conn)
	res, err := client.SearchFile(context.Background(), &pb.SearchRequest{FileName: fileName})
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	if len(res.PeerAddresses) == 0 {
		fmt.Println("No peers found with the file.")
	} else {
		fmt.Println("Peers with file:", res.PeerAddresses)
	}
}

func registerPeerWithServer(serverAddr, peerAddress string) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewCentralServerClient(conn)
	_, err = client.RegisterPeer(context.Background(), &pb.RegisterRequest{
		PeerAddress: peerAddress,
		FilePaths:   []string{}, // Send file paths instead of chunk IDs
	})
	if err != nil {
		log.Fatalf("Failed to register peer: %v", err)
	}
	fmt.Println("Peer registered successfully with the server!")
}

func main() {
	serverAddr := flag.String("server", "localhost:50051", "Central server address")
	peerPort := flag.String("port", "50054", "Port for peer server")
	flag.Parse()

	peerAddress := fmt.Sprintf("localhost:%s", *peerPort)

	// Register peer with server even if no files are being registered
	registerPeerWithServer(*serverAddr, peerAddress)

	files := make(map[string]string)
	go startPeerServer(peerAddress, files)

	for {
		fmt.Println("Choose an option: ")
		fmt.Println("1. Register Files")
		fmt.Println("2. Search File")
		fmt.Println("3. Exit")
		fmt.Print("Enter choice: ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			fmt.Print("Enter file directory path: ")
			scanner.Scan()
			fileDir := scanner.Text()

			err := filepath.Walk(fileDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					fileName := filepath.Base(path)
					files[fileName] = path
				}
				return nil
			})
			if err != nil {
				log.Fatalf("Error walking directory: %v", err)
			}

			registerFilesWithServer(*serverAddr, peerAddress, files)

		case "2":
			searchFileOnServer(*serverAddr)

		case "3":
			fmt.Println("Exiting...")
			return

		default:
			fmt.Println("Invalid option. Please try again.")
		}
	}
}
