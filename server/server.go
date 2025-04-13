package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"sort"
	"github.com/lithammer/fuzzysearch/fuzzy"

	"google.golang.org/grpc"
	pb "napster"
)

// CentralServer holds the peer status and a mapping from original file path to peers.
type CentralServer struct {
	pb.UnimplementedCentralServerServer
	mu                sync.Mutex
	chunkIndex        map[string][]string // mapping: original filePath -> list of peerAddresses
	peerStatus        map[string]bool     // Peer health status
	replicationFactor int                 // Number of replicas per file
}

func NewCentralServer() *CentralServer {
	return &CentralServer{
		chunkIndex:        make(map[string][]string),
		peerStatus:        make(map[string]bool),
		replicationFactor: 3,
	}
}

// randomString generates a random alphanumeric string of length n.
func randomString(n int) string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// --- RegisterPeer ---
// For each file in req.FilePaths, the server renames the file by appending a random suffix,
// computes its chunk metadata by virtually chunking the file in a temporary directory,
// generates a torrent file (stored in ./torrents) with ArtistName and CreatedAt, and sends the new name.
func (s *CentralServer) RegisterPeer(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.peerStatus[req.PeerAddress] = true

	// If no file paths provided, just register the peer.
	if len(req.FilePaths) == 0 {
		log.Printf("Peer %s registered and available for chunk storage.", req.PeerAddress)
		return &pb.RegisterResponse{
			Success: true,
			Message: "Peer registered successfully with the server",
		}, nil
	}

	availablePeers := []string{}
	for peer := range s.peerStatus {
		if s.peerStatus[peer] {
			availablePeers = append(availablePeers, peer)
		}
	}
	if len(availablePeers) < 3 {
		log.Printf("Not enough peers available. At least 3 required, found %d", len(availablePeers))
		return &pb.RegisterResponse{
			Success: false,
			Message: "Not enough peers available to store the file",
		}, nil
	}

	return &pb.RegisterResponse{
		Success:     true,
		Message:     "File registered with redundancy",
	}, nil
}

// --- UploadFile ---
// Client-streaming RPC where the client sends file chunks. The server appends a random string
// to the file name on the first chunk, computes per-chunk and overall checksums, and generates a torrent.
func (s *CentralServer) UploadFile(stream pb.CentralServer_UploadFileServer) error {
	var metadata TorrentMetadata
	metadata.ChunkChecksums = make(map[int]string)
	sha256Hasher := sha256.New()
	chunkIndex := 0
    var newFileName string
	// Receive streamed file chunks.
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// End of stream; finish processing.
			break
		}
		if err != nil {
			log.Printf("Error receiving chunk: %v", err)
			return err
		}
		// On the first chunk, rename the file by appending a random string.
		if chunkIndex == 0 {
			baseName := strings.TrimSuffix(req.FileName, filepath.Ext(req.FileName))
			ext := filepath.Ext(req.FileName)
			newFileName = fmt.Sprintf("%s_%s%s", baseName, randomString(6), ext)
			metadata.FileName = newFileName
			metadata.ChunkSize = ChunkSize
		}
		data := req.ChunkData

		// Update overall file checksum.
		sha256Hasher.Write(data)

		// Compute and store the checksum for this chunk.
		chunkChecksum := computeDataChecksum(data)
		metadata.ChunkChecksums[chunkIndex] = chunkChecksum

		chunkIndex++
	}

	// Finalize overall checksum and update metadata.
	metadata.Checksum = hex.EncodeToString(sha256Hasher.Sum(nil))
	metadata.CreatedAt = time.Now().Format(time.RFC3339)
	metadata.ArtistName = "Unknown Artist"
	metadata.Duration = int64(len(metadata.ChunkChecksums)) // Using number of chunks.
	metadata.FileSize = int64(chunkIndex * ChunkSize) // Assuming all chunks are full size.
	
	// metadata.Peers = --- During loadbalancing, this will be filled with the list of peers.


	// Generate torrent file.
	torrentsDir := "../torrents"
	os.MkdirAll(torrentsDir, os.ModePerm)
	torrentFileName, err := generateTorrentFile(&metadata, torrentsDir)
	if err != nil {
		log.Printf("Error generating torrent file: %v", err)
		return err
	}

	// Respond to the client with the torrent file info.
	return stream.SendAndClose(&pb.UploadResponse{
		Success:         true,
		TorrentFileName: torrentFileName,
		RenamedFileName: newFileName,
		Message:         "Torrent file generated successfully",
	})
}

// --- Functions for Chunking and Torrent File Generation ---

// ChunkSize is fixed at 64KB.
const ChunkSize = 65536

// TorrentMetadata holds metadata for a file's chunks along with artist info and timestamps.
type TorrentMetadata struct {
	FileName       string         `json:"file_name"`
	FileSize       int64          `json:"file_size"`
	ChunkSize      int            `json:"chunk_size"`
	Checksum       string         `json:"checksum"`        // Full file checksum
	ChunkChecksums map[int]string `json:"chunk_checksums"` // Mapping: chunk number -> checksum
	Peers          []string       `json:"peers"`
	ArtistName     string         `json:"artist_name"` // Artist name
	CreatedAt      string         `json:"created_at"`  // Creation timestamp
	Duration       int64          `json:"duration"`    // e.g., number of chunks
}

