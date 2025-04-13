package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/grpc"
	pb "napster"
)

// ChunkSize is defined as 64KB.
const ChunkSize = 65536

// PeerServer implements the PeerService for serving file requests and health checks.
type PeerServer struct {
	pb.UnimplementedPeerServiceServer
	peerAddress string
	files       map[string]string // fileName -> filePath
}

// RequestFile reads and returns the requested file's data.
func (p *PeerServer) RequestFile(ctx context.Context, req *pb.FileRequest) (*pb.FileResponse, error) {
	filePath, exists := p.files[req.FileName]
	if !exists {
		return nil, fmt.Errorf("file not found")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file")
	}
	return &pb.FileResponse{FileData: data}, nil
}

// HealthCheck returns alive status.
func (p *PeerServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Alive: true}, nil
}

// startPeerServer starts a local gRPC server so that other peers can communicate with this peer.
func startPeerServer(peerAddress string, files map[string]string) {
	listener, err := net.Listen("tcp", peerAddress)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterPeerServiceServer(server, &PeerServer{peerAddress: peerAddress, files: files})
	log.Printf("Peer listening on %s...", peerAddress)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

// registerPeerWithServer lets this peer register with the central server.
func registerPeerWithServer(serverAddr, peerAddress string) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	client := pb.NewCentralServerClient(conn)
	_, err = client.RegisterPeer(context.Background(), &pb.RegisterRequest{
		PeerAddress: peerAddress,
		FilePaths:   []string{}, // No file paths are provided during registration.
	})
	if err != nil {
		log.Fatalf("Failed to register peer: %v", err)
	}
	fmt.Println("Peer registered successfully with the server!")
}

// uploadFile streams the file to the central server chunk by chunk,
// receives renamed file name from server, then saves chunks locally with the new name.
func uploadFile(serverAddr, localFilePath string) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewCentralServerClient(conn)

	// Start the client-streaming RPC
	stream, err := client.UploadFile(context.Background())
	if err != nil {
		log.Fatalf("Error starting upload: %v", err)
	}

	// Open the original file
	file, err := os.Open(localFilePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	buffer := make([]byte, ChunkSize)
	originalBaseName := filepath.Base(localFilePath)

	// Send chunks to server
	chunkIndex := 0
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Fatalf("Error reading file: %v", err)
		}
		if bytesRead == 0 {
			break
		}

		req := &pb.FileChunk{
			FileName:  originalBaseName,
			ChunkData: buffer[:bytesRead],
		}
		if err := stream.Send(req); err != nil {
			log.Fatalf("Error sending chunk: %v", err)
		}
		chunkIndex++
	}

	// Close stream and receive final response
	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}

	fmt.Printf("Upload completed.\nRenamed file name: %s\nTorrent file: %s\n", res.RenamedFileName, res.TorrentFileName)

	renamedFilePath := filepath.Join(filepath.Dir(localFilePath), res.RenamedFileName)
	if localFilePath != renamedFilePath {
		err = os.Rename(localFilePath, renamedFilePath)
		if err != nil {
			log.Fatalf("Failed to rename local file: %v", err)
		}
		fmt.Printf("Local file renamed to: %s\n", renamedFilePath)
	}

	file, err = os.Open(renamedFilePath)
	if err != nil {
		log.Fatalf("Failed to reopen renamed file: %v", err)
	}
	defer file.Close()
	chunksDir := "./chunks"
	os.MkdirAll(chunksDir, os.ModePerm)

	chunkIndex = 0
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Fatalf("Error reading renamed file: %v", err)
		}
		if bytesRead == 0 {
			break
		}
		chunkFileName := fmt.Sprintf("%s_chunk_%d.chunk", res.RenamedFileName, chunkIndex)
		chunkFilePath := filepath.Join(chunksDir, chunkFileName)
		err = os.WriteFile(chunkFilePath, buffer[:bytesRead], 0644)
		if err != nil {
			log.Fatalf("Error writing chunk file %s: %v", chunkFileName, err)
		}
		chunkIndex++
	}

	fmt.Printf("Chunks stored locally as: %s_chunk_*.chunk in ./chunks/\n", res.RenamedFileName)
}

