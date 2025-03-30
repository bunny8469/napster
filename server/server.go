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

func randomString(n int) string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// --- Modified RegisterPeer ---
//
// For each file in req.FilePaths, the server renames the file by appending a random suffix,
// computes its chunk metadata (virtually chunking the file in a temporary directory),
// creates a torrent file (stored in ./torrents) that now includes ArtistName and CreatedAt fields,
// and sends the renamed file name back to the client in the RegisterResponse.
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

	var renamedFile string // Assume one file per registration for simplicity.
	for _, filePath := range req.FilePaths {
		if _, exists := s.chunkIndex[filePath]; !exists {
			s.chunkIndex[filePath] = []string{}
		}
		// Distribute the file to available peers.
		for len(s.chunkIndex[filePath]) < s.replicationFactor && len(availablePeers) > 0 {
			assignedPeer := availablePeers[len(s.chunkIndex[filePath])%len(availablePeers)]
			if !contains(s.chunkIndex[filePath], assignedPeer) {
				s.chunkIndex[filePath] = append(s.chunkIndex[filePath], assignedPeer)
			}
		}
		log.Printf("File: %s is now held by peers: %v", filePath, s.chunkIndex[filePath])

		// --- Begin Torrent Generation Flow ---
		// Rename the file with a random suffix.
		newFileName := fmt.Sprintf("%s_%s%s",
			strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
			randomString(6),
			filepath.Ext(filePath))
		newFilePath := filepath.Join(filepath.Dir(filePath), newFileName)
		err := os.Rename(filePath, newFilePath)
		if err != nil {
			log.Printf("Failed to rename file %s: %v", filePath, err)
			continue
		}
		renamedFile = newFileName

		// The server creates temporary chunks solely to compute metadata.
		tempChunksDir := "./temp_chunks"
		torrentsDir := "./torrents"
		os.MkdirAll(tempChunksDir, os.ModePerm)
		os.MkdirAll(torrentsDir, os.ModePerm)
		metadata, err := chunkFile(newFilePath, tempChunksDir, availablePeers)
		if err != nil {
			log.Printf("Error chunking file %s: %v", newFilePath, err)
			continue
		}
		// Set additional metadata fields.
		metadata.ArtistName = "Unknown Artist"
		metadata.CreatedAt = time.Now().Format(time.RFC3339)
		_, err = generateTorrentFile(metadata, torrentsDir)
		if err != nil {
			log.Printf("Error generating torrent for file %s: %v", newFilePath, err)
			continue
		}
		// Remove temporary chunks.
		os.RemoveAll(tempChunksDir)
		log.Printf("Torrent generated for file %s (renamed to %s)", filePath, newFileName)
		// --- End Torrent Generation Flow ---
	}

	return &pb.RegisterResponse{
		Success:     true,
		Message:     "File registered with redundancy and torrent generated",
		RenamedFile: renamedFile,
	}, nil
}

//
// --- New Functions for Chunking and Torrent File Generation ---
//

// ChunkSize is fixed at 64KB.
const ChunkSize = 65536

// TorrentMetadata holds metadata for a file's chunks along with artist info and timestamp.
type TorrentMetadata struct {
	FileName       string         `json:"file_name"`
	FileSize       int64          `json:"file_size"`
	ChunkSize      int            `json:"chunk_size"`
	Checksum       string         `json:"checksum"`        // Full file checksum
	ChunkChecksums map[int]string `json:"chunk_checksums"` // Mapping: chunk number -> checksum
	Peers          []string       `json:"peers"`
	ArtistName     string         `json:"artist_name"`     // New: Artist name
	CreatedAt      string         `json:"created_at"`      // New: Creation timestamp
}

// computeDataChecksum returns the SHA-256 checksum for data.
func computeDataChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// chunkFile virtually splits the file into chunks, computes checksums, and writes temporary chunk files.
func chunkFile(filePath, outputDir string, peers []string) (*TorrentMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	metadata := &TorrentMetadata{
		FileName:       filepath.Base(filePath),
		FileSize:       info.Size(),
		ChunkSize:      ChunkSize,
		ChunkChecksums: make(map[int]string),
		Peers:          peers,
	}

	os.MkdirAll(outputDir, os.ModePerm)

	fullHash := sha256.New()
	buffer := make([]byte, ChunkSize)
	chunkIndex := 0
	for {
		bytesRead, readErr := file.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}
		if bytesRead == 0 {
			break
		}
		fullHash.Write(buffer[:bytesRead])
		chunkData := buffer[:bytesRead]
		checksum := computeDataChecksum(chunkData)
		metadata.ChunkChecksums[chunkIndex] = checksum

		// Write temporary chunk file.
		chunkFileName := fmt.Sprintf("%s_chunk_%d.chunk", metadata.FileName, chunkIndex)
		chunkFilePath := filepath.Join(outputDir, chunkFileName)
		err = os.WriteFile(chunkFilePath, chunkData, 0644)
		if err != nil {
			return nil, err
		}
		chunkIndex++
	}
	metadata.Checksum = hex.EncodeToString(fullHash.Sum(nil))
	return metadata, nil
}

// generateTorrentFile writes the TorrentMetadata as a JSON file to outputDir.
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

func (s *CentralServer) SearchFile(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	peers, found := s.chunkIndex[req.FileName]
	if found {
		return &pb.SearchResponse{PeerAddresses: peers}, nil
	}
	return &pb.SearchResponse{PeerAddresses: []string{}}, nil
}

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

	go centralServer.MonitorPeers()
	log.Printf("Central Server running on port %s...", *port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
