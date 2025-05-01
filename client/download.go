package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "napster"

	"github.com/stathat/consistent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var DOWNLOAD_PATH = "./downloads"			// Folder for .crdownload, downloaded music files
const LIBRARY_DIR = "../torrents"	// Folder for storing torrent files
var TORRENTS_DIR = "./downloads/torrents"	// Folder for storing torrent files
var CACHE_DIR = "./downloads/cache"		// cache for downloads, to make it resumable
var MAX_THREADS = 4							// Max. threads for multi-source downloading

func ParseTorrent(filepath string) (TorrentMetadata) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		fmt.Println("Error reading torrent file:", err)
		return TorrentMetadata{}
	}

	var metadata TorrentMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		fmt.Println("Error parsing torrent file:", err)
		return TorrentMetadata{}
	}

	return metadata
}

func GetTorrent(client pb.CentralServerClient, filename string) (string) {
	res, err := client.GetTorrent(context.Background(), &pb.SearchRequest{Query: filename})
	if err != nil {
		log.Printf("Torrent Retrival Failed: %v", err)
		return ""
	} else if res.Status != 200 {
		log.Printf("Response Status: %d", res.Status)
		return ""
	}

	os.MkdirAll(TORRENTS_DIR, os.ModePerm)

	download_path := filepath.Join(TORRENTS_DIR, res.Filename)
	err = os.WriteFile(download_path, res.Content, 0644)
	if err != nil {
		log.Printf("%s is downloaded!", res.Filename)
	}
	
	return download_path
}

func IsExisting(metadata TorrentMetadata, filename string) bool {

	filePath := filepath.Join(DOWNLOAD_PATH, filename)
	if _, err := os.Stat(filePath); err != nil {
		return false
	}
	torrentFile, err := verifyFileChecksum(filePath, metadata.Checksum)
	if err != nil {
		log.Printf("File might be corrupted, redownloading chunks... %v", err);
	}
	return torrentFile
}

func (p *PeerServer) DownloadFile(filename string) (string) {

	torrent_path := GetTorrent(p.Client, filename)
	if torrent_path == "" {
		return ""
	}
	
	metadata := ParseTorrent(torrent_path)
	if metadata.FileName == "" {
		return ""
	}

	p.EventEmitter("download-queue", metadata)

	time.Sleep(5 * time.Second)
	
	p.EventEmitter("download-status", DownloadStatus{
		Filename: filename,
		Status: "Torrent Parsed",
	})
	time.Sleep(5 * time.Second)
	
	if IsExisting(metadata, filename) {
		p.EventEmitter("download-status", DownloadStatus{
			Filename: filename,
			Status: metadata.Status,	// if seeding
		})
		log.Printf("File already exists, and verified with server.")
		return ""
	}

	p.StartDownload(metadata, p.Client, p.PeerAddress)

	return ""
}

type DownloadTask struct {
	ChunkID 	int
	ChunkName   string 
	ClientAddr 	string
	CheckSum	string
}

type ChunkCoordinator struct {
	chunkData   map[int][]byte
	chunkReady  chan int
	chunkMutex  *sync.Mutex
	hashRing	*consistent.Consistent
}

type DownloadStatus struct {
    Filename string `json:"filename"`
    Status   string `json:"status"`
}

func GetChunkName(filename string, chunkId int) string {
	return fmt.Sprintf("%s_chunk_%d", filename, chunkId)
}

// Check and load already downloaded chunks into memory
func ImportExistingChunks(metadata TorrentMetadata, chunkCoordinator *ChunkCoordinator) {

	numChunks := len(metadata.ChunkChecksums);
	log.Println(numChunks)

	for chunkID := range numChunks {
		chunkName := GetChunkName(metadata.FileName, chunkID)
		chunkPath := filepath.Join(CACHE_DIR, chunkName)

		if _, err := os.Stat(chunkPath); err == nil {
			res, err := verifyFileChecksum(chunkPath, metadata.ChunkChecksums[chunkID])
			if err != nil || !res {
				log.Printf("Checksum verification failed for chunk %d, will re-download %v", chunkID, err)
				continue
			}

			// If valid, load it into memory
			chunkData, err := os.ReadFile(chunkPath)
			if err != nil {
				log.Printf("Failed to read already downloaded chunk %d: %v", chunkID, err)
				continue
			}

			chunkCoordinator.chunkMutex.Lock()
			chunkCoordinator.chunkData[chunkID] = chunkData
			chunkCoordinator.chunkMutex.Unlock()

			// Signal that the chunk is ready for streaming
			chunkCoordinator.chunkReady <- chunkID
			log.Printf("Loaded chunk %d from disk", chunkID)
		}
	}
}