// searchFileOnServer queries the central server for peers storing the given file.
func searchFileOnServer(serverAddr string) {
	fmt.Print("Enter song or artist name to search: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	query := scanner.Text()

	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()
	client := pb.NewCentralServerClient(conn)

	res, err := client.SearchFile(context.Background(), &pb.SearchRequest{Query: query})
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	if len(res.Results) == 0 {
		fmt.Println("No results found.")
		return
	}

	fmt.Println("Matching songs:")
	for _, song := range res.Results {
		fmt.Printf("- Title: %s\n  Artist: %s\n  Created: %s\n  Peers: %v\n\n",
			song.FileName, song.ArtistName, song.CreatedAt, song.PeerAddresses)
	}
}
// mergeChunks merges locally stored chunk files into a single file.
func mergeChunks(fileName, chunksDir, outputFile string) error {
	pattern := filepath.Join(chunksDir, fmt.Sprintf("%s_chunk_*.chunk", fileName))
	chunkFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("error getting chunk files: %v", err)
	}
	if len(chunkFiles) == 0 {
		return fmt.Errorf("no chunk files found for %s", fileName)
	}
	sort.Slice(chunkFiles, func(i, j int) bool {
		getIndex := func(file string) int {
			base := filepath.Base(file)
			parts := strings.Split(base, "_")
			if len(parts) < 3 {
				return 0
			}
			indexPart := strings.TrimSuffix(parts[len(parts)-1], ".chunk")
			var index int
			fmt.Sscanf(indexPart, "%d", &index)
			return index
		}
		return getIndex(chunkFiles[i]) < getIndex(chunkFiles[j])
	})
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer outFile.Close()
	for _, chunkFile := range chunkFiles {
		inFile, err := os.Open(chunkFile)
		if err != nil {
			return fmt.Errorf("error opening chunk file %s: %v", chunkFile, err)
		}
		_, err = io.Copy(outFile, inFile)
		inFile.Close()
		if err != nil {
			return fmt.Errorf("error copying chunk file %s: %v", chunkFile, err)
		}
	}
	return nil
}

// verifyFileChecksum calculates the SHA-256 checksum of a file and compares it with the expected checksum.
func verifyFileChecksum(filePath, expectedChecksum string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("error opening file for checksum verification: %v", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, fmt.Errorf("error reading file for checksum verification: %v", err)
	}
	computedChecksum := hex.EncodeToString(hash.Sum(nil))
	return computedChecksum == expectedChecksum, nil
}

// rebuildFile merges local chunk files and verifies integrity using the provided checksum.
func rebuildFile(fileName, chunksDir, outputFile, expectedChecksum string) error {
	if err := mergeChunks(fileName, chunksDir, outputFile); err != nil {
		return fmt.Errorf("error merging chunks: %v", err)
	}
	valid, err := verifyFileChecksum(outputFile, expectedChecksum)
	if err != nil {
		return fmt.Errorf("error verifying checksum: %v", err)
	}
	if !valid {
		return fmt.Errorf("checksum verification failed; file may be corrupted")
	}
	return nil
}

// rebuildFileOption reads a torrent file for the expected checksum and rebuilds the file using local chunks.
func rebuildFileOption() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Enter the base file name to rebuild (e.g., musix_randomXYu.mp3): ")
	scanner.Scan()
	baseFileName := scanner.Text()
	fmt.Print("Enter the output file path for the rebuilt file (e.g., reconstructed_musix.mp3): ")
	scanner.Scan()
	outputFile := scanner.Text()

	torrentFileName := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName)) + ".torrent"
	torrentFilePath := filepath.Join("./torrents", torrentFileName)
	data, err := os.ReadFile(torrentFilePath)
	if err != nil {
		fmt.Println("Error reading torrent file:", err)
		return
	}
	// Local struct matching the torrent metadata.
	type TorrentMetadata struct {
		FileName       string            `json:"file_name"`
		FileSize       int64             `json:"file_size"`
		ChunkSize      int               `json:"chunk_size"`
		Checksum       string            `json:"checksum"`
		ChunkChecksums map[string]string `json:"chunk_checksums"`
		Peers          []string          `json:"peers"`
		ArtistName     string            `json:"artist_name"`
		CreatedAt      string            `json:"created_at"`
	}
	var metadata TorrentMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		fmt.Println("Error parsing torrent file:", err)
		return
	}
	expectedChecksum := metadata.Checksum
	err = rebuildFile(baseFileName, "./chunks", outputFile, expectedChecksum)
	if err != nil {
		fmt.Println("Error rebuilding file:", err)
	} else {
		fmt.Println("File rebuilt successfully and checksum verified!")
	}
}

func main() {
	serverAddr := flag.String("server", "localhost:50051", "Central server address")
	peerPort := flag.String("port", "50054", "Port for peer server")
	flag.Parse()

	peerAddress := fmt.Sprintf("localhost:%s", *peerPort)
	// Register this peer with the central server.
	registerPeerWithServer(*serverAddr, peerAddress)

	files := make(map[string]string)
	// Start a peer server concurrently.
	go startPeerServer(peerAddress, files)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\nChoose an option:")
		fmt.Println("1. Upload File")
		fmt.Println("2. Search File")
		fmt.Println("3. Rebuild File")
		fmt.Println("4. Exit")
		fmt.Print("Enter choice: ")
		scanner.Scan()
		choice := strings.TrimSpace(scanner.Text())
		switch choice {
		case "1":
			fmt.Print("Enter full path of the file to upload: ")
			scanner.Scan()
			localFilePath := scanner.Text()
			uploadFile(*serverAddr, localFilePath)
		case "2":
			searchFileOnServer(*serverAddr)
		case "3":
			rebuildFileOption()
		case "4":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid option. Please try again.")
		}
	}
}
