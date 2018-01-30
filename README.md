# k8s2slack
Slack bot for visibility into Kubernetes.

Has 2 functions

1. Stream Kubernetes events into a Slack channel.
2. Provide some commands to gain visibility into pods.


### Usage

Only tested against Kubernetes 1.7.x

#### In-cluster Kubernetes deployment

```
kubectl create secret generic slack --from-literal=bot_token=xXXb-XXXXXXXXXX-CCCCCC
kubectl create -f ds.yaml
```

#### Running out-of-cluster using docker

```
docker run --rm -v ~/.kube/config:/tmp/config -e SLACK_TOKEN="xXXb-XXXXXXXXXX-CCCCCC" -e SLACK_CHANNEL="#kubernetes" -it turbobytes/k8s2slack /bin/k8s2slack -kubeconfig /tmp/config -exclude kubemr
```

#### Manually running out-of-cluster

```
go get github.com/turbobytes/k8s2slack
cd $GOPATH/github.com/turbobytes/k8s2slack
glide up -v
SLACK_TOKEN="xXXb-XXXXXXXXXX-CCCCCC" SLACK_CHANNEL="#kubernetes" go run *.go -kubeconfig ~/.kube/config -exclude kubemr
```

##### Usage

```
-apiserver string
    Url to apiserver, blank to read from kubeconfig
-exclude string
    Namespace to filter out
-heapsterns string
    The namespace heapster is running on, set to blank to dissable (default "kube-system")
-heapsterport string
    The port for heapster, set to blank to dissable (default "80")
-heapsterscheme string
    The service name for heapster, set to blank to dissable (default "http")
-heapstersvc string
    The service name for heapster, set to blank to dissable (default "heapster")
-kubeconfig string
    path to kubeconfig, if absent then we use rest.InClusterConfig()
-namespace string
    Namespace to watch, blank means all namespaces
```

(Removed flags that are added from client-go)

Streams all Kubernetes events to `#kubernetes` channel.

`SLACK_TOKEN` is the oAuth token for your bot user.

Additionally, you can interact with the bot user to get some state of the cluster. The bot can be interacted with in direct message, or in a room where the bot is a member.

`list` - shows a list of all possible commands

Example output

```
deploy default prometheus-operator
deploy infra fluentd
deploy infra grafana-core
deploy infra minio
deploy kube-system cluster-autoscaler
deploy kube-system dns-controller
deploy kube-system heapster
deploy kube-system kube-dns
deploy kube-system kube-dns-autoscaler
deploy kube-system kube-state-metrics
deploy kube-system kubernetes-dashboard
sts infra etcd
sts infra nats
sts infra prometheus-infra-prometheus
sts kube-system prometheus-kube-system-prometheus
ds default logdna-agent
ds default nginx
ds kube-system node-exporter
```

(Shortened the output)


`<resource-type> <namespace> <resource-name>` Lists pods belonging to a resource.

Example `ds default logdna-agent`

```
Replicas: 18/18 of 18
NAMESPACE NAME               READY STATUS  RESTARTS AGE CPU            Memory
default   logdna-agent-1gftm 1/1   Running 0        15d 19m/100m 19.0% 140Mi/500Mi 28.1%
default   logdna-agent-1q4wn 1/1   Running 0        15d 29m/100m 29.0% 370Mi/500Mi 74.1%
default   logdna-agent-1z3q5 1/1   Running 0        15d 100m           500Mi
default   logdna-agent-43hwk 1/1   Running 0        15d 25m/100m 25.0% 249Mi/500Mi 50.0%
default   logdna-agent-4d1sn 1/1   Running 0        15d 38m/100m 38.0% 383Mi/500Mi 76.6%
default   logdna-agent-8x2rz 1/1   Running 0        15d 30m/100m 30.0% 411Mi/500Mi 82.2%
default   logdna-agent-92j30 1/1   Running 0        15d 24m/100m 24.0% 177Mi/500Mi 35.5%
default   logdna-agent-c7wwh 1/1   Running 0        15d 26m/100m 26.0% 377Mi/500Mi 75.6%
default   logdna-agent-clqkk 1/1   Running 0        15d 36m/100m 36.0% 371Mi/500Mi 74.3%
default   logdna-agent-f66q7 1/1   Running 0        15d 18m/100m 18.0% 82Mi/500Mi 16.4%
default   logdna-agent-hsl39 1/1   Running 0        15d 15m/100m 15.0% 127Mi/500Mi 25.5%
default   logdna-agent-kw1s3 1/1   Running 0        15d 33m/100m 33.0% 380Mi/500Mi 76.1%
default   logdna-agent-pl59g 1/1   Running 0        15d 27m/100m 27.0% 280Mi/500Mi 56.2%
default   logdna-agent-sc9bd 1/1   Running 0        15d 33m/100m 33.0% 324Mi/500Mi 65.0%
default   logdna-agent-tjx77 1/1   Running 0        15d 27m/100m 27.0% 323Mi/500Mi 64.6%
default   logdna-agent-tkcpq 1/1   Running 0        15d 22m/100m 22.0% 258Mi/500Mi 51.7%
default   logdna-agent-w96l9 1/1   Running 0        15d 23m/100m 23.0% 247Mi/500Mi 49.4%
default   logdna-agent-x6mjx 1/1   Running 0        15d 19m/100m 19.0% 348Mi/500Mi 69.7%
```

The CPU and Memory columns show resource used vs resource requested.

![screenshot](/screenshot.png?raw=true "Screenshot")

### TODO

1. Support more types - nodes, etc.
2. Make bot response easier to read on mobile.
3. Prettify/compact event notifications.
