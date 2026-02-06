package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// PodInfo holds formatted pod information
type PodInfo struct {
	Name      string
	Namespace string
	NodeName  string
	Phase     string
	PodIP     string
	Restarts  int32
	Age       time.Duration
	CreatedAt time.Time
}

// getTotalRestarts calculates total restart count for all containers in a pod
func getTotalRestarts(containerStatuses []v1.ContainerStatus) int32 {
	var total int32
	for _, cs := range containerStatuses {
		total += cs.RestartCount
	}
	return total
}

// extractPodInfo extracts relevant information from a pod
func extractPodInfo(pod *v1.Pod, now time.Time) PodInfo {
	return PodInfo{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Phase:     string(pod.Status.Phase),
		PodIP:     pod.Status.PodIP,
		Restarts:  getTotalRestarts(pod.Status.ContainerStatuses),
		Age:       now.Sub(pod.CreationTimestamp.Time).Truncate(time.Second),
		CreatedAt: pod.CreationTimestamp.Time,
	}
}

// printPodInfo prints formatted pod information
func printPodInfo(info PodInfo) {
	fmt.Printf("Pod: %s\n", info.Name)
	fmt.Printf("  Namespace: %s\n", info.Namespace)
	if info.NodeName != "" {
		fmt.Printf("  Node: %s\n", info.NodeName)
	} else {
		fmt.Printf("  Node: <unscheduled>\n")
	}
	fmt.Printf("  Phase: %s\n", info.Phase)
	if info.PodIP != "" {
		fmt.Printf("  IP: %s\n", info.PodIP)
	} else {
		fmt.Printf("  IP: <none>\n")
	}
	fmt.Printf("  Restarts: %d\n", info.Restarts)
	fmt.Printf("  Age: %s\n\n", info.Age.String())
}

// createKubernetesClient creates and returns a Kubernetes client
func createKubernetesClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return client, nil
}

func main() {
	// Parse command line flags
	kubeconfig := flag.String("kubeconfig", "/Users/viskumar/.kube/config", "absolute path to the kubeconfig file")
	namespace := flag.String("namespace", "", "namespace to list pods from (empty for all namespaces)")
	flag.Parse()

	// Create Kubernetes client
	client, err := createKubernetesClient(*kubeconfig)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List pods
	pods, err := client.CoreV1().Pods(*namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing pods: %v", err)
	}

	if len(pods.Items) == 0 {
		if *namespace != "" {
			fmt.Printf("No pods found in namespace '%s'\n", *namespace)
		} else {
			fmt.Println("No pods found in the cluster")
		}
		os.Exit(0)
	}

	// Process and display pods
	now := time.Now()
	fmt.Printf("Found %d pods:\n\n", len(pods.Items))

	for i := range pods.Items {
		podInfo := extractPodInfo(&pods.Items[i], now)
		printPodInfo(podInfo)
	}

	if *namespace != "" {
		fmt.Printf("Total: %d pods in namespace '%s'\n", len(pods.Items), *namespace)
	} else {
		fmt.Printf("Total: %d pods across all namespaces\n", len(pods.Items))
	}
}