// computeDataChecksum returns the SHA-256 checksum for the given data.
func computeDataChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// // chunkFile virtually splits the file into chunks, computes per-chunk checksums,
// // writes temporary chunk files, and returns the torrent metadata.
// func chunkFile(filePath, outputDir string, peers []string) (*TorrentMetadata, error) {
// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer file.Close()

// 	info, err := file.Stat()
// 	if err != nil {
// 		return nil, err
// 	}
// 	fmt.Printf("---------------------------------File size: %d bytes\n", info.Size())

// 	metadata := &TorrentMetadata{
// 		FileName:       filepath.Base(filePath),
// 		FileSize:       info.Size(),
// 		ChunkSize:      ChunkSize,
// 		ChunkChecksums: make(map[int]string),
// 		Peers:          peers,
// 	}

// 	os.MkdirAll(outputDir, os.ModePerm)

// 	fullHash := sha256.New()
// 	buffer := make([]byte, ChunkSize)
// 	chunkIndex := 0
// 	for {
// 		bytesRead, readErr := file.Read(buffer)
// 		if readErr != nil && readErr != io.EOF {
// 			return nil, readErr
// 		}
// 		if bytesRead == 0 {
// 			break
// 		}
// 		fullHash.Write(buffer[:bytesRead])
// 		chunkData := buffer[:bytesRead]
// 		checksum := computeDataChecksum(chunkData)
// 		metadata.ChunkChecksums[chunkIndex] = checksum

// 		// Write temporary chunk file.
// 		chunkFileName := fmt.Sprintf("%s_chunk_%d.chunk", metadata.FileName, chunkIndex)
// 		chunkFilePath := filepath.Join(outputDir, chunkFileName)
// 		err = os.WriteFile(chunkFilePath, chunkData, 0644)
// 		if err != nil {
// 			return nil, err
// 		}
// 		chunkIndex++
// 	}
// 	metadata.Checksum = hex.EncodeToString(fullHash.Sum(nil))
// 	return metadata, nil
// }

// generateTorrentFile writes the TorrentMetadata as a JSON file to outputDir and returns the file name.
func generateTorrentFile(metadata *TorrentMetadata, outputDir string) (string, error) {
	torrentFileName := fmt.Sprintf("%s.torrent", strings.TrimSuffix(metadata.FileName, filepath.Ext(metadata.FileName)))
	torrentFilePath := filepath.Join(outputDir, torrentFileName)
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", err
	}
	err = os.WriteFile(torrentFilePath, data, 0644)
	if err != nil {
		return "", err
	}
	return torrentFileName, nil
}

// SearchFile returns a list of peer addresses that host the requested file.
func (s *CentralServer) SearchFile(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	files, err := os.ReadDir("../torrents")
	if err != nil {
		return nil, err
	}

	var results []*pb.SongInfo
	query := strings.ToLower(req.Query)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".torrent") {
			data, err := os.ReadFile(filepath.Join("../torrents", file.Name()))
			if err != nil {
				continue
			}
			var metadata TorrentMetadata
			if err := json.Unmarshal(data, &metadata); err != nil {
				continue
			}
			// songName := strings.ToLower(metadata.FileName)
			// artist := strings.ToLower(metadata.ArtistName)

			// if fuzzy.MatchNormalized(query, songName) || fuzzy.MatchNormalized(query, artist){
			// 	results = append(results, &pb.SongInfo{
			// 		FileName:     metadata.FileName,
			// 		ArtistName:   metadata.ArtistName,
			// 		PeerAddresses: metadata.Peers,
			// 		CreatedAt:    metadata.CreatedAt,
			// 	})
			// }
			candidates := []string{metadata.FileName, metadata.ArtistName}
			matches := fuzzy.RankFindNormalized(query, candidates)
			sort.Sort(matches) // Most relevant first

			if len(matches) > 0 {
				results = append(results, &pb.SongInfo{
					FileName:      metadata.FileName,
					ArtistName:    metadata.ArtistName,
					PeerAddresses: metadata.Peers,
					CreatedAt:     metadata.CreatedAt,
				})
			}
		}
	}

	return &pb.SearchResponse{Results: results}, nil
}
// MonitorPeers periodically checks the health of registered peers.
func (s *CentralServer) MonitorPeers() {
	for {
		s.mu.Lock()
		for peer := range s.peerStatus {
			if !s.CheckPeerHealth(peer) {
				s.peerStatus[peer] = false
				log.Printf("Peer %s is offline", peer)
			} else {
				s.peerStatus[peer] = true
			}
		}
		s.mu.Unlock()
		time.Sleep(5 * time.Second)
	}
}

// CheckPeerHealth performs a simple gRPC health check on a peer.
func (s *CentralServer) CheckPeerHealth(peerAddr string) bool {
	conn, err := grpc.Dial(peerAddr, grpc.WithInsecure())
	if err != nil {
		return false
	}
	defer conn.Close()
	client := pb.NewPeerServiceClient(conn)
	res, err := client.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
	return err == nil && res.Alive
}

func main() {
	port := flag.String("port", "50051", "Port to run the central server")
	flag.Parse()

	listener, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	server := grpc.NewServer()
	centralServer := NewCentralServer()
	pb.RegisterCentralServerServer(server, centralServer)

	// Start monitoring peer health.
	go centralServer.MonitorPeers()
	log.Printf("Central Server running on port %s...", *port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
