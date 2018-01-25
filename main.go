package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nlopes/slack"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//TODO: bot commands, simple queries, nothing too invasive.

var (
	kubeconfig = flag.String("kubeconfig", "", "path to kubeconfig, if absent then we use rest.InClusterConfig()")
	apiserver  = flag.String("apiserver", "", "Url to apiserver, blank to read from kubeconfig")
	namespace  = flag.String("namespace", "", "Namespace to watch, blank means all namespaces")
	exclude    = flag.String("exclude", "", "Namespace to filter out")
)

var (
	kubeclient *kubernetes.Clientset
	slackAPI   = slack.New(os.Getenv("SLACK_TOKEN"))
)

const help = `help : Shows this help message

To list pods belonging to some parent resource
<type> <namespace> <name>

example

deployment default prometheus-operator

To list all available commands for resources

list
`

func init() {
	flag.Parse()
	//Initialize clientset, or die hard
	var cfg *rest.Config
	var err error
	if *kubeconfig == "" {
		cfg, err = rest.InClusterConfig()
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags(*apiserver, *kubeconfig)
	}
	if err != nil {
		log.Fatal(err)
	}
	kubeclient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
}

func prettyduration(dur time.Duration) string {
	dur = dur.Truncate(time.Second)
	if dur.Hours() > 24 {
		days := dur.Hours() / 24
		return fmt.Sprintf("%vd", int(days))
	}
	if dur.Hours() > 1 {
		dur = dur.Truncate(time.Minute)
	}
	if dur.Hours() > 12 {
		dur = dur.Truncate(time.Hour)
	}
	s := dur.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func renderpodlist(namespace, selector string) string {
	b := bytes.NewBufferString("")
	pods, err := kubeclient.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "Error: " + err.Error()
	}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, "NAMESPACE\tNAME\tREADY\tSTATUS\tRESTARTS\tAGE\tCPU\tMemory")
	//Fetch metrics from heapster...
	podCPU := make(map[string]*resource.Quantity)
	podMemory := make(map[string]*resource.Quantity)
	//TODO: Make optional
	resp := kubeclient.CoreV1().Services("kube-system").ProxyGet("http", "heapster", "80", "/apis/metrics/v1alpha1/pods", map[string]string{"labelSelector": selector})
	byt, err := resp.DoRaw()
	if err != nil {
		log.Println(err) //Silent fail
	} else {
		res := &PodMetricsList{}
		err = json.Unmarshal(byt, res)
		if err != nil {
			log.Println(err) //Silent fail
		} else {
			//log.Println(res)
			for _, pod := range res.Items {
				for _, container := range pod.Containers {
					if podCPU[pod.Name] == nil {
						podCPU[pod.Name] = container.Usage.Cpu()
					} else {
						podCPU[pod.Name].Add(*container.Usage.Cpu())
					}
					if podMemory[pod.Name] == nil {
						podMemory[pod.Name] = container.Usage.Memory()
					} else {
						podMemory[pod.Name].Add(*container.Usage.Memory())
					}
				}
			}
		}
	}
	//Hopefully we have these
	//log.Println(podCPU, podMemory)
	for _, pod := range pods.Items {
		containers := 0
		containerReady := 0
		var restartCount int32
		var cpu *resource.Quantity
		var memory *resource.Quantity
		for _, container := range pod.Spec.Containers {
			if cpu == nil {
				cpu = container.Resources.Requests.Cpu()
			} else {
				cpu.Add(*container.Resources.Requests.Cpu())
			}
			if memory == nil {
				memory = container.Resources.Requests.Memory()
			} else {
				memory.Add(*container.Resources.Requests.Memory())
			}
			//log.Println(cpu)
			//log.Println(memory)
		}

		for _, container := range pod.Status.ContainerStatuses {
			containers++
			if container.Ready {
				containerReady++
			}
			restartCount += container.RestartCount
		}
		var cpustr string
		var memstr string
		//Rounding logic via https://github.com/kubernetes/kubernetes/blob/9fed4878ee66cd4bfb0684897b30403cc28c2205/pkg/kubectl/metricsutil/metrics_printer.go#L190-L199
		if podCPU[pod.Name] != nil {
			cpustr = fmt.Sprintf("%vm/%vm %.1f%%", podCPU[pod.Name].MilliValue(), cpu.MilliValue(), 100*float64(podCPU[pod.Name].MilliValue())/float64(cpu.MilliValue()))
		} else {
			cpustr = cpu.String()
		}
		if podMemory[pod.Name] != nil {
			//Find appropriate scale...
			memstr = fmt.Sprintf("%vMi/%vMi %.1f%%", podMemory[pod.Name].Value()/(1024*1024), memory.Value()/(1024*1024), 100*float64(podMemory[pod.Name].MilliValue())/float64(memory.MilliValue()))
		} else {
			memstr = memory.String()
		}

		fmt.Fprintf(w, "%s\t%s\t%v/%v\t%s\t%v\t%s\t%s\t%s\n", pod.Namespace, pod.Name, containerReady, containers, pod.Status.Phase, restartCount, prettyduration(time.Since(pod.Status.StartTime.Time)), cpustr, memstr)
	}
	w.Flush()
	return b.String()
}

