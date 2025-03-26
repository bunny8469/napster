package main

import (
	"context"
	"flag"
	"log"
	"net"
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


// Peer registration with chunk-based storage and load balancing
func (s *CentralServer) RegisterPeer(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	availablePeers := []string{}
	for peer := range s.peerStatus {
		if s.peerStatus[peer] {
			availablePeers = append(availablePeers, peer)
		}
	}

	// Distribute chunks among available peers
	for _, filePath := range req.FilePaths {
		if _, exists := s.chunkIndex[filePath]; !exists {
			s.chunkIndex[filePath] = []string{}
		}
		
		// Store full file on multiple peers
		for len(s.chunkIndex[filePath]) < s.replicationFactor && len(availablePeers) > 0 {
			assignedPeer := availablePeers[len(s.chunkIndex[filePath]) % len(availablePeers)]
			if !contains(s.chunkIndex[filePath], assignedPeer) {
				s.chunkIndex[filePath] = append(s.chunkIndex[filePath], assignedPeer)
			}
		}
		
	}

	s.peerStatus[req.PeerAddress] = true

	if len(req.FilePaths) == 0  {
		log.Printf("Peer %s registered and available for chunk storage.", req.PeerAddress)
	}

	// Print chunk distribution details
	for chunk, peers := range s.chunkIndex {
		log.Printf("Chunk: %s is held by peers: %v", chunk, peers)
	}

	return &pb.RegisterResponse{Success: true, Message: "Chunks registered with redundancy"}, nil
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

// Search for a chunk and return all available peers
func (s *CentralServer) SearchChunk(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peers, found := s.chunkIndex[req.ChunkID]
	if !found || len(peers) == 0 {
		return &pb.SearchResponse{PeerAddresses: []string{}}, nil
	}

	return &pb.SearchResponse{PeerAddresses: peers}, nil // Return all peers storing the chunk
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
		time.Sleep(10 * time.Second) // Check every 10 seconds
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
