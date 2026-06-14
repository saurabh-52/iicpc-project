// backend/internal/sandbox/runner.go
package sandbox

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type sandboxSpec struct {
	image   string
	command string
	port    int
}

type ExecutionResult struct {
	PodID       string `json:"pod_id"`
	ServiceName string `json:"service_name"`
	Phase       string `json:"phase"`
	Output      string `json:"output"`
	NodePort    int32  `json:"node_port"`
	// ExternalIP is the LoadBalancer IP assigned by minikube tunnel (e.g. 127.0.0.1).
	// Empty when running in-cluster (DNS is used instead) or before tunnel assigns it.
	ExternalIP  string `json:"external_ip,omitempty"`
	// LocalPort is the host-side port of the kubectl port-forward process.
	// When non-zero, traffic should be sent to 127.0.0.1:LocalPort.
	LocalPort   int    `json:"local_port,omitempty"`
}

// portForwardRegistry tracks active kubectl port-forward processes keyed by pod ID.
var portForwardRegistry = struct {
	mu    sync.Mutex
	procs map[string]*exec.Cmd
}{
	procs: make(map[string]*exec.Cmd),
}

// startPortForward starts `kubectl port-forward pod/<podID> localPort:containerPort`
// and waits until the local port is reachable (up to 10 s).
// It returns the chosen local port so callers can build the target address.
func startPortForward(namespace, podID string, containerPort int) (int, error) {
	// Try to bind to the same port as the container first (simpler target URL).
	// Fall back to a random free port if that port is busy on the host.
	localPort := containerPort
	if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort)); err == nil {
		ln.Close() // port is free, we'll use it
	} else {
		// container port is busy on host — pick any free port
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, fmt.Errorf("cannot find free port: %v", err)
		}
		localPort = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
	}

	kubeconfigPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	args := []string{
		"port-forward",
		"-n", namespace,
		"pod/" + podID,
		fmt.Sprintf("%d:%d", localPort, containerPort),
	}
	if kubeconfigPath != "" {
		args = append(args, "--kubeconfig="+kubeconfigPath)
	}

	cmd := exec.Command("kubectl", args...)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start kubectl port-forward: %v", err)
	}

	portForwardRegistry.mu.Lock()
	portForwardRegistry.procs[podID] = cmd
	portForwardRegistry.mu.Unlock()

	// Wait until the local port is actually accepting connections (up to 10 s).
	addr := fmt.Sprintf("127.0.0.1:%d", localPort)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			fmt.Printf("kubectl port-forward ready: %s → pod/%s:%d\n", addr, podID, containerPort)
			return localPort, nil
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Port-forward started but isn't accepting yet — still return the port.
	// waitForTargetReachable will continue probing.
	fmt.Printf("kubectl port-forward started (not yet ready): %s\n", addr)
	return localPort, nil
}

// stopPortForward kills the kubectl port-forward process for the given pod ID.
func stopPortForward(podID string) {
	portForwardRegistry.mu.Lock()
	cmd, ok := portForwardRegistry.procs[podID]
	delete(portForwardRegistry.procs, podID)
	portForwardRegistry.mu.Unlock()
	if ok && cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

// InCluster reports whether the backend is running inside a Kubernetes cluster.
func InCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}

func getKubernetesConfig() (*rest.Config, error) {
	if config, err := rest.InClusterConfig(); err == nil {
		fmt.Println("Using in-cluster Kubernetes config")
		return config, nil
	}

	// Fall back to kubeconfig when running the backend locally.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %v", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %v", kubeconfig, err)
	}

	fmt.Printf("Using kubeconfig: %s\n", kubeconfig)
	return config, nil
}