//Fetches pods associated with parentType/parentName
func podlist(parentType, parentNamespace, parentName string) string {
	switch parentType {
	case "deployment":
		fallthrough
	case "deployments":
		fallthrough
	case "deploy":
		dp, err := kubeclient.AppsV1beta1().Deployments(parentNamespace).Get(parentName, metav1.GetOptions{})
		if err != nil {
			return "Error: " + err.Error()
		}
		result := fmt.Sprintf("Replicas: %v/%v of %v\n", dp.Status.ReadyReplicas, dp.Status.AvailableReplicas, dp.Status.Replicas)
		//Ugly way to generate LabelSelector from MatchLabels. Can't find helper in client-go
		pairs := make([]string, 0)
		for k, v := range dp.Spec.Selector.MatchLabels {
			pairs = append(pairs, k+"="+v)
		}
		selector := strings.Join(pairs, ",")
		return result + renderpodlist(dp.Namespace, selector)
	case "statefulsets":
		fallthrough
	case "sts":
		ss, err := kubeclient.AppsV1beta1().StatefulSets(parentNamespace).Get(parentName, metav1.GetOptions{})
		if err != nil {
			return "Error: " + err.Error()
		}

		result := fmt.Sprintf("Replicas: %v/%v of %v\n", ss.Status.ReadyReplicas, ss.Status.CurrentReplicas, ss.Status.Replicas)
		//Ugly way to generate LabelSelector from MatchLabels. Can't find helper in client-go
		pairs := make([]string, 0)
		for k, v := range ss.Spec.Selector.MatchLabels {
			pairs = append(pairs, k+"="+v)
		}
		selector := strings.Join(pairs, ",")
		return result + renderpodlist(ss.Namespace, selector)
	case "daemonsets":
		fallthrough
	case "ds":
		ds, err := kubeclient.ExtensionsV1beta1().DaemonSets(parentNamespace).Get(parentName, metav1.GetOptions{})
		if err != nil {
			return "Error: " + err.Error()
		}
		result := fmt.Sprintf("Replicas: %v/%v of %v\n", ds.Status.NumberReady, ds.Status.CurrentNumberScheduled, ds.Status.NumberAvailable)
		pairs := make([]string, 0)
		for k, v := range ds.Spec.Selector.MatchLabels {
			pairs = append(pairs, k+"="+v)
		}
		selector := strings.Join(pairs, ",")
		return result + renderpodlist(ds.Namespace, selector)
	default:
		return "Unsupported type: " + parentType
	}
}

func listCommands() string {
	//Deployments
	dps, err := kubeclient.AppsV1beta1().Deployments("").List(metav1.ListOptions{})
	if err != nil {
		return "Error: " + err.Error()
	}
	result := ""
	for _, dp := range dps.Items {
		result += fmt.Sprintf("deploy %s %s\n", dp.Namespace, dp.Name)
	}
	//Statefulsets
	sts, err := kubeclient.AppsV1beta1().StatefulSets("").List(metav1.ListOptions{})
	if err != nil {
		return "Error: " + err.Error()
	}
	for _, s := range sts.Items {
		result += fmt.Sprintf("sts %s %s\n", s.Namespace, s.Name)
	}
	//DaemonSets
	dss, err := kubeclient.ExtensionsV1beta1().DaemonSets("").List(metav1.ListOptions{})
	if err != nil {
		return "Error: " + err.Error()
	}
	for _, s := range dss.Items {
		result += fmt.Sprintf("ds %s %s\n", s.Namespace, s.Name)
	}
	return result
}

