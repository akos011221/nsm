package controller

import "k8s.io/client-go/kubernetes"

// Controller is the controller implementation for NetworkService and NetworkConnection resources
type Controller struct {
	// Standard kubernetes clientset
	kubeClientset kubernetes.Interface
}