func ExecuteCode(ctx context.Context, filePath string, language string, port int, systemName string) (ExecutionResult, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to get absolute path: %v", err)
	}

	spec, err := buildSandboxSpec(absPath, language, port)
	if err != nil {
		return ExecutionResult{}, err
	}

	config, err := getKubernetesConfig()
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to create Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	sourceBytes, err := os.ReadFile(absPath)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("failed to read source file: %v", err)
	}
	fmt.Printf("runner.go: read %d bytes from source file %s\n", len(sourceBytes), absPath)

	podID, serviceName, err := createSandboxPod(ctx, clientset, filepath.Base(absPath), string(sourceBytes), spec, port, systemName)
	if err != nil {
		return ExecutionResult{}, err
	}

	// Wait for the pod to be Ready (engine compiled and listening) instead of
	// waiting for it to terminate.  Trading engines are long-running servers
	// and will never reach the Succeeded phase on their own.
	phase, waitErr := waitForPodReady(ctx, clientset, "trading-sandbox", podID, 120*time.Second)

	// Collect whatever logs are available (startup output, compilation errors).
	output, logErr := getPodLogs(ctx, clientset, "trading-sandbox", podID)

	// Retrieve the NodePort so callers can reach the engine.
	nodePort, npErr := getServiceNodePort(ctx, clientset, "trading-sandbox", serviceName)

	if waitErr != nil {
		if logErr == nil && strings.TrimSpace(output) != "" {
			output = strings.TrimSpace(output) + "\nwait_error: " + waitErr.Error()
		} else if logErr != nil {
			output = "wait_error: " + waitErr.Error() + " ; log_error: " + logErr.Error()
		} else {
			output = "wait_error: " + waitErr.Error()
		}
		if phase == "" {
			phase = "Unknown"
		}
		// Clean up the failed/pending sandbox resources immediately so the port is freed
		fmt.Printf("Cleaning up failed sandbox pod %s (phase: %s)\n", podID, phase)
		_ = CleanupSandbox(podID)

		return ExecutionResult{
			PodID:       podID,
			ServiceName: serviceName,
			Phase:       phase,
			Output:      strings.TrimSpace(output),
			NodePort:    nodePort,
		}, waitErr
	} else if phase == "Running" {
		// Start a background watcher to clean up resources immediately if the pod terminates after it started successfully
		go watchAndCleanupPod(clientset, "trading-sandbox", podID)
	}

	if npErr != nil {
		fmt.Printf("Warning: could not retrieve NodePort for %s: %v\n", serviceName, npErr)
	}

	// Wait for minikube tunnel to assign a LoadBalancer external IP (up to 45s).
	// On Windows with Docker driver, `minikube tunnel` assigns 127.0.0.1 as the
	// external IP for LoadBalancer services, making them reachable from the host.
	// We poll here because tunnel provisioning happens asynchronously after service creation.
	//
	// REPLACED: minikube tunnel is unreliable on Windows Docker driver.
	// Instead, use kubectl port-forward for guaranteed host→pod connectivity.
	localPort, pfErr := startPortForward("trading-sandbox", podID, port)
	if pfErr != nil {
		fmt.Printf("Warning: kubectl port-forward failed: %v — will fall back to 127.0.0.1:%d\n", pfErr, port)
		localPort = port
	}

	fmt.Printf("Kubernetes pod %s — phase: %s, NodePort: %d, LocalPort: %d\n", podID, phase, nodePort, localPort)
	return ExecutionResult{
		PodID:       podID,
		ServiceName: serviceName,
		Phase:       phase,
		Output:      strings.TrimSpace(output),
		NodePort:    nodePort,
		ExternalIP:  "127.0.0.1",
		LocalPort:   localPort,
	}, nil
}

