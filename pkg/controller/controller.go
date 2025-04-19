package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akos011221/nsm/pkg/config"
	"github.com/akos011221/nsm/pkg/hardware"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Controller manages the NSM components
type Controller struct {
	// Configuration
	config *config.Config
	// Logger
	logger *logrus.Logger
	// Kubernetes client
	clientset *kubernetes.Clientset
	// Context for cancellation
	ctx context.Context
	// Cancel function
	cancel context.CancelFunc
	// Wait group for goroutines
	wg sync.WaitGroup

	// Component managers
	sriovManager *hardware.SRIOVManager
}

// NewController creates a new controller instance
func NewController(cfg *config.Config, logger *logrus.Logger) (*Controller, error) {
	ctx, cancel := context.WithCancel(context.Background())

	ctrl := &Controller{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	/* k8s client */

	k8sConfig, err := getKubernetesConfig(cfg.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	ctrl.clientset = clientset

	// initialize components
	if err := ctrl.initComponents(); err != nil {
		cancel() // clean up the context
		return nil, err
	}

	return ctrl, nil
}

// getKubernetesConfig creates a Kubernetes config from kubeconfig or in-cluster config
func getKubernetesConfig(kubeconfigPath string) (*rest.Config, error) {
	// try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// fall back to kubeconfig file
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return nil, fmt.Errorf("could not create kubernetes config: %v", err)
}

// initComponents initializes all controller components
func (c *Controller) initComponents() error {
	if c.config.EnableSRIOV {
		c.sriovManager = hardware.NewSRIOVManager(c.ctx, c.clientset, c.logger)
	}

	// others will come

	return nil
}

// Start launches all controller components
func (c *Controller) Start() error {
	c.logger.Info("Starting NSM Controller")

	// validate hardware capabilities
	if err := c.validateHardware(); err != nil {
		c.logger.Warn("Hardware validation failed, continuing with reduced capabilities")
	}

	// Start SR-IOV manager if enabled
	if c.sriovManager != nil {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			if err := c.sriovManager.Start(); err != nil {
				c.logger.WithError(err).Error(("SR-IOV manager failed"))
			}
		}()
		c.logger.Info("Started SR-IOV manager")
	}

	c.logger.Info("All components started successfully")
	return nil
}

// Stop gracefully shuts down all controller components
func (c *Controller) Stop() error {
	c.logger.Info("Stopping NSM Controller")

	// signal all goroutines to stop
	c.cancel()

	// wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("All components stopped gracefully")
	case <-time.After(30 * time.Second):
		c.logger.Warn("Timeout waiting for components to stop")
	}

	return nil
}

// validateHardware checks if the hardware meets the requirements
func (c *Controller) validateHardware() error {
	var errors []error

	// validate SR-IOV if enabled
	if c.config.EnableSRIOV {
		if err := hardware.ValidateSRIOVCapabilities(); err != nil {
			errors = append(errors, fmt.Errorf("SR-IOV validation failed: %w", err))
		}
	}

	// TODO for DPDK

	if len(errors) > 0 {
		return fmt.Errorf("hardware validation failed: %v", errors)
	}

	return nil
}
