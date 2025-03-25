package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	pb "interface/temp/proto" // Import generated gRPC files
)

// Server struct implementing PeerService
type Server struct {
	pb.UnimplementedPeerServiceServer
}

// ListPeers - Returns a list of active peers
func (s *Server) ListPeers(ctx context.Context, req *pb.PeerRequest) (*pb.PeerResponse, error) {
	peers := []string{"peer1", "peer2", "peer69"}
	return &pb.PeerResponse{Peers: peers}, nil
}

// DownloadFile - Simulates downloading a file
func (s *Server) DownloadFile(ctx context.Context, req *pb.DownloadRequest) (*pb.DownloadResponse, error) {
	log.Printf("Downloading file: %s", req.FileName)
	return &pb.DownloadResponse{FileName: req.FileName, Status: "Download started"}, nil
}

func main() {
	// Start gRPC Server
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPeerServiceServer(grpcServer, &Server{})

	log.Println("Server is running on port 50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
