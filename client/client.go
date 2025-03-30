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

// --- Existing PeerServer and Client Functions ---

type PeerServer struct {
	pb.UnimplementedPeerServiceServer
	peerAddress string
	files       map[string]string // fileName -> filePath
}

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

func (p *PeerServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Alive: true}, nil
}

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

// createLocalChunks splits a local file into 64KB chunks and saves them in outputDir.
func createLocalChunks(filePath, outputDir string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file for chunking: %v", err)
	}
	defer file.Close()

	os.MkdirAll(outputDir, os.ModePerm)
	buffer := make([]byte, 65536)
	chunkIndex := 0
	for {
		bytesRead, readErr := file.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		if bytesRead == 0 {
			break
		}
		chunkFileName := fmt.Sprintf("%s_chunk_%d.chunk", filepath.Base(filePath), chunkIndex)
		chunkFilePath := filepath.Join(outputDir, chunkFileName)
		err := os.WriteFile(chunkFilePath, buffer[:bytesRead], 0644)
		if err != nil {
			return err
		}
		chunkIndex++
	}
	log.Printf("Local chunks created for file %s in %s", filePath, outputDir)
	return nil
}

// registerFilesWithServer sends file paths to the server and receives the renamed file name.
// Then, the client renames its local file accordingly and creates local chunks.
// registerFilesWithServer sends file paths to the server and receives the renamed file name.
// Then, the client renames its local file accordingly (if it still exists) and creates local chunks.
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
	renamedFile := res.RenamedFile
	fmt.Println("Files registered successfully!")
	fmt.Println("Renamed file on server:", renamedFile)

	// Pick the first file from the map.
	var originalFilePath string
	for _, fp := range files {
		originalFilePath = fp
		break
	}
	localDir := filepath.Dir(originalFilePath)
	newLocalFilePath := filepath.Join(localDir, renamedFile)

	// Check if the original file exists.
	if _, err := os.Stat(originalFilePath); err == nil {
		// If it exists, rename it locally.
		err = os.Rename(originalFilePath, newLocalFilePath)
		if err != nil {
			log.Fatalf("Error renaming local file: %v", err)
		}
		fmt.Println("Local file renamed to:", newLocalFilePath)
	} else {
		// If not, assume the file is already renamed.
		fmt.Println("Original file not found, assuming already renamed.")
	}

	// Create local chunks from the renamed file.
	err = createLocalChunks(newLocalFilePath, "./chunks")
	if err != nil {
		log.Fatalf("Error creating local chunks: %v", err)
	}
}

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
		FilePaths:   []string{},
	})
	if err != nil {
		log.Fatalf("Failed to register peer: %v", err)
	}
	fmt.Println("Peer registered successfully with the server!")
}

// --- New Rebuild (Merge) Functions ---

// mergeChunks merges chunk files (with names like <base>_chunk_X.chunk) from chunksDir into outputFile.
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

// verifyFileChecksum computes the SHA-256 checksum of filePath and compares it with expectedChecksum.
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

// rebuildFile merges local chunks and verifies integrity using the expected checksum.
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

// RebuildFileOption reads the torrent file to get the expected checksum and rebuilds the file.
func rebuildFileOption() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Enter the base file name to rebuild (e.g., musix_randomXYu.mp3): ")
	scanner.Scan()
	baseFileName := scanner.Text()
	fmt.Print("Enter the output file path for the rebuilt file (e.g., reconstructed_musix.mp3): ")
	scanner.Scan()
	outputFile := scanner.Text()

	// Derive torrent file path from the base file name.
	torrentFileName := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName)) + ".torrent"
	torrentFilePath := filepath.Join("./torrents", torrentFileName)
	data, err := os.ReadFile(torrentFilePath)
	if err != nil {
		fmt.Println("Error reading torrent file:", err)
		return
	}
	// Local struct for torrent metadata including artist and created_at.
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

// --- Main Function and Menu ---

func main() {
	serverAddr := flag.String("server", "localhost:50051", "Central server address")
	peerPort := flag.String("port", "50054", "Port for peer server")
	flag.Parse()

	peerAddress := fmt.Sprintf("localhost:%s", *peerPort)
	// Register the peer.
	registerPeerWithServer(*serverAddr, peerAddress)

	files := make(map[string]string)
	go startPeerServer(peerAddress, files)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\nChoose an option: ")
		fmt.Println("1. Register Files")
		fmt.Println("2. Search File")
		fmt.Println("3. Rebuild File")
		fmt.Println("4. Exit")
		fmt.Print("Enter choice: ")
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
			rebuildFileOption()
		case "4":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid option. Please try again.")
		}
	}
}