func MoveChunksToStore(filename string) error {
	chunkPrefix := filename + "_chunk_"
	srcDir := filepath.Join(DOWNLOAD_PATH, "cache")
	destDir := CHUNKS_DIR

	os.Mkdir(destDir, os.ModePerm)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("reading source dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, chunkPrefix) {
			srcPath := filepath.Join(srcDir, name)
			destPath := filepath.Join(destDir, name)
			err := os.Rename(srcPath, destPath)
			if err != nil {
				return fmt.Errorf("moving chunk %s: %w", name, err)
			}
		}
	}

	return nil
}

var torrentStatus = struct {
	sync.RWMutex
    status map[string]string
}{status: make(map[string]string)}


func (p *PeerServer) StartDownload(metadata TorrentMetadata, indexingClient pb.CentralServerClient, peerAddr string) {
	numChunks := len(metadata.ChunkChecksums)
	// peerCount := len(metadata.Peers)

	tasks := make(chan DownloadTask, numChunks)
	
	chunkCoordinator := &ChunkCoordinator{
		chunkData: make(map[int][]byte),
		chunkReady: make(chan int, numChunks),
		chunkMutex: &sync.Mutex{},
		hashRing: consistent.New(),
	}

	p.EventEmitter("download-status", DownloadStatus{
		Filename: metadata.FileName,
		Status: "Downloading",
	})
	torrentStatus.RLock()
	changeTorrentStatus(getFileName(metadata.FileName), "Downloading")
	torrentStatus.RUnlock()

	time.Sleep(5 * time.Second)

	ImportExistingChunks(metadata, chunkCoordinator)

	// Launch worker goroutines
	for i := 0; i < MAX_THREADS; i++ {
		go DownloadWorker(i, tasks, tasks, chunkCoordinator)
	}

	// chunkCoordinator.hashRing.NumberOfReplicas = 100
	for _, peer := range metadata.Peers {
		if peer != peerAddr {
			chunkCoordinator.hashRing.Add(peer)
		}
	}

	// Assign tasks round-robin
	for chunkID := range numChunks {
		if _, downloaded := chunkCoordinator.chunkData[chunkID]; downloaded {
			continue
		}
		
		// peerIndex := chunkID % peerCount
		// clientAddr := metadata.Peers[peerIndex]

		clientAddr, _ := chunkCoordinator.hashRing.Get(GetChunkName(metadata.FileName, chunkID))
		
		tasks <- DownloadTask{
			ChunkID: chunkID, 
			ChunkName: GetChunkName(metadata.FileName, chunkID), 
			ClientAddr: clientAddr, 
			CheckSum: metadata.ChunkChecksums[chunkID],
		}
	}

	// writes to a file parallely as chunks are received
	StreamWriter(metadata, chunkCoordinator)
	close(tasks)
	
	MoveChunksToStore(metadata.FileName)

	p.EventEmitter("download-status", DownloadStatus{
		Filename: metadata.FileName,
		Status: "Downloaded",
	})

	torrentStatus.RLock()
	changeTorrentStatus(getFileName(metadata.FileName), "Downloaded")
	torrentStatus.RUnlock()
	
	_, err := indexingClient.EnableSeeding(context.Background(), &pb.SeedingRequest{FileName: metadata.FileName, ClientAddr: peerAddr})
	if err != nil {
		log.Printf("Seeding Failed: %v", err)
		return
	}

	time.Sleep(5 * time.Second)

	p.EventEmitter("download-status", DownloadStatus{
		Filename: metadata.FileName,
		Status: "Seeding",
	})

	torrentStatus.RLock()
	changeTorrentStatus(getFileName(metadata.FileName), "Seeding")
	torrentStatus.RUnlock()
}

