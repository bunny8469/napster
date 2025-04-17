package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// "encoding/json"
	// "path/filepath"
	"log"
	// "os"
	// "strings"

	pb "napster"     // For SongInfo
	"napster/client" // gRPC client

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	grpcClient	 	*client.PeerServer
	peerAddress		string
	httpPort		string
	ctx        		context.Context
}

func NewApp(address string, httpPort string) *App {
	_, indexingClient := client.GetIndexingClient("localhost:50051")
	
	clt := &client.PeerServer{
		Client: indexingClient,
		PeerAddress: address,
	}

	parts := strings.Split(address, ":")
	port := parts[len(parts)-1] // "50051"

	client.DOWNLOAD_PATH = client.DOWNLOAD_PATH + "_" + port 
	client.TORRENTS_DIR = client.DOWNLOAD_PATH + "/torrents"
	client.CACHE_DIR = client.DOWNLOAD_PATH + "/cache"

	go func() {
		if err := client.StartPeerServer(clt); err != nil {
			log.Printf("Peer server failed to start: %v", err)
		}
	}()

	return &App{
		grpcClient: clt,
		peerAddress: address,
		httpPort: httpPort,
	}
}

// ============ Wails Lifecycle Hooks ============

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("Wails app started")
}

func (a *App) domReady(ctx context.Context) {
	log.Println("DOM ready")
}

func (a *App) beforeClose(ctx context.Context) bool {
	log.Println("App is closing...")
	return false
}

func (a *App) shutdown(ctx context.Context) {
	log.Println("App shutdown")
}

// ============ App Methods Bound to Frontend ============

func (a *App) GetHttpPort() string {
	return a.httpPort
}
func (a *App) GetPeerAddress() string { 
	return a.peerAddress;
}

func (a *App) GetTorrents() []client.TorrentInfo {
	torrents, err := a.grpcClient.GetLocalTorrents()
	if err != nil {
		log.Printf("Failed to get torrents: %v", err)
		return nil
	}
	return torrents
}

func (a *App) GetLibraryTorrents() []client.TorrentInfo {
	torrents, err := a.grpcClient.GetLibraryTorrents()
	if err != nil {
		log.Printf("Failed to get library torrents: %v", err)
		return nil
	}
	return torrents
}


func (a *App) SearchSongs(query string) []*pb.SongInfo {
	results, err := a.grpcClient.SearchFile(query)
	if err != nil {
		log.Printf("SearchSongs error: %v", err)
		return nil
	}
	return results
}

func (a *App) UploadFile(filePath string, artist string) string {
	result, err := a.grpcClient.UploadFile(filePath, a.grpcClient.PeerAddress, func(metadata client.TorrentMetadata) {
		runtime.EventsEmit(a.ctx, "upload-status", metadata)
	})
	if err != nil {
		log.Printf("UploadFile error: %v", err)
		return fmt.Sprintf("Upload failed: %v", err)
	}
	return result
}

func (a *App) DownloadFile(query string) string {

	result := a.grpcClient.DownloadFile(query, func(statusObject client.DownloadStatus) {
		runtime.EventsEmit(a.ctx, "download-status", statusObject)
	}, func(metadata client.TorrentMetadata) {
		runtime.EventsEmit(a.ctx, "download-queue", metadata)
	})
	return result
}

func (a *App) SelectFileAndUpload() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a Song",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Audio Files",
				Pattern:     "*.mp3;*.wav;*.flac",
			},
		},
	})
	if err != nil || path == "" {
		return "", fmt.Errorf("file not selected or cancelled")
	}

	result := a.UploadFile(path, a.peerAddress)
	return result, nil
}

// GetMusicFilePath returns the absolute path to an MP3 file
func (a *App) GetMusicFilePath(filename string) string {
    dir, err := os.Getwd()
    if err != nil {
        return ""
    }
    
    absolutePath := filepath.Join(dir, filename)
    
    if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
        return ""
    }
    
    return "file://" + absolutePath
}