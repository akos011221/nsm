package hardware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SRIOVManager manages SR-IOV Virtual Functions
type SRIOVManager struct {
	// Context for cancellation
	ctx context.Context
	// Kubernetes client
	clientset *kubernetes.Clientset
	// Logger
	logger *logrus.Logger
	// Available VF inventory
	vfInventory map[string]VirtualFunction
	// Mutex for protecting the inventory
	mu sync.RWMutex
	// Poll interval for VF discovery
	pollInterval time.Duration
}

// VirtualFunction represents an SR-IOV Virtual Function
type VirtualFunction struct {
	// PF name (e.g., eth0)
	PFName string
	// VF ID (e.g., 0)
	VFID int
	// VF PCI address
	PCIAddress string
	// VF interface name if bound to network driver
	InterfaceName string
	// Whether the VF is allocated
	Allocated bool
	// Pod using this VF, if any
	AllocatedTo string
	// Namespace of the pod using this VF
	Namespace string
}

// NewSRIOVManager creates a new SR-IOV manager
func NewSRIOVManager(ctx context.Context, clientset *kubernetes.Clientset, logger *logrus.Logger) *SRIOVManager {
	return &SRIOVManager{
		ctx:          ctx,
		clientset:    clientset,
		logger:       logger,
		vfInventory:  make(map[string]VirtualFunction),
		pollInterval: 30 * time.Second,
	}
}

// Start begins the SR-IOV manager's operation
func (m *SRIOVManager) Start() error {
	m.logger.Info("Starting SR-IOV Manager")

	// initial discovery of VFs
	if err := m.discoverVirtualFunctions(); err != nil {
		m.logger.WithError(err).Error("Initial VF discovery failed")
	}

	// start periodic discovery
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// rediscover VFs
			if err := m.discoverVirtualFunctions(); err != nil {
				m.logger.WithError(err).Error("VF discovery failed")
				continue
			}

			// reconcile VF allocations
			if err := m.reconcileAllocations(); err != nil {
				m.logger.WithError(err).Error("VF allocation reconciliation failed")
			}

		case <-m.ctx.Done():
			m.logger.Info("Stopping SR-IOV Manager")
			return nil
		}
	}
}

// ValidateSRIOVCapabilities checks if the system supports SR-IOV
func ValidateSRIOVCapabilities() error {
	// does the sriov_numvfs file exists for any network device?
	// TODO: use more robust validation
	devices, err := filepath.Glob("sys/class/net/*/device/sriov_numvfs")
	if err != nil {
		return fmt.Errorf("failed to glob for sriov devices: %w", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("no SR-IOV capable devices found")
	}

	return nil
}

// discoverVirtualFunctions scans the system for SR-IOV Virtual Functions
func (m *SRIOVManager) discoverVirtualFunctions() error {
	// temp inventory for the newly discovered VFs (avoids race conditions)
	newInventory := make(map[string]VirtualFunction)

	// find all network devices (in linux sysfs)
	devices, err := filepath.Glob("/sys/class/net/*")
	if err != nil {
		return fmt.Errorf("failed to glob network devices: %w", err)
	}

	// for each device, check if it's a physical function
	for _, devicePath := range devices {
		// (e.g., "eth0" from "/sys/class/net/eth0")
		pfName := filepath.Base(devicePath)

		// skip virtual devices (e.g., Docker bridges, veth pairs)
		if strings.HasPrefix(pfName, "docker") || strings.HasPrefix(pfName, "veth") {
			continue
		}

		// does the devices has SR-IOV capability?
		numVFsPath := filepath.Join(devicePath, "device/sriov_numvfs")
		if _, err := os.Stat(numVFsPath); os.IsNotExist(err) {
			continue
		}

		// read number of configured VFs
		data, err := os.ReadFile(numVFsPath)
		if err != nil {
			m.logger.WithError(err).Warnf("Failed to read sriov_numvfs for %s", pfName)
		}

		numVFs, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			m.logger.WithError(err).Warnf("Failed to parse sriov_numvfs for %s", pfName)
			continue
		}

		if numVFs <= 0 {
			continue
		}

		// at this point, it's sure that this device has VFs
		m.logger.Debugf("Found %d VFs for device %s", numVFs, pfName)

		// get each VF's details
		for vfID := range numVFs {
			vf, err := m.getVFDetails(pfName, vfID)
			if err != nil {
				m.logger.WithError(err).Warnf("Failed to get details for VF %d of %s", vfID, pfName)
				continue
			}

			// create unique key for this VF
			vfKey := fmt.Sprintf("%s-vf%d", pfName, vfID)

			// check if this VF was previously allocated (thread-safe read)
			m.mu.RLock()
			if existingVF, exists := m.vfInventory[vfKey]; exists && existingVF.Allocated {
				vf.Allocated = existingVF.Allocated
				vf.AllocatedTo = existingVF.AllocatedTo
				vf.Namespace = existingVF.Namespace
			}
			m.mu.RUnlock()

			// add to new inventory
			newInventory[vfKey] = vf
		}
	}

	// update inventory (thread-safe write)
	m.mu.Lock()
	m.vfInventory = newInventory
	m.mu.Unlock()

	m.logger.WithField("vfCount", len(newInventory)).Info("SR-IOV VF discovery completed")
	return nil
}

