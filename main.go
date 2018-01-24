package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nlopes/slack"
	"k8s.io/api/core/v1"
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
	api        = slack.New(os.Getenv("SLACK_TOKEN"))
)

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

	channelID, timestamp, err := api.PostMessage(os.Getenv("SLACK_CHANNEL"), "", params)
	if err != nil {
		fmt.Printf("%s\n", err)
		return err
	}

	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	return nil

}

func main() {
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
