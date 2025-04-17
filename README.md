# napster

This project implements a Napster-inspired P2P file-sharing system with a centralized indexing server for efficient search and discovery. It balances scalability and bandwidth by enabling direct peer-to-peer file transfers while maintaining secure, reliable, and fault-tolerant communication

## Interface

0. `cd interface`
1. `wails clean`
2. `mkdir build`
3. Copy `appicon.png` to build folder
4. `wails build`
5. `wails dev`

## Instructions

- First run server:

```bash
go run server/server.go
```

- Then run the number of clients you wish to run:

```bash
go run client/client.go -port=<port_number>
```

- On running the client you will be asked to choose an option as shown below:

```bash
Choose an option: 
1. Register Files
2. Search File
3. Exit
```

- To download Fuzzy Search:

  ```bash
  go get github.com/lithammer/fuzzysearch/fuzzy
  ```

- On choosing 1 or 2 you will be asked for a file path to register or search, search returns the peers storing the file redundancy is also implemented.

- Heartbeats are also implmented to make sure if the peer goes offline
- Add 