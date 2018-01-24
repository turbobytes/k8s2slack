package main

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Was having issues with importing both heapster and client-go using glide, so just stealing the relavent bits.
//https://github.com/kubernetes/heapster/blob/master/metrics/apis/metrics/types.go
//Perhaps this should be in a library. main files feels dirty

// resource usage metrics of a container.
type ContainerMetrics struct {
	// Container name corresponding to the one from pod.spec.containers.
	Name string `json:"name"`
	// The memory usage is the memory working set.
	Usage v1.ResourceList `json:"usage"`
}

// resource usage metrics of a pod.
type PodMetrics struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The following fields define time interval from which metrics were
	// collected from the interval [Timestamp-Window, Timestamp].
	Timestamp metav1.Time     `json:"timestamp"`
	Window    metav1.Duration `json:"window"`

	// Metrics for all containers are collected within the same time window.
	Containers []ContainerMetrics `json:"containers"`
}

// PodMetricsList is a list of PodMetrics.
type PodMetricsList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#types-kinds
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of pod metrics.
	Items []PodMetrics `json:"items"`
}
