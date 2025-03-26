package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	pb "napster"
)

type CentralServer struct {
	pb.UnimplementedCentralServerServer
	mu                sync.Mutex
	chunkIndex        map[string][]string // chunkID -> list of peerAddresses
	peerStatus        map[string]bool     // Peer health status
	replicationFactor int                 // Number of replicas per chunk
}

func NewCentralServer() *CentralServer {
	return &CentralServer{
		chunkIndex:        make(map[string][]string),
		peerStatus:        make(map[string]bool),
		replicationFactor: 3, // Store each chunk on at least 3 peers
	}
}

// Compute SHA-256 checksum of a file
func computeChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Generate a random string for unique metadata filenames
func randomString(n int) string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Peer registration with chunk-based storage and load balancing
func (s *CentralServer) RegisterPeer(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()


	s.peerStatus[req.PeerAddress] = true

	if len(req.FilePaths) == 0 {
		log.Printf("Peer %s registered and available for chunk storage.", req.PeerAddress)
		return &pb.RegisterResponse{Success: true, Message: "Peer registered successfully with the server"}, nil
	}
	
	availablePeers := []string{}
	for peer := range s.peerStatus {
		if s.peerStatus[peer] {
			availablePeers = append(availablePeers, peer)
		}
	}
	// If fewer than 3 peers are available, return an error to the client
	if len(availablePeers) < 3 {
		log.Printf("Not enough peers available. At least 3 required, found %d", len(availablePeers))
		return &pb.RegisterResponse{Success: false, Message: "Not enough peers available to store the file"}, nil
	}

	// Distribute chunks among available peers
	for _, filePath := range req.FilePaths {
		if _, exists := s.chunkIndex[filePath]; !exists {
			s.chunkIndex[filePath] = []string{}
		}

		// Store full file on multiple peers
		for len(s.chunkIndex[filePath]) < s.replicationFactor && len(availablePeers) > 0 {
			assignedPeer := availablePeers[len(s.chunkIndex[filePath])%len(availablePeers)]
			if !contains(s.chunkIndex[filePath], assignedPeer) {
				s.chunkIndex[filePath] = append(s.chunkIndex[filePath], assignedPeer)
			}
		}
		// Compute checksum
		checksum, err := computeChecksum(filePath)
		if err != nil {
			log.Printf("Failed to compute checksum for %s: %v", filePath, err)
			continue
		}

		// Save metadata to a unique JSON file
		metadata := map[string]interface{}{
			"file_path": filePath,
			"peers":     s.chunkIndex[filePath],
			"checksum":  checksum,
		}
		metadataFile := fmt.Sprintf("metadata_%s_%s.json", filePath, randomString(6))
		fileData, _ := json.MarshalIndent(metadata, "", "  ")
		ioutil.WriteFile(metadataFile, fileData, 0644)

		log.Printf("Metadata stored in %s", metadataFile)
	}

	// s.peerStatus[req.PeerAddress] = true

	// if len(req.FilePaths) == 0 {
	// 	log.Printf("Peer %s registered and available for chunk storage.", req.PeerAddress)
	// }

	// Print chunk distribution details
	for _, filePath := range req.FilePaths {
		log.Printf("File: %s is now held by peers: %v", filePath, s.chunkIndex[filePath])
	}

	return &pb.RegisterResponse{Success: true, Message: "File registered with redundancy"}, nil
}

// Helper function to check if a slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func (s *CentralServer) SearchFile(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var peers []string

	peers, found := s.chunkIndex[req.FileName]
	if found {
		return &pb.SearchResponse{PeerAddresses: peers}, nil
	}

	if len(peers) == 0 {
		return &pb.SearchResponse{PeerAddresses: []string{}}, nil
	}

	return &pb.SearchResponse{PeerAddresses: peers}, nil
}

// Periodically check peer health and update status
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
		time.Sleep(5 * time.Second) // Check every 10 seconds
	}
}


// Check if a peer is responsive
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

	// Start monitoring peer health
	go centralServer.MonitorPeers()

	log.Printf("Central Server running on port %s...", *port)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
