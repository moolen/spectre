// Package helpers provides port-forward utilities for e2e testing.
package helpers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwarder manages a port-forward to a Kubernetes service.
type PortForwarder struct {
	LocalPort  uint16
	Namespace  string
	Service    string
	RemotePort uint16
	stopCh     chan struct{}
	readyCh    chan struct{}
	t          *testing.T
}

// NewPortForwarder creates a new port forwarder.
func NewPortForwarder(t *testing.T, kubeConfig, namespace, service string, remotePort uint16) (*PortForwarder, error) {
	t.Logf("Setting up port-forward for %s/%s:%d", namespace, service, remotePort)

	// Find a free local port
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find free port: %w", err)
	}

	localPort := uint16(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()

	pf := &PortForwarder{
		LocalPort:  localPort,
		Namespace:  namespace,
		Service:    service,
		RemotePort: remotePort,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		t:          t,
	}

	// Start port-forward in background
	go func() {
		if err := pf.run(kubeConfig); err != nil {
			t.Logf("Port-forward error: %v", err)
		}
	}()

	// Wait for port-forward to be ready
	select {
	case <-pf.readyCh:
		t.Logf("✓ Port-forward established on localhost:%d -> %s/%s:%d", localPort, namespace, service, remotePort)
		return pf, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout waiting for port-forward to be ready")
	}
}

// GetURL returns the local URL for the forwarded service.
func (pf *PortForwarder) GetURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", pf.LocalPort)
}

// Stop closes the port-forward.
func (pf *PortForwarder) Stop() error {
	pf.t.Logf("Stopping port-forward on localhost:%d", pf.LocalPort)
	close(pf.stopCh)
	// Give it a moment to clean up
	time.Sleep(500 * time.Millisecond)
	return nil
}

// run executes the port-forward.
func (pf *PortForwarder) run(kubeConfigPath string) error {
	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get pods in the namespace with service selector
	pods, err := clientset.CoreV1().Pods(pf.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found in namespace %s", pf.Namespace)
	}

	// Use first pod (typically there's only one for services)
	pod := pods.Items[0]

	// Prepare the port forward request
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pf.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	ports := []string{fmt.Sprintf("%d:%d", pf.LocalPort, pf.RemotePort)}
	fw, err := portforward.New(dialer, ports, pf.stopCh, pf.readyCh, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	return fw.ForwardPorts()
}

// WaitForReady waits until the port-forwarded service is responsive.
func (pf *PortForwarder) WaitForReady(timeout time.Duration) error {
	pf.t.Logf("Waiting for service to be ready at %s", pf.GetURL())

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for service at %s", pf.GetURL())
		case <-ticker.C:
			resp, err := http.Get(pf.GetURL() + "/health")
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					pf.t.Logf("✓ Service is ready")
					return nil
				}
			}
		}
	}
}