func createSandboxPod(ctx context.Context, clientset *kubernetes.Clientset, fileName string, sourceCode string, spec sandboxSpec, port int, systemName string) (string, string, error) {
	namespace := "trading-sandbox"
	sanitizedSystem := sanitizeDNSName(systemName)
	if sanitizedSystem == "" {
		sanitizedSystem = "default"
	}
	podName := fmt.Sprintf("sb-%s-%d-%s", sanitizedSystem, time.Now().Unix(), randomString(4))
	serviceName := podName + "-svc"
	configMapName := podName + "-code"

	// Auto-cleanup: remove only the old sandbox pods/services/configmaps belonging to this systemName
	cleanupUserSandboxes(ctx, clientset, namespace, systemName)

	labels := map[string]string{
		"app":         "trading-sandbox",
		"pod":         podName,
		"port":        strconv.Itoa(port),
		"system-name": sanitizedSystem,
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			fileName: sourceCode,
		},
	}

	_, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to create configmap: %v", err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "sandbox",
					Image:           spec.image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"sh", "-lc"},
					Args:            []string{spec.command},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: int32(port),
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					// TCP readiness probe: the pod is considered Ready only
					// after the engine has compiled and started listening on
					// the configured port.
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(port),
							},
						},
						InitialDelaySeconds: 5,
						PeriodSeconds:       2,
						TimeoutSeconds:      1,
						FailureThreshold:    60,
						SuccessThreshold:    1,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "code",
							MountPath: "/app",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "code",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	createdPod, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		_ = clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
		return "", "", fmt.Errorf("failed to create pod: %v", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			// Select the specific sandbox pod so different sandboxes don't
			// all become endpoints for every service.
			Selector: map[string]string{
				"app": "trading-sandbox",
				"pod": podName,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       int32(port),
					TargetPort: intstr.FromInt(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			// LoadBalancer type: with `minikube tunnel` running, Kubernetes assigns
			// ExternalIP=127.0.0.1 to this service, making it reachable from the
			// Windows host at 127.0.0.1:<containerPort> without any extra port-forward.
			// NodePort was tried but minikube tunnel does NOT expose NodePorts on
			// the host — only LoadBalancer services get the 127.0.0.1 external IP.
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	_, err = clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		_ = clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		_ = clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
		return "", "", fmt.Errorf("failed to create service: %v", err)
	}

	return createdPod.Name, serviceName, nil
}

// waitForPodReady polls the pod status until it reaches the Running phase with
// a Ready condition (meaning the readiness probe has passed and the engine is
// actively listening on its port).  If the pod enters the Failed phase the
// function returns immediately with an error.
func waitForPodReady(ctx context.Context, clientset *kubernetes.Clientset, namespace string, podName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "Unknown", fmt.Errorf("failed to get pod status: %v", err)
		}

		switch pod.Status.Phase {
		case corev1.PodFailed:
			return "Failed", fmt.Errorf("pod entered Failed phase")
		case corev1.PodSucceeded:
			// Unexpected for a long-running server, but handle gracefully.
			return "Succeeded", nil
		case corev1.PodRunning:
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return "Running", nil
				}
			}
		}

		time.Sleep(2 * time.Second)
	}

	return "Pending", fmt.Errorf("timed out waiting for pod to become ready after %s", timeout)
}

// getServiceNodePort fetches the NodePort allocated by Kubernetes for a service.
func getServiceNodePort(ctx context.Context, clientset *kubernetes.Clientset, namespace, serviceName string) (int32, error) {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to get service %s: %v", serviceName, err)
	}
	for _, p := range svc.Spec.Ports {
		if p.NodePort > 0 {
			return p.NodePort, nil
		}
	}
	return 0, fmt.Errorf("no NodePort found on service %s", serviceName)
}

// waitForServiceExternalIP polls a LoadBalancer service until minikube tunnel
// assigns an external IP (e.g. 127.0.0.1 on Windows Docker driver) or the
// timeout expires.  This is necessary because `minikube tunnel` provisions the
// external IP asynchronously, after the service is created.
func waitForServiceExternalIP(ctx context.Context, clientset *kubernetes.Clientset, namespace, serviceName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		svc, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err == nil {
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					return ingress.IP, nil
				}
				if ingress.Hostname != "" {
					return ingress.Hostname, nil
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("timeout waiting for LoadBalancer external IP on service %s after %s — is `minikube tunnel` running?", serviceName, timeout)
}


func getPodLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace string, podName string) (string, error) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open pod logs stream: %v", err)
	}
	defer stream.Close()

	content, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read pod logs: %v", err)
	}

	return string(content), nil
}

