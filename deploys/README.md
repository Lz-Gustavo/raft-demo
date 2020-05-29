## deploys

This directory indexes various YAML resources to easily launch **kvstore**, **diskstorage**, load generators, and loggers on a Kubernetes environment.

### Naming convention
**disk** and **kv** prefixes represent diskstorage and kvstore applications respectively. Their different YAMLs variate on monitoring configuration (**monit** flag), that when set deploys on the same application node an image running a [psutil](https://github.com/giampaolo/psutil) script, monitoring disk usage and network io; and the **applog** or **notlog** sufixes, that enables or disables application-level logging during exection, respectively. The **monit** variant is also present on **loggers** and **loadgen** deployments.

**loadgen** YAMLs launch a predefined number of loadgenerators (set by prefix number), that stimulate each group of application replicas running over the current namespace. Its **-script** variant executes the local **script/run.sh** saved on its current image, whereas the non-script ones (e.g. **loadgen-kv.yaml**) stimulate their choosen application with staticaly defined parameters (number of clients, execution time, etc).


### Automatic IP discovery
During initialization, processes utilize the official [k8s Go SDK](https://github.com/kubernetes/client-go) API to discover all the different pods running on the same namespace. By matching container image names, with simple string comparisons, the configured topology is eventually applied during deployment. Follower pods may crash during initilization by not finding a corresponding leaders IP, even it is already launched. This result is a normal behavior treated by the algorithm, and it happens because leader pods are not immediately scheduled into nodes, nor ensured to be launched before followers. After safely crashing, the container is automatically restarted by Kube, allowing the follower to once again try to identify the matching leader's IP.

For example, consider the following scenario:
* A **kvstore-follower-2** pod is deployed.
* During initilization, the application searches for its corresponding ***-leader-2** leader node IP address.
* If the leader has an attributed IP, the follower sends a *join* request in order to establish connection, and so participate on the consensus quorum.
* If it didn't find a leader, or the leader doesn't have an IP address, the node safely crashes and tries again.


### Anti-affinity rules
Using *podAntiAffinity* property, some schedule rules are implemented to satisfy architecture safety properties and ensure a trusty comparison during evaluation.

1. **Individual application replicas must always be distributed across different nodes.**

	Respecting the replication factor of *2f + 1* replicas and *k + 1* loggers, deployment is configured to not schedule pods in nodes that already have pods running with the same application identifier.

2. **Leaders from different applications are never scheduled into same nodes.**

	To avoid nodes to be overloaded, this rule prohibits the allocation of multiple leaders of consensus protocol in a same machine. This is done by assigning the *r3-leader* label on every pod hosting an application leader.

3. **Loggers are evenly distributed and do not share a leader-scheduled nodes.**

	To avoid performance interference caused by workload imposed to the consensus protocol, logger instances do not execute in the same machines where leaders of consensus protocol are executing. This is done by assigning a podAntiAffinity for *leader* labels on every logger pod.

4. **Load generators do not share leader-scheduled nodes.**

	Related to the performance evaluation, aims to avoid oscillation during load generation. It ensures that *loadgen* pods are never hosted together with consensus leaders, which would cause one-round propagation with unfair latency metrics being observed.