// getVFDetails collects the details about a specific Virtual Function
func (m *SRIOVManager) getVFDetails(pfName string, vfID int) (VirtualFunction, error) {
	vf := VirtualFunction{
		PFName: pfName,
		VFID:   vfID,
	}

	// get PCI address
	pciPath := fmt.Sprintf("/sys/class/net/%s/device/virtfn%d/uevent", pfName, vfID)
	data, err := os.ReadFile(pciPath)
	if err != nil {
		return vf, fmt.Errorf("failed to read VF PCI into: %w", err)
	}

	// parse the uevent file to find PCI address
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
			vf.PCIAddress = strings.TrimPrefix(line, "PCI_SLOT_NAME=")
			break
		}
	}

	// try to find the network interface name for this VF
	// this is just a guess, as in real world, VF interface
	// names are not guaranteed to follow: pfname_vfX
	// if the guess is wrong, pods can't bind to it
	vf.InterfaceName = fmt.Sprintf("%s_vf%d", pfName, vfID)

	return vf, nil
}

// reconcileAllocations reconciles VF allocations with pods that request them
func (m *SRIOVManager) reconcileAllocations() error {
	// get pods that request SR-IOV
	pods, err := m.clientset.CoreV1().Pods("").List(m.ctx, metav1.ListOptions{
		LabelSelector: "network.nsm.akosrbn.io/sriov=true",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods requesting SR-IOV: %w", err)
	}

	m.logger.Debugf("Found %d pods requestion SR-IOV", len(pods.Items))

	// track allocated VFs
	allocatedVFs := make(map[string]bool)

	// first pass: check existing allocations
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, vf := range m.vfInventory {
		// check if the pod that was using this VF still exists
		podExists := false
		if vf.Allocated && vf.AllocatedTo != "" {
			for _, pod := range pods.Items {
				if pod.Name == vf.AllocatedTo && pod.Namespace == vf.Namespace {
					// pod still exists, keep allocation
					podExists = true
					allocatedVFs[key] = true
					break
				}
			}
		}

		if !podExists {
			// pod no longer exists, free the VF
			vf.Allocated = false
			vf.AllocatedTo = ""
			vf.Namespace = ""
			m.vfInventory[key] = vf
		}
	}

	// second pass: allocate VFs to pods that need them
	for _, pod := range pods.Items {
		// skip if pod is terminating
		if pod.DeletionTimestamp != nil {
			continue
		}

		// skip if pod already has a VF allocated
		alreadyAllocated := false
		for _, vf := range m.vfInventory {
			if vf.Allocated && vf.AllocatedTo == pod.Name && vf.Namespace == pod.Namespace {
				alreadyAllocated = true
				break
			}
		}

		if alreadyAllocated {
			continue
		}

		// find an available VF
		for key, vf := range m.vfInventory { // NOTE: vf is a copy, not a reference
			if !vf.Allocated {
				// allocate this VF to the pod
				vf.Allocated = true
				vf.AllocatedTo = pod.Name
				vf.Namespace = pod.Namespace
				m.vfInventory[key] = vf
				allocatedVFs[key] = true

				m.logger.Infof("Allocated VF %s to pod %s/%s", key, pod.Namespace, pod.Name)

				break
			}
		}
	}

	m.logger.Infof("VF allocation reconciliation completed: %d/%d VFs allocated",
		len(allocatedVFs), len(m.vfInventory))

	return nil
}

// Get VFForPod returns the allocated VF for a pod, if any.
func (m *SRIOVManager) GetVFForPod(namespace, podName string) (VirtualFunction, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, vf := range m.vfInventory {
		if vf.Allocated && vf.AllocatedTo == podName && vf.Namespace == namespace {
			return vf, true
		}
	}

	return VirtualFunction{}, false
}

// ReleaseVF releases a VF allocation
func (m *SRIOVManager) ReleaseVF(namespace, podName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, vf := range m.vfInventory { // NOTE: vf is a copy, not a reference
		if vf.Allocated && vf.AllocatedTo == podName && vf.Namespace == namespace {
			vf.Allocated = false
			vf.AllocatedTo = ""
			vf.Namespace = ""
			m.vfInventory[key] = vf

			m.logger.Infof("Released VF %s from pod %s/%s", key, namespace, podName)
			return true
		}
	}

	return false
}
