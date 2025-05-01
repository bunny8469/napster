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
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/stathat/consistent"

	pb "napster"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var debug_mode = false;
var TORRENTS_DIR = "./torrents";

// CentralServer holds the peer status and a mapping from original file path to peers.
type CentralServer struct {
	pb.UnimplementedCentralServerServer
	mu                	sync.Mutex
	fileMap			  	map[string]string
	// chunkIndex       map[string][]string // mapping: original filePath -> list of peerAddresses
	peerStatus        	map[string]bool     // Peer health status
	replicationFactor 	int                 // Number of replicas per file
	ContributorHashring *consistent.Consistent
	cNodes				map[string]pb.PeerServiceClient
}

func NewCentralServer() *CentralServer {
	return &CentralServer{
		fileMap:        make(map[string]string),
		peerStatus:        make(map[string]bool),
		replicationFactor: 3,
		ContributorHashring: consistent.New(),
		cNodes: make(map[string]pb.PeerServiceClient),
	}
}

func (s *CentralServer) RegisterContributor(ctx context.Context, req *pb.ContributorRequest) (*pb.GenResponse, error) {

	if _, exists := s.cNodes[req.ContriAddr]; exists {
		return &pb.GenResponse{Status: 204}, nil
	}

	if (len(s.cNodes) > s.replicationFactor * 5) {
		return &pb.GenResponse{Status: 400}, nil
	}

	conn, err := grpc.NewClient(req.ContriAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pb.GenResponse{}, err
	}
	
	client := pb.NewPeerServiceClient(conn)
	s.cNodes[req.ContriAddr] = client
	s.ContributorHashring.Add(req.ContriAddr)

	log.Printf("Added Contributor %s", req.ContriAddr);
	
	return &pb.GenResponse{Status: 200}, nil
}

// --- UploadFile ---
// Client-streaming RPC where the client sends file chunks. The server appends a random string
// to the file name on the first chunk, computes per-chunk and overall checksums, and generates a torrent.
func (s *CentralServer) UploadFile(stream pb.CentralServer_UploadFileServer) error {
	var metadata TorrentMetadata
	metadata.ChunkChecksums = make(map[int]string)
	sha256Hasher := sha256.New()
	chunkIndex := 0

    // var newFileName string
	// Receive streamed file chunks.
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error receiving chunk: %v", err)
			return err
		}
		if chunkIndex == 0 {
			metadata.FileName = req.FileName
			metadata.ChunkSize = ChunkSize
			metadata.ArtistName = req.AlbumArtist
			metadata.Peers = []string{req.PeerAddress}
			metadata.Duration = int64(req.Duration)
			
			if req.FileName == "" || req.PeerAddress == "" {
				return stream.SendAndClose(&pb.UploadResponse{
					Status:         301,
					Message:        "Incomplete Chunk 0, missing filename or peer address",
				})
			} else if strings.Contains(req.FileName, "_chunk") {
				return stream.SendAndClose(&pb.UploadResponse{
					Status:         401,
					Message:        "File name contains '_chunk' which is not allowed.",
				})
			}
		}
		data := req.ChunkData

		// Update overall file checksum.
		sha256Hasher.Write(data)

		// Compute and store the checksum for this chunk.
		chunkChecksum := computeDataChecksum(data)
		metadata.ChunkChecksums[chunkIndex] = chunkChecksum
		// log.Print(chunkIndex, chunkChecksum)

		chunkIndex++
	}

	// Finalize overall checksum and update metadata.
	metadata.Checksum = hex.EncodeToString(sha256Hasher.Sum(nil))
	metadata.CreatedAt = time.Now().Format(time.RFC3339)
	metadata.FileSize = int64(chunkIndex * ChunkSize) // Assuming all chunks are full size.
	
	// metadata.Peers = --- During loadbalancing, this will be filled with the list of peers.

	// Generate torrent file.
	torrentsDir := TORRENTS_DIR
	os.MkdirAll(torrentsDir, os.ModePerm)
	torrentFileName, err := generateTorrentFile(&metadata, torrentsDir)
	if err != nil {
		log.Printf("Error generating torrent file: %v", err)
		return err
	}

	s.fileMap[metadata.FileName] = torrentFileName

	go func() {
		for _ = range min(3, len(s.cNodes)) {
			clientAddr, _ := s.ContributorHashring.Get(metadata.CreatedAt)
			resp, err := s.cNodes[clientAddr].DownloadThisFile(context.Background(), &pb.SearchRequest{
				Query: metadata.FileName,
			})
			if err != nil {
				log.Printf("%s %v", resp, err)
			}
		}
	}()

	// Respond to the client with the torrent file info.
	return stream.SendAndClose(&pb.UploadResponse{
		Status:         200,
		TorrentFileName: torrentFileName,
		// RenamedFileName: newFileName,
		Message:         "Torrent file generated successfully",
	})
}

