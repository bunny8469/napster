package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	pb "interface/temp/proto"
)

// Client struct
type Client struct {
	grpcClient pb.PeerServiceClient
}

// Initialize gRPC client
func NewClient() *Client {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure()) // Replace with your actual gRPC server address
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	client := pb.NewPeerServiceClient(conn)
	return &Client{grpcClient: client}
}

// Fetch peer list from gRPC server
func (c *Client) GetPeers() []string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := c.grpcClient.ListPeers(ctx, &pb.PeerRequest{})
	if err != nil {
		log.Printf("Error calling gRPC: %v", err)
		return []string{"Error fetching peers"}
	}
	return resp.Peers
}

// Request file download
func (c *Client) DownloadFile(fileName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	req := &pb.DownloadRequest{FileName: fileName}
	resp, err := c.grpcClient.DownloadFile(ctx, req)
	if err != nil {
		log.Printf("Download error: %v", err)
		return "Download failed"
	}
	return fmt.Sprintf("File %s downloaded successfully!", resp.FileName)
}
