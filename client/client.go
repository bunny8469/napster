package client

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
	"time"
	"github.com/tcolgate/mp3"
	pb "napster"

	"github.com/dhowden/tag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var debug_mode = true
const CHUNKS_DIR = "./chunks"				// Files being served to other peers

// ChunkSize is defined as 256KB.
const ChunkSize = 1 << 18

type TorrentMetadata struct {
	FileName       string         `json:"file_name"`
	FileSize       int64          `json:"file_size"`
	ChunkSize      int            `json:"chunk_size"`
	Checksum       string         `json:"checksum"`        // Full file checksum
	ChunkChecksums map[int]string `json:"chunk_checksums"` // Mapping: chunk number -> checksum
	Peers          []string       `json:"peers"`
	ArtistName     string         `json:"artist_name"` // Artist name
	CreatedAt      string         `json:"created_at"`  // Creation timestamp
	Duration       int64          `json:"duration"`    
	Status 		   string 		  `json:"status"`
}


// PeerServer implements the PeerService for serving file requests and health checks.
type PeerServer struct {
	pb.UnimplementedPeerServiceServer
	PeerAddress 	string
	Client			pb.CentralServerClient
	EventEmitter 	func (eventName string, returnObject any)
}

// HealthCheck returns alive status.
func (p *PeerServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Alive: true}, nil
}

// startPeerServer starts a local gRPC server so that other peers can communicate with this peer.
func StartPeerServer(peerServer *PeerServer) error {
	listener, err := net.Listen("tcp", peerServer.PeerAddress)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
		return err
	}
	
	server := grpc.NewServer()
	pb.RegisterPeerServiceServer(server, peerServer)

	log.Printf("Peer listening on %s...", peerServer.PeerAddress)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
		return err
	}

	return nil
}

func GetIndexingClient(serverAddr string) (*grpc.ClientConn, pb.CentralServerClient) {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}

	client := pb.NewCentralServerClient(conn)

	log.Print("Connected to server!")
	return conn, client
}

func computeDataChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Get duration (in seconds) of MP3 file
func getMP3Duration(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	d := mp3.NewDecoder(file)
	var f mp3.Frame
	skipped := 0
	var duration time.Duration

	for {
		if err := d.Decode(&f, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		duration += f.Duration()
	}

	return int(duration.Seconds()), nil
}

// uploadFile streams the file to the central server chunk by chunk,
// receives renamed file name from server, then saves chunks locally with the new name.
func (p *PeerServer) UploadFile(localFilePath string, peerAddress string) (string, error) {

	file, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		metadata = nil
	}

	// Reset file pointer to the beginning
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("failed to reset file pointer: %v", err)
	}
	
	albumArtist := "Unknown Artist" 
	if metadata != nil {
		albumArtist = metadata.AlbumArtist()
	}
	duration, err := getMP3Duration(localFilePath)
	if err != nil {
		log.Printf("Failed to calculate duration: %v", err)
		duration = 0
	}
	fmt.Printf("Duration: %d\n", duration)

	// Start the client-streaming RPC
	stream, err := p.Client.UploadFile(context.Background())
	if err != nil {
		log.Printf("Error starting upload: %v", err)
		return "", err
	}

	buffer := make([]byte, ChunkSize)
	originalBaseName := filepath.Base(localFilePath)

	// Send chunks to server
	chunkIndex := 0
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("Error reading file: %v", err)
			return "", err
		}
		if bytesRead == 0 {
			break
		}

		req := &pb.FileChunk{
			FileName:  originalBaseName,
			ChunkData: buffer[:bytesRead],
		}
		log.Println(chunkIndex, computeDataChecksum(buffer[:bytesRead]))

		if chunkIndex == 0 {
			req.PeerAddress = peerAddress
			req.AlbumArtist = albumArtist
			req.Duration = int32(duration)
		}
		if err := stream.Send(req); err != nil {
			log.Printf("Error sending chunk: %v", err)
			return "", err
		}
		chunkIndex++
	}

	// Close stream and receive final response
	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("Upload failed: %v", err)
		return "", err
	}
	if res.Status != 200 {
		log.Printf("Server rejected upload: %s", res.Message)
		return "", err
	}
	log.Printf("Upload successful: %s", res.Message)
	file.Close()

	fmt.Printf("Upload completed.\nTorrent file: %s\n", res.TorrentFileName)

	file, err = os.Open(localFilePath)
	if err != nil {
		log.Printf("Failed to reopen file: %v", err)
		return "", err
	}
	defer file.Close()

	chunksDir := CHUNKS_DIR
	os.MkdirAll(chunksDir, os.ModePerm)
	
	buffer = make([]byte, ChunkSize)

	chunkIndex = 0
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("Error reading renamed file: %v", err)
			return "", err
		}
		if bytesRead == 0 {
			break
		}
		chunkFileName := fmt.Sprintf("%s_chunk_%d", originalBaseName, chunkIndex)
		chunkFilePath := filepath.Join(chunksDir, chunkFileName)

		err = os.WriteFile(chunkFilePath, buffer[:bytesRead], 0644)
		if err != nil {
			log.Printf("Error writing chunk file %s: %v", chunkFileName, err)
			return "", err
		}

		chunkIndex++
	}

	torrent_path := GetTorrent(p.Client, originalBaseName)
	if torrent_path == "" {
		return "", err
	}
	
	var metadata_ TorrentMetadata
	metadata_ = ParseTorrent(torrent_path)
	if metadata_.FileName == "" {
		return "", err
	}
	
	p.EventEmitter("upload-status", metadata_)
	mergeChunks(originalBaseName, CHUNKS_DIR, filepath.Join(DOWNLOAD_PATH, originalBaseName))

	fmt.Printf("Chunks stored locally as: %s_chunk_* in ./chunks/\n", originalBaseName)
	return "", nil
}