// --- Functions for Chunking and Torrent File Generation ---

// ChunkSize is fixed at 64KB. (2^6 * 2^10)
const ChunkSize = 1 << 18

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
	Status 		   string 		  `json:"status"`
}

// computeDataChecksum returns the SHA-256 checksum for the given data.
func computeDataChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

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

func (s *CentralServer) EnableSeeding(ctx context.Context, req *pb.SeedingRequest) (*pb.GenResponse, error) {
	torrent_file := filepath.Join(TORRENTS_DIR, s.fileMap[req.FileName])

	data, err := os.ReadFile(torrent_file)
	if err != nil {
		return &pb.GenResponse{}, err
	}

	var metadata TorrentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return &pb.GenResponse{}, err
	}

	newPeer := req.ClientAddr
	found := false
	for _, peer := range metadata.Peers {
		if peer == newPeer {
			found = true
			break
		}
	}
	if !found {
		metadata.Peers = append(metadata.Peers, newPeer)
	}

	// Marshal back and overwrite the .torrent file
	updatedData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal updated torrent: %v", err)
	} else {
		err = os.WriteFile(torrent_file, updatedData, 0644)
		if err != nil {
			log.Printf("Failed to write updated torrent: %v", err)
		}
	}

	return &pb.GenResponse{}, nil
}

func (s *CentralServer) StopSeeding(ctx context.Context, req *pb.SeedingRequest) (*pb.GenResponse, error) {
	torrent_file := filepath.Join(TORRENTS_DIR, s.fileMap[req.FileName])

	data, err := os.ReadFile(torrent_file)
	if err != nil {
		return &pb.GenResponse{}, err
	}

	var metadata TorrentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return &pb.GenResponse{}, err
	}

	newPeer := req.ClientAddr
	filteredPeers := make([]string, 0, len(metadata.Peers))
	for _, peer := range metadata.Peers {
		if peer != newPeer {
			filteredPeers = append(filteredPeers, peer)
		}
	}
	metadata.Peers = filteredPeers

	// Marshal back and overwrite the .torrent file
	updatedData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal updated torrent: %v", err)
	} else {
		err = os.WriteFile(torrent_file, updatedData, 0644)
		if err != nil {
			log.Printf("Failed to write updated torrent: %v", err)
		}
	}

	return &pb.GenResponse{}, nil
}

// SearchFile returns a list of peer addresses that host the requested file.
func (s *CentralServer) SearchFile(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	
	files, err := os.ReadDir(TORRENTS_DIR)
	if err != nil {
		return nil, err
	}

	var results []*pb.SongInfo
	query := strings.ToLower(req.Query)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".torrent") {
			data, err := os.ReadFile(filepath.Join(TORRENTS_DIR, file.Name()))
			if err != nil {
				continue
			}
			var metadata TorrentMetadata
			if err := json.Unmarshal(data, &metadata); err != nil {
				continue
			}
			songName := strings.ToLower(metadata.FileName)
			artist := strings.ToLower(metadata.ArtistName)

			if fuzzy.MatchNormalized(query, songName) || fuzzy.MatchNormalized(query, artist){
				results = append(results, &pb.SongInfo{
					FileName:     metadata.FileName,
					ArtistName:   metadata.ArtistName,
					PeerAddresses: metadata.Peers,
					CreatedAt:    metadata.CreatedAt,
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
				if debug_mode {
					log.Printf("Peer %s is offline", peer)
				}
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

func (s *CentralServer) GetTorrent(ctx context.Context, req *pb.SearchRequest) (*pb.TorrentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	torrentPath, exists := s.fileMap[req.Query]
	if !exists {
		return &pb.TorrentResponse{ Status: 404 }, nil
	}
	
	torrentPath = filepath.Join(TORRENTS_DIR, torrentPath)
	log.Print(torrentPath)

	content, err := os.ReadFile(torrentPath)
	if err != nil {
		log.Printf("Error reading torrent file %s: %v", torrentPath, err)
		return &pb.TorrentResponse{ Status: 500 }, nil
	}

	return &pb.TorrentResponse{
		Status:   200,
		Filename: filepath.Base(torrentPath),
		Content:  content,
	}, nil
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