// cleanupOldSandboxes removes all existing sandbox pods, services, and configmaps
// in the namespace.  Best-effort: errors are logged but don't block new sandbox creation.
func cleanupOldSandboxes(ctx context.Context, clientset *kubernetes.Clientset, namespace string) {
	gracePeriod := int64(0)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	// Delete all pods with the sandbox label
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=trading-sandbox",
	})
	if err == nil {
		for _, pod := range podList.Items {
			_ = clientset.CoreV1().Pods(namespace).Delete(ctx, pod.Name, deleteOptions)
			fmt.Printf("Auto-cleanup: deleted old pod %s\n", pod.Name)
		}
	}

	// Delete all services with the sandbox label
	svcList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=trading-sandbox",
	})
	if err == nil {
		for _, svc := range svcList.Items {
			_ = clientset.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			fmt.Printf("Auto-cleanup: deleted old service %s\n", svc.Name)
		}
	}

	// Delete all sandbox configmaps
	cmList, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=trading-sandbox",
	})
	if err == nil {
		for _, cm := range cmList.Items {
			_ = clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
			fmt.Printf("Auto-cleanup: deleted old configmap %s\n", cm.Name)
		}
	}

	// Give K8s a moment to release resources
	time.Sleep(1 * time.Second)
}

// CleanupAllSandboxes deletes all sandbox pods, services, and configmaps.
// It is intended to run once at backend server startup.
func CleanupAllSandboxes(ctx context.Context) error {
	config, err := getKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	cleanupOldSandboxes(ctx, clientset, "trading-sandbox")
	return nil
}

// CleanupContestSandboxes deletes all sandbox pods, services, and configmaps
// that belong to a specific contest. It waits for pod termination confirmation
// to prevent NodePort conflicts on sequential runs.
func CleanupContestSandboxes(ctx context.Context, contestID string) error {
	config, err := getKubernetesConfig()
	if err != nil {
		return fmt.Errorf("cleanup contest sandboxes: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("cleanup contest sandboxes: %w", err)
	}

	namespace := "trading-sandbox"
	// Clean up all sandbox resources — contest pods don't carry a contest-id label
	// in the current schema, so we clean all sandboxes after contest finalization.
	cleanupOldSandboxes(ctx, clientset, namespace)

	// Wait for all pods to be fully terminated (up to 30s)
	if err := waitForPodsTerminated(ctx, clientset, namespace, 30*time.Second); err != nil {
		fmt.Printf("WARNING: some contest sandbox pods may not have fully terminated: %v\n", err)
		return err
	}
	fmt.Printf("Contest %s sandbox cleanup confirmed — all pods terminated\n", contestID)
	return nil
}

// waitForPodsTerminated polls until no sandbox pods remain in the namespace,
// or until the timeout expires. This prevents NodePort conflicts on sequential runs.
func waitForPodsTerminated(ctx context.Context, clientset *kubernetes.Clientset, namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=trading-sandbox",
		})
		if err != nil {
			return fmt.Errorf("failed to list pods: %v", err)
		}
		if len(podList.Items) == 0 {
			return nil
		}
		fmt.Printf("Waiting for %d sandbox pod(s) to terminate...\n", len(podList.Items))
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for sandbox pods to terminate after %s", timeout)
}


