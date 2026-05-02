package client

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	config     *rest.Config
	configOnce sync.Once
)

// SINGLETON CONFIG
func GetKubeConfig() *rest.Config {
	configOnce.Do(func() {

		// Try in-cluster config first
		cfg, err := rest.InClusterConfig()
		if err == nil {
			config = cfg
			fmt.Println("Using in-cluster config")
			return
		}

		// Fallback to local kubeconfig
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")

		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(fmt.Sprintf("failed to load kubeconfig: %v", err))
		}

		config = cfg
		fmt.Println("Using local kubeconfig")
	})

	return config
}

// CLIENTSET (cached config)
func GetClientset() *kubernetes.Clientset {
	cfg := GetKubeConfig()

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(fmt.Sprintf("clientset error: %v", err))
	}

	return clientset
}

// DYNAMIC CLIENT
func GetDynamicClient() dynamic.Interface {
	cfg := GetKubeConfig()

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		panic(fmt.Sprintf("dynamic client error: %v", err))
	}

	return dynamicClient
}