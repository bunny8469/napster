# napster
This project implements a Napster-inspired P2P file-sharing system with a centralized indexing server for efficient search and discovery. It balances scalability and bandwidth by enabling direct peer-to-peer file transfers while maintaining secure, reliable, and fault-tolerant communication

## Instructions:
- First run server: 
```
go run server/server.go
```

- Then run the number of clients you wish to run:
```
go run client/client.go -port=<port_number>
```

- On running the client you will be asked to choose an option as shown below:
```
Choose an option: 
1. Register Files
2. Search File
3. Exit
```

- On choosing 1 or 2 you will be asked for a file path to register or search, search returns the peers storing the file redundancy is also implemented.

- Heartbeats are also implmented to make sure if the peer goes offline.