// cleanupUserSandboxes removes existing sandbox pods, services, and configmaps
// that match the given system name.
func cleanupUserSandboxes(ctx context.Context, clientset *kubernetes.Clientset, namespace string, systemName string) {
	sanitizedSystem := sanitizeDNSName(systemName)
	if sanitizedSystem == "" {
		return
	}

	selector := fmt.Sprintf("system-name=%s", sanitizedSystem)

	gracePeriod := int64(0)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	// Delete all pods with the system-name label
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err == nil {
		for _, pod := range podList.Items {
			_ = clientset.CoreV1().Pods(namespace).Delete(ctx, pod.Name, deleteOptions)
			fmt.Printf("Auto-cleanup: deleted old pod %s for system %s\n", pod.Name, systemName)
		}
	}

	// Delete all services with the system-name label
	svcList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err == nil {
		for _, svc := range svcList.Items {
			_ = clientset.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			fmt.Printf("Auto-cleanup: deleted old service %s for system %s\n", svc.Name, systemName)
		}
	}

	// Delete all configmaps with the system-name label
	cmList, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err == nil {
		for _, cm := range cmList.Items {
			_ = clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
			fmt.Printf("Auto-cleanup: deleted old configmap %s for system %s\n", cm.Name, systemName)
		}
	}

	// Give K8s a moment to release resources and wait for deletion to complete
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		pods, err1 := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		svcs, err2 := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		
		podCount := 0
		if err1 == nil {
			podCount = len(pods.Items)
		}
		svcCount := 0
		if err2 == nil {
			svcCount = len(svcs.Items)
		}
		
		if podCount == 0 && svcCount == 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// CleanupUserSandboxes removes existing sandbox pods, services, and configmaps
// that match the given system name.
func CleanupUserSandboxes(ctx context.Context, systemName string) error {
	config, err := getKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	cleanupUserSandboxes(ctx, clientset, "trading-sandbox", systemName)
	return nil
}

// sanitizeDNSName sanitizes a string to be a valid DNS subdomain name and K8s label value.
func sanitizeDNSName(s string) string {
	var sb strings.Builder
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			sb.WriteRune(r)
		} else if r == '-' {
			if sb.Len() > 0 {
				sb.WriteRune(r)
			}
		}
	}
	res := sb.String()
	// trim trailing hyphens
	for len(res) > 0 && res[len(res)-1] == '-' {
		res = res[:len(res)-1]
	}
	if len(res) > 50 {
		res = res[:50]
	}
	return strings.ToLower(res)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			b[i] = letters[0]
		} else {
			b[i] = letters[num.Int64()]
		}
	}
	return string(b)
}

// CleanupSandbox deletes the pod, service, and configmap created for a sandbox
// identified by podID.  Best-effort: collects all errors rather than failing
// on the first one.
func CleanupSandbox(podID string) error {
	// Kill the kubectl port-forward process for this pod (if any).
	stopPortForward(podID)

	ctx := context.Background()
	config, err := getKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	namespace := "trading-sandbox"
	serviceName := podID + "-svc"
	configMapName := podID + "-code"

	var errs []string

	if err := clientset.CoreV1().Pods(namespace).Delete(ctx, podID, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			errs = append(errs, fmt.Sprintf("pod: %v", err))
		}
	}
	if err := clientset.CoreV1().Services(namespace).Delete(ctx, serviceName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			errs = append(errs, fmt.Sprintf("service: %v", err))
		}
	}
	if err := clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			errs = append(errs, fmt.Sprintf("configmap: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CleanupSandboxByTarget cleans up the sandbox pod, service, and configmap associated with the given target URL.
func CleanupSandboxByTarget(ctx context.Context, target string) error {
	urlStr := target
	if strings.HasPrefix(urlStr, "http://") {
		urlStr = urlStr[7:]
	} else if strings.HasPrefix(urlStr, "https://") {
		urlStr = urlStr[8:]
	}

	host, portStr, err := net.SplitHostPort(urlStr)
	if err != nil {
		host = urlStr
	}

	config, err := getKubernetesConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	namespace := "trading-sandbox"
	var podID string

	if strings.Contains(host, "trading-sandbox.svc.cluster.local") {
		svcHost := strings.Split(host, ".")[0]
		podID = strings.TrimSuffix(svcHost, "-svc")
	} else if host == "127.0.0.1" || host == "localhost" || net.ParseIP(host) != nil {
		if portStr != "" {
			portVal, err := strconv.Atoi(portStr)
			if err == nil {
				svcList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
					LabelSelector: "app=trading-sandbox",
				})
				if err == nil {
					for _, svc := range svcList.Items {
						for _, p := range svc.Spec.Ports {
							if p.NodePort == int32(portVal) || p.Port == int32(portVal) {
								podID = strings.TrimSuffix(svc.Name, "-svc")
								break
							}
						}
						if podID != "" {
							break
						}
					}
				}
			}
		}
	} else {
		podID = strings.TrimSuffix(host, "-svc")
	}

	if podID == "" {
		return fmt.Errorf("could not identify sandbox pod for target %s", target)
	}

	return CleanupSandbox(podID)
}