//Stollen from https://github.com/beeradb/kubectl-slackbot/blob/master/main.go
func kubectlproxy() {
	log.Println("start")
	rtm := slackAPI.NewRTM()
	go rtm.ManageConnection()
	var UserID string
	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			//Ignoring
		case *slack.ConnectedEvent:
			UserID = ev.Info.User.ID
			log.Println(UserID)
		case *slack.MessageEvent:
			log.Println(ev.Text)
			command := strings.Trim(strings.TrimPrefix(ev.Text, "<@"+UserID+">"), " ")
			command = strings.Replace(command, "—", "--", -1)
			if len(command) > 0 && !strings.Contains(command, "uploaded a file") {
				if command[0:1] == ":" {
					command = command[1:]
				}
				log.Println(command)
				switch command {
				case "help":
					rtm.SendMessage(rtm.NewOutgoingMessage(help, ev.Channel))
				case "list":
					rtm.SendMessage(rtm.NewOutgoingMessage(listCommands(), ev.Channel))
				default:
					splitted := strings.Split(command, " ")
					if len(splitted) != 3 {
						rtm.SendMessage(rtm.NewOutgoingMessage("3 words needed", ev.Channel))
					} else {
						result := podlist(splitted[0], splitted[1], splitted[2])
						rtm.SendMessage(rtm.NewOutgoingMessage(fmt.Sprintf("```%s```", result), ev.Channel))
					}
				}
				/*
					if !strings.HasPrefix(command, "get") {
						//ONLY GET ALLOWED!!!11
						rtm.SendMessage(rtm.NewOutgoingMessage("only get type operations allowed", ev.Channel))
					} else {
						result = kubectl(command)
						params := slack.FileUploadParameters{
							Title:    "Kubectl result",
							Filetype: "shell",
							File:     "sh",
							Channels: []string{ev.Channel},
							Content:  result,
						}
						file, err := api.UploadFile(params)
						if err != nil {
							fmt.Printf("%s\n", err)
						}
						log.Printf("Name: %s, URL: %s\n", file.Name, file.URL)
					}*/
			}

		case *slack.InvalidAuthEvent:
			log.Fatal("Invalid credentials")
		default:
		}
	}
	log.Println("end")
}

func sendtoslack(e *v1.Event) error {
	//Stollen from https://github.com/ultimateboy/slack8s
	//TODO: Output needs to be more compact. Does slack have expandable things?
	params := slack.PostMessageParameters{}
	attachment := slack.Attachment{
		// The fallback message shows in clients such as IRC or OS X notifications.
		Fallback: e.Message,
		Text:     e.Message,
		Fields: []slack.AttachmentField{
			slack.AttachmentField{
				Title: "Namespace",
				Value: e.Namespace,
				Short: true,
			},
			slack.AttachmentField{
				Title: "Object",
				Value: e.InvolvedObject.Kind,
				Short: true,
			},
			slack.AttachmentField{
				Title: "Name",
				Value: e.Name,
				Short: true,
			},
			slack.AttachmentField{
				Title: "Reason",
				Value: e.Reason,
				Short: true,
			},
			slack.AttachmentField{
				Title: "Component",
				Value: e.Source.Component,
				Short: true,
			},
			slack.AttachmentField{
				Title: "Count",
				Value: fmt.Sprintf("%d", e.Count),
				Short: true,
			},
			slack.AttachmentField{
				Title: "First",
				Value: e.FirstTimestamp.Format(time.RFC3339),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Last",
				Value: e.LastTimestamp.Format(time.RFC3339),
				Short: true,
			},
		},
	}

	if strings.HasPrefix(e.Reason, "Success") {
		attachment.Color = "good"
	} else if strings.HasPrefix(e.Reason, "Fail") {
		attachment.Color = "danger"
	}
	params.Attachments = []slack.Attachment{attachment}

	channelID, timestamp, err := slackAPI.PostMessage(os.Getenv("SLACK_CHANNEL"), "", params)
	if err != nil {
		fmt.Printf("%s\n", err)
		return err
	}

	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	return nil

}

func main() {
	//Start the bot things
	go kubectlproxy()
	//log.Println(podlist("deploy", "newrealtime", "hdbloader"))
	//Do a simple list call to fetch ResourceVersion
	evlist, err := kubeclient.CoreV1().Events(*namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	watchopt := metav1.ListOptions{Watch: true}
	if len(evlist.Items) > 0 {
		watchopt.ResourceVersion = evlist.GetListMeta().GetResourceVersion()
	}
	//watcher, err := cache.NewListWatchFromClient(kubeclient.RESTClient(), "event", *namespace, fields.Everything()).Watch(watchopt)
	for {
		log.Println(watchopt)
		watcher, err := kubeclient.CoreV1().Events(*namespace).Watch(watchopt)
		if err != nil {
			log.Fatal(err)
		}
		for item := range watcher.ResultChan() {
			//log.Println(item.Type, item.Object)
			//Make note of last processed item
			event, ok := item.Object.(*v1.Event)
			if ok {
				watchopt.ResourceVersion = event.GetResourceVersion()
				// Filter namespace
				if event.Namespace != *exclude {
					log.Println(event.Name, event.Namespace, event.Count, event.Type, event.Reason, event.FirstTimestamp, event.LastTimestamp, event.Message)
					err := sendtoslack(event)
					if err != nil {
						log.Println(err)
					}
				}
			} else {
				log.Println("Not *v1.Event", item)
			}
		}
		log.Println("Resuming loop")
	}
}