func (p *PeerServer) DownloadThisFile(ctx context.Context, req *pb.SearchRequest) (*pb.GenResponse, error) {
	p.DownloadFile(req.Query)
	return &pb.GenResponse{Status: 200}, nil
}

// searchFileOnServer queries the central server for peers storing the given file.
func searchFileOnServer(client pb.CentralServerClient) {
	fmt.Print("Enter song or artist name to search: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	query := scanner.Text()

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
		fmt.Printf("- Title: %s\n  Artist: %s\n  Created: %s\n  Peers: %d\n\n",
			song.FileName, song.ArtistName, song.CreatedAt, len(song.PeerAddresses))
	}
}

// mergeChunks merges locally stored chunk files into a single file.
func mergeChunks(fileName, chunksDir, outputFile string) error {
	pattern := filepath.Join(chunksDir, fmt.Sprintf("%s_chunk_*", fileName))
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
			parts := strings.Split(base, "_chunk_")
			if len(parts) < 2 {
				return 0
			}
			indexPart := parts[1]
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

	var metadata TorrentMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		fmt.Println("Error parsing torrent file:", err)
		return
	}
	expectedChecksum := metadata.Checksum
	err = rebuildFile(baseFileName, CHUNKS_DIR, outputFile, expectedChecksum)
	if err != nil {
		fmt.Println("Error rebuilding file:", err)
	} else {
		fmt.Println("File rebuilt successfully and checksum verified!")
	}
}

func (peer *PeerServer) RequestChunk(ctx context.Context, req *pb.ChunkRequest) (*pb.ChunkResponse, error) {
	chunkPath := filepath.Join(CHUNKS_DIR, req.ChunkName)

	// log.Println(req.ChunkName)
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &pb.ChunkResponse{
				Status:    404,
				ChunkData: nil,
			}, nil
		}

		return nil, fmt.Errorf("failed to read chunk: %v", err)
	}

	return &pb.ChunkResponse{
		Status:    200,
		ChunkData: data,
	}, nil
}


// SearchFile queries the central server for files matching the query.
func (c *PeerServer) SearchFile(query string) ([]*pb.SongInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	res, err := c.Client.SearchFile(ctx, &pb.SearchRequest{Query: query})
	if err != nil {
		log.Printf("search error: %v", err)
		return nil, fmt.Errorf("search error: %v", err)
	}
	return res.Results, nil
}

var peerAddress string;

func main() {
	serverAddr := flag.String("server", "localhost:50051", "Central server address")
	peerPort := flag.String("port", "50054", "Port for peer server")
	flag.Parse()

	peerAddress = fmt.Sprintf("localhost:%s", *peerPort)
	// Register this peer with the central server.
	 
	conn, indexingClient := GetIndexingClient(*serverAddr)
	defer conn.Close()

	// Start a peer server concurrently.
	// go StartPeerServer(peerAddress, indexingClient)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("\nChoose an option:")
		fmt.Println("1. Upload File")
		fmt.Println("2. Search File")
		fmt.Println("3. Rebuild File")
		fmt.Println("4. Download File")
		fmt.Println("0. Exit")
		fmt.Print("Enter choice: ")
		scanner.Scan()
		choice := strings.TrimSpace(scanner.Text())
		switch choice {
		case "1":
			fmt.Print("Enter full path of the file to upload: ")
			scanner.Scan()
			// localFilePath := scanner.Text()
			// uploadFile(indexingClient, localFilePath, peerAddress)
		case "2":
			searchFileOnServer(indexingClient)
		case "3":
			rebuildFileOption()
		case "4":
			fmt.Print("Enter file name: ")
			scanner.Scan()
			// file := scanner.Text()
			// downloadFile(indexingClient, file)
		case "0":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid option. Please try again.")
		}
	}
}
