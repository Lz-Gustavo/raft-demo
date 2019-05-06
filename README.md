# raft-demo

This repository organizes some demo applications using [hashicorp's Go implementation](https://github.com/hashicorp/raft) of the [Raft Consensus Algorithm](https://raft.github.io).

## Applications

* **chatRoom**
	
	A naieve implementation of a chat room service following publish-subscriber pattern. On every chat message received, broadcasts it to every client connected.
	
	- Client execution is configured by a .toml file, and store received messages on a queue to discard equal messages sent by other replica.

	- /logger/ is a logger process that acts as a non-Voter replica on the raft cluster, only logging committed commands to it's own log file using Journey¹.

	- Channels implementation based from [drewolson's gist chat.go](https://gist.github.com/drewolson/3950226).
	
	- ¹Journey: an optimized logging package for log-recovery, work in progress.

* **kvstore**
	
	A key-value in-memory SM-Replicated storage server. Applies received "get", "set" and "delete" operations on a regular map, using Raft to ensure total order.

* **webkvstore**
	
	[Otoolep's](https://github.com/otoolep/hraftd) reference example of hashicorp raft, intially logging committed messages using Journey.

## Usage

**chatRoom and kvstore** 

1. Set the number of replicas and their corresponding IP's on a .toml config file

		```
		rep=3
		svrIps=["127.0.0.1:11000", "127.0.0.1:11001", "127.0.0.1:11002"]
		mqueueSize=100
		```

2. Build and run the first server, passing a corresponding address to handle new join requests to the cluster. If no "-port" and "-raft" are set, ":11000" and ":12000" are assumed.
	
		```
		go build
		./server -id node0 -hjoin :13000
		```

3. Build and run the other replicas, configuring different address to listen for clients' requests and another to comunicate with the raft cluster. Also, don't forget to join the first replicated on the defined addr.
	
		```
		go build
		./server -id node1 -port :11001 -raft :12001 -join :13000
		./server -id node2 -port :11002 -raft :12002 -join :13000
		```

4. If needed, run the logger processes to record new entries to the Raft FMS on a txt file.
	
		```
		go build
		./logger -id log1 -raft :12003 -join :13000
		./logger -id log2 -raft :12004 -join :13000
		```

5. Now execute any number of clients and send desirable requisitions to the cluster.

		```
		go build
		./client -config=../client-config.toml
		```

**OBS:** Message formats accepted by the kvstore are:

		```
		get-[key]
		set-[key]-[value]
		delete-[key]
		``` 

## Profiling

**kvstore** application supports both CPU and memory profiling from [pprof](https://golang.org/pkg/runtime/pprof/) library. To record some performance metrics, you just need to run the server programm passing the flags:

		```
		./server -cpuprofile=filename.prof -memprofile=filename.prof
		```

## License
MPL 2.0