package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	clientset "Kubernetes_Programming/pkg/generated/clientset/versioned"
)

func main() {
	// Parse kubeconfig path
	kubeconfig := flag.String("kubeconfig", getDefaultKubeconfig(), "path to kubeconfig file")
	namespace := flag.String("namespace", "default", "namespace to list At resources")
	flag.Parse()

	// Build config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create the generated clientset
	client, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	// List At resources in the specified namespace
	ctx := context.Background()
	fmt.Printf("Fetching 'At' resources from namespace '%s'...\n", *namespace)

	ats, err := client.CnatV1alpha1().Ats(*namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing At resources: %v", err)
	}

	// Display results
	if len(ats.Items) == 0 {
		fmt.Println("No At resources found")
		return
	}

	fmt.Printf("Found %d At resource(s):\n", len(ats.Items))
	for i, at := range ats.Items {
		fmt.Printf("%d. Name: %s\n", i+1, at.Name)
		fmt.Printf("   Schedule: %s\n", at.Spec.Schedule)
		fmt.Printf("   Command: %s\n", at.Spec.Command)
		if at.Status.Phase != "" {
			fmt.Printf("   Phase: %s\n", at.Status.Phase)
		}
		fmt.Println()
	}
}

// getDefaultKubeconfig returns the default kubeconfig path
func getDefaultKubeconfig() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