func RetryRequestChunk(task DownloadTask, tasks chan<- DownloadTask, chunkCoordinator *ChunkCoordinator) {
	chunkCoordinator.hashRing.Remove(task.ClientAddr)
	clientAddr, _ := chunkCoordinator.hashRing.Get(task.ChunkName)

	tasks <- DownloadTask{
		ChunkID: task.ChunkID,
		ChunkName: task.ChunkName,
		ClientAddr: clientAddr,
		CheckSum: task.CheckSum,
	}
}

func getFileName(chunkName string) string {
	parts := strings.Split(chunkName, "_chunk_")
	if len(parts) < 2 {
		return ""
	}
	indexPart := parts[0]
	return indexPart
}

func changeTorrentStatus(filename string, status string) {
	torrentStatus.RLock()
	defer torrentStatus.RUnlock()

	torrentStatus.status[filename] = status

	torrentFile := fmt.Sprintf("%s.torrent", strings.TrimSuffix(filename, filepath.Ext(filename)))

	data, err := os.ReadFile(torrentFile)
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

	metadata.Status = status

	updatedData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling updated metadata:", err)
		return
	}

	err = os.WriteFile(torrentFile, updatedData, 0644)
	if err != nil {
		fmt.Println("Error writing updated torrent file:", err)
	}
}

func getTorrentStatus(filename string) string {
	torrentStatus.RLock()
	defer torrentStatus.RUnlock()
	
	return torrentStatus.status[filename]
}

func (p *PeerServer) PauseDownload(filename string) {
	changeTorrentStatus(filename, "Paused")
}

func DownloadWorker(workerID int, tasks <-chan DownloadTask, sendTasks chan<- DownloadTask, chunkCoordinator *ChunkCoordinator) {
	for task := range tasks {

		log.Printf("%d worker %d", workerID, task.ChunkID)

		status := getTorrentStatus(getFileName(task.ChunkName))

		if status == "Paused" {
			if debug_mode {
				log.Printf("Pausing Download")
			}
 			return
		}

		// Connect to peer via gRPC
		conn, err := grpc.NewClient(task.ClientAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Worker %d: Failed to connect to peer %s: %v", workerID, task.ClientAddr, err)
			continue
		}
		client := pb.NewPeerServiceClient(conn)

		// Request chunk
		resp, err := client.RequestChunk(context.Background(), &pb.ChunkRequest{ChunkName: task.ChunkName})
		conn.Close()

		if err != nil || resp.Status != 200 {
			log.Printf("Worker %d: Failed to download chunk %s from %s, retrying...", workerID, task.ChunkName, task.ClientAddr)
			RetryRequestChunk(task, sendTasks, chunkCoordinator)
			continue
		}

		os.Mkdir(CACHE_DIR, os.ModePerm)
		chunkPath := filepath.Join(CACHE_DIR, task.ChunkName)
		err = os.WriteFile(chunkPath, resp.ChunkData, 0644)
		if err != nil {
			log.Printf("Worker %d: Failed to write chunk %s: %v", workerID, task.ChunkName, err)
		} else if debug_mode {
			log.Printf("Worker %d: Successfully downloaded chunk %s from %s", workerID, task.ChunkName, task.ClientAddr)
		}

		res, err := verifyFileChecksum(chunkPath, task.CheckSum)
		if err != nil || !res {
			if debug_mode {
				log.Printf("Worker %d: Failed to verify checksum %s: %v", workerID, task.ChunkName, err)
			}
			return 
		}

		// Save to chunkData and signal
		chunkCoordinator.chunkMutex.Lock()
		chunkCoordinator.chunkData[task.ChunkID] = resp.ChunkData
		chunkCoordinator.chunkMutex.Unlock()
		chunkCoordinator.chunkReady <- task.ChunkID

		if debug_mode {
			log.Printf("Checksum verified: %s", task.ChunkName)
		}
	}
}

// verifyFileChecksum calculates the SHA-256 checksum of a file and compares it with the expected checksum.
func verifyFileChecksum(filePath, expectedChecksum string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("error reading file for checksum verification: %v", err)
	}
	computedChecksum := computeDataChecksum(data)
	log.Println(computedChecksum)
	return computedChecksum == expectedChecksum, nil
}

func StreamWriter(metadata TorrentMetadata, chunkCoordinator *ChunkCoordinator) {
	tempFilePath := filepath.Join(DOWNLOAD_PATH, metadata.FileName+".crdownload")
	streamFile, err := os.Create(tempFilePath)
	if err != nil {
		log.Fatalf("Failed to create stream output: %v", err)
	}
	defer streamFile.Close()

	expectedChunk := 0
	totalChunks := len(metadata.ChunkChecksums)
	timeout := 10 * time.Millisecond

	for expectedChunk < totalChunks {
		select {
		case readyID := <-chunkCoordinator.chunkReady:
			if readyID == expectedChunk {
				chunkCoordinator.chunkMutex.Lock()
				data, exists := chunkCoordinator.chunkData[readyID]
				chunkCoordinator.chunkMutex.Unlock()
				
				if exists {
					streamFile.Write(data)
					if debug_mode {
						log.Printf("Streamed chunk %d", readyID)
					}
					expectedChunk++
				} else if debug_mode {
					log.Printf("Chunk %d ready but missing in map", readyID)
				}
			} else {

				// Requeue and wait for the chunk to become ready
				chunkCoordinator.chunkReady <- readyID
			}
		case <-time.After(timeout): 
		}
	}

	streamFile.Close()

	// Once all chunks are streamed, rename the temporary file to the final filename
	finalFilePath := filepath.Join(DOWNLOAD_PATH, metadata.FileName)
	err = os.Rename(tempFilePath, finalFilePath)
	if err != nil {
		log.Fatalf("Failed to rename file: %v", err)
	}

	log.Println("Streaming complete!")
}

type TorrentInfo struct {
	Metadata 	TorrentMetadata;
	Progress	int;
	Status		string;
}

func (c *PeerServer) GetLocalTorrents() ([]TorrentInfo, error) {
	files, err := os.ReadDir(TORRENTS_DIR)
	if err != nil {
		return nil, err
	}

	log.Println(TORRENTS_DIR)

	var torrents []TorrentInfo
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".torrent" {
			data, err := os.ReadFile(filepath.Join(TORRENTS_DIR, file.Name()))
			if err != nil {
				continue
			}
			var meta TorrentMetadata;
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}
			
			verified, _ := verifyFileChecksum(filepath.Join(DOWNLOAD_PATH, meta.FileName), meta.Checksum)
			
			torrent := TorrentMetadata{
				FileName:     meta.FileName,
				ArtistName:   meta.ArtistName,
				Peers:    meta.Peers,
				// Progress: 100,               // Default for now
				// Status:   "Complete",        // Future: check actual progress
				FileSize:    meta.FileSize,
				CreatedAt:  meta.CreatedAt, // Current timestamp
				Duration:   meta.Duration,               // Default for now
			}

			var torrent_info TorrentInfo;
			torrent_info.Metadata = torrent

			if verified {
				torrent_info.Progress = 100
				torrent_info.Status = "Seeding"
			} else {
				// GetStatus()
			}
			fmt.Printf("Appending: %+v\n", torrent_info)
			torrents = append(torrents, torrent_info)
		}
	}
	return torrents, nil
}


func (c *PeerServer) GetLibraryTorrents() ([]TorrentInfo, error) {
	files, err := os.ReadDir(LIBRARY_DIR)
	if err != nil {
		return nil, err
	}

	log.Println(LIBRARY_DIR)

	var torrents []TorrentInfo
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".torrent" {
			data, err := os.ReadFile(filepath.Join(LIBRARY_DIR, file.Name()))
			if err != nil {
				continue
			}
			var meta TorrentMetadata;
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}
			
			// verified, _ := verifyFileChecksum(filepath.Join(DOWNLOAD_PATH, meta.FileName), meta.Checksum)
			
			torrent := TorrentMetadata{
				FileName:     meta.FileName,
				ArtistName:   meta.ArtistName,
				Peers:    meta.Peers,
				// Progress: 100,               // Default for now
				// Status:   "Complete",        // Future: check actual progress
				FileSize:    meta.FileSize,
				CreatedAt:  meta.CreatedAt, // Current timestamp
				Duration:   meta.Duration,               // Default for now
			}

			var torrent_info TorrentInfo;
			torrent_info.Metadata = torrent

			// if verified {
			// 	torrent_info.Progress = 100
			// 	torrent_info.Status = "Downloaded"
			// } else {
			// 	// GetStatus()
			// }
			fmt.Printf("Appending: %+v\n", torrent_info)
			torrents = append(torrents, torrent_info)
		}
	}
	return torrents, nil
}