func buildSandboxSpec(absPath string, language string, port int) (sandboxSpec, error) {
	fileName := filepath.Base(absPath)
	normalizedLanguage := strings.ToLower(strings.TrimSpace(language))

	switch normalizedLanguage {
	case "cpp", "c++", "cc", "cxx":
		return sandboxSpec{
			image:   "gcc:latest",
			command: fmt.Sprintf("g++ /app/%s -o /tmp/run && /tmp/run", fileName),
			port:    port,
		}, nil
	case "go":
		return sandboxSpec{
			image:   "golang:latest",
			command: fmt.Sprintf("/usr/local/go/bin/go run /app/%s", fileName),
			port:    port,
		}, nil
	case "rust":
		return sandboxSpec{
			image:   "rust:latest",
			command: fmt.Sprintf("/usr/local/cargo/bin/rustc /app/%s -o /tmp/run && /tmp/run", fileName),
			port:    port,
		}, nil
	case "python", "py":
		return sandboxSpec{
			image:   "python:latest",
			command: fmt.Sprintf("python /app/%s", fileName),
			port:    port,
		}, nil
	default:
		return sandboxSpec{}, fmt.Errorf("unsupported language: %s", language)
	}
}

// GetSandboxTargetURL finds the running sandbox pod/service for a given system name and returns its target URL.
func GetSandboxTargetURL(ctx context.Context, systemName string, protocol string) (string, error) {
	config, err := getKubernetesConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get k8s config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to get k8s clientset: %v", err)
	}

	sanitizedSystem := sanitizeDNSName(systemName)
	if sanitizedSystem == "" {
		return "", fmt.Errorf("invalid system name")
	}

	selector := fmt.Sprintf("system-name=%s", sanitizedSystem)
	namespace := "trading-sandbox"
	
	// Ensure pod is actually running and not in a crash loop
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(podList.Items) == 0 {
		return "", fmt.Errorf("no sandbox pod found for system %s", systemName)
	}
	
	isRunning := false
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			isRunning = true
			break
		}
	}
	if !isRunning {
		return "", fmt.Errorf("no running sandbox pod found for system %s", systemName)
	}

	svcList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil || len(svcList.Items) == 0 {
		return "", fmt.Errorf("no sandbox service found for system %s", systemName)
	}

	svc := svcList.Items[0]

	// Get port from service
	var servicePort int32 = 8080
	for _, p := range svc.Spec.Ports {
		if p.Port > 0 {
			servicePort = p.Port
			break
		}
	}

	proto := strings.ToLower(strings.TrimSpace(protocol))
	isTCP := proto == "tcp" || proto == "fix"

	if InCluster() {
		host := fmt.Sprintf("%s.trading-sandbox.svc.cluster.local", svc.Name)
		if isTCP {
			return fmt.Sprintf("%s:%d", host, servicePort), nil
		}
		return fmt.Sprintf("http://%s:%d", host, servicePort), nil
	}

	// If the service has a LoadBalancer External IP (e.g. from minikube tunnel),
	// use the External IP and the service port directly.
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		ingressIP := svc.Status.LoadBalancer.Ingress[0].IP
		if ingressIP != "" {
			if isTCP {
				return fmt.Sprintf("%s:%d", ingressIP, servicePort), nil
			}
			return fmt.Sprintf("http://%s:%d", ingressIP, servicePort), nil
		}
	}

	var nodePort int32
	for _, p := range svc.Spec.Ports {
		if p.NodePort > 0 {
			nodePort = p.NodePort
			break
		}
	}

	if nodePort == 0 {
		return "", fmt.Errorf("no node port assigned to service %s", svc.Name)
	}

	hostIP := "127.0.0.1"
	if minikubeIP := os.Getenv("MINIKUBE_IP"); minikubeIP != "" {
		hostIP = minikubeIP
	}

	if isTCP {
		return fmt.Sprintf("%s:%d", hostIP, nodePort), nil
	}
	return fmt.Sprintf("http://%s:%d", hostIP, nodePort), nil
}

func watchAndCleanupPod(clientset *kubernetes.Clientset, namespace string, podName string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Give the pod a moment to start up
	time.Sleep(3 * time.Second)

	for range ticker.C {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				return
			}
			continue
		}

		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			fmt.Printf("Watcher: Pod %s entered terminal phase (%s). Cleaning up service and resources...\n", podName, pod.Status.Phase)
			_ = CleanupSandbox(podName)
			return
		}
	}
}
