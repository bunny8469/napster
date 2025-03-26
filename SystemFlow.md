# Napster
*(Jahnavi, Kriti, Sai Praneeth)*

## User Interface
**Wails (Best for Web + Native Integration)**
https://wails.io

Svelte + Tailwind + Go

### Napster Flow
1. Central Indexing Server stores the `.torrent` files. Individual file for each music file, mapping should be stored in a different file.
2. Torrents store metadata of a music file. `musix_randomXYu.torrent` file format
```
file_name: musix_randomXYu.mp3
artist_name: Alan Walker ft. Sabrina Carpenter
created_at: 172892350
file_size (bytes): 987890
chunk_size (bytes): 65536
duration (seconds): 173
checksum: 3849027328352347901237049

CHUNK_DATA
chunk_0: CHECKSUM_OF_CHUNK_0
chunk_1: CHECKSUM_OF_CHUNK_1
chunk_2: CHECKSUM_OF_CHUNK_2
chunk_3: CHECKSUM_OF_CHUNK_3
...

PEERS
localhost:5123
localhost:5121
localhost:5122
localhost:5126
```
3. Also define a struct similar to this format

**1. Chunking Flow:**
---
(For now, let's focus on only CHUNK_DATA and PEERS in `.torrent` files, other details can be added later)

1. Define a chunking algorithm, common to both client & indexing server (same CHUNK_SIZE).
2. Client uploads a music file to the central server.
3. Central Server appends a random text / number to the end of file name. `musix.mp3 -> musix_randomXYu.mp3`
4. Central Server computes full hash of the file. Then, divides the file into chunks, computes checksums of each chunk, and stores them in struct.
5. Initially, only the peer who uploaded the file will be present in the peer list (will see redundancy logic later).
6. Store all these details (checksums, file_name, file_size, created_at, peer) in the struct. 
7. Now, after the computation, export this struct into a `.torrent` file. 
8. Send response back to the client with the updated name (`musix_randomXYu.mp3`)
9. Client upon receival of this response, will do the chunking locally, and stores the chunks in `chunks/` folder. Choose some naming scheme for the chunks. Example, `musix_randomXYu_chunk_0.chunk`

**2. Rebuilding Chunks:**
---
1. Write a function, which takes file_name as input, it should get all the chunks, merge all of them in order, and finally form the required audio.
2. Check if the music file remains same, plays same after merging the divided chunks.

### Edits
- (edits here)

### Additional Features
- Remember that built client application will be an exe file. No changes after.
- Add "Change IP address of  central server" option in settings.
- Maintain track of unfinished downloads at client
- After dividing the music file into chunks and sending them to respective peers, store the peers' details in a `.torrent`file. You can directly send this `.torrent` file to the leeching peer.
- Contributor Nodes (option for client which allows them to act as redundant servers, can be added later)
- Add option to pause / resume seeding at client’s side.