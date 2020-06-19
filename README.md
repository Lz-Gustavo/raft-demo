# raft-demo

This repository organizes scripts, papers, and experiment applications developed using [hashicorp's Go implementation](https://github.com/hashicorp/raft) of the [Raft consensus algorithm](https://raft.github.io).

**IMPORTANT** *beelog-exp* is currently an unstable branch. Some dependencies are maintained unavailable until final adjustments. Beelog implementation will be omited until TLA+ formal verification is fully finished.

## Non-application directories

* **docs**

	Research papers contemplating experiments utilizing the applications from this repository.

* **scripts**

	Useful scripts to conduct experimentations using **kvstore** and **diskstorage** applications.

* **deploys**

	Useful YAMLs to deploy **kvstore** and **diskstorage** applications under different configurations and topologies on a Kubernetes cluster.

* **monit**

	Resource usage monitoring scripts utilizing [psutil](https://github.com/giampaolo/psutil).

## Applications

* **kvstore**

	A key-value in-memory SM-Replicated storage server. Applies received "get", "set" and "delete" operations on a regular map, using Raft to ensure total order.

* **logger**

	A logger process that acts as a non-Voter replica on the raft cluster, only logging committed commands to it's own log file.

* **diskstorage**

	A persistent storage application. Applies received "get", "set" and "delete" operations on a regular file following a calculated offset, simply defined by (key * storeValueOffset). Uses the same logic from **kvstore** application, except it's storage and FSM implementation.

* **recovery**

	A dummy client implementation that sends a state transfer request to application replica's after a pre-defined timeout.

* **[DEPRECATED] webkvstore**
	
	[Otoolep's](https://github.com/otoolep/hraftd) reference example of hashicorp raft, intially logging committed messages using Journey.

* **[DEPRECATED] chatRoom**

	A naieve implementation of a chat room service following publish-subscriber pattern. On every chat message received, broadcasts it to every client connected.
	
	- Client execution is configured by a .toml file, and store received messages on a queue to discard equal messages sent by other replica.

	- Channels implementation based from [drewolson's gist chat.go](https://gist.github.com/drewolson/3950226).

## Usage

### CMDLI chatRoom, kvstore and diskstorage 

1. Set the number of replicas, their corresponding IP's and a port to listen for cluster UDP repplies on a .toml config file

	```toml
	rep=3
	svrIps=["127.0.0.1:11000", "127.0.0.1:11001", "127.0.0.1:11002"]
	udpport=15000
	```

2. Build and run the first server, passing a corresponding address to handle new join requests to the cluster. If no "-port" and "-raft" are set, ":11000" and ":12000" are assumed.

	```bash
	go build
	./kvstore -id node0 -hjoin :13000
	```

3. Build and run the other replicas, configuring different address to listen for clients' requests and another to comunicate with the raft cluster. Also, don't forget to join the first replicated on the defined addr.
	
	```bash
	go build
	./kvstore -id node1 -port :11001 -raft :12001 -join :13000
	./kvstore -id node2 -port :11002 -raft :12002 -join :13000
	```

4. If needed, run the logger processes to record new entries to the Raft FMS on a txt file.
	
	```bash
	go build
	./logger -id log1 -raft :12003 -join :13000
	./logger -id log2 -raft :12004 -join :13000
	```

5. Now execute any number of clients and send desirable requisitions to the cluster.

	```bash
	go build
	./client -config=../client-config.toml
	```

### OBS:

- kvstore application can interpret the [Protocol Buffers](https://developers.google.com/protocol-buffers/) message format specified at journey/pb.Message, or the ad-hoc format described bellow:

	```bash
	get-[key]
	set-[key]-[value]
	delete-[key]
	```

- A single Logger processes can connect to multiple Raft clusters. Simply pass unique value tuples as command line arguments:

	```bash
	./logger -id 'log0,log1' -raft ':12000,:12001' -join ':13000,:13001'
	```

### Docker

In order to reference go mods inside Docker build context, you must build the desired application imagem from the repository root folder, like the example below:

```bash
docker build -f kvstore/Dockerfile -t kvstore .
```

### Kubernetes
After properly installing Kubernetes via [kubeadm](https://vitux.com/install-and-deploy-kubernetes-on-ubuntu/) or [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), a series of YAML resources for easily test and analyses under different scenarios and configurations are available at **deploys/**. To deploy a simple cluster of three replicas and a load generator, first deploy the application's resource:

```bash
kubectl apply -f deploys/kv-notlog.yaml
```

Automatic IP discovery between replicas, load generators, and loggers is implemented by defining naming conventions and tags for each pod, which is consulted on runtime via [k8s/client-go](https://github.com/kubernetes/client-go). After waiting a few seconds for pod IP attribution (you can check status via ```kubectl get nodes```), launch the corresponding load generator:

```bash
kubectl apply -f deploys/loadgen-kv.yaml
```

For a more detailed explanation about the different YAMLs and naming conventions, check **deploys/README.md**.

## Profiling

**kvstore** and **diskstorage** applications supports both CPU and memory profiling from [pprof](https://golang.org/pkg/runtime/pprof/) library. To record some performance metrics, you just need to run the server programm passing the flags:

```bash
./kvstore -cpuprofile=filename.prof -memprofile=filename.prof
```

In case you want to measure the efficiency of the decoupled logger process against application level logging, you can force both **kvstore** and **diskstorage** applications to synchronously save each new requisition into a log, by passing the flag:

```bash
./kvstore -logfolder=/path/to/folder/
```

## License
[MPL 2.0](https://www.mozilla.org/en-US/MPL/2.0/)
