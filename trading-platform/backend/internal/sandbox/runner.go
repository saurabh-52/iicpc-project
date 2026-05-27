// backend/internal/sandbox/runner.go
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
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

func ExecuteCode(filePath string, language string, port int) (ExecutionResult, error) {
	ctx := context.Background()
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

	podID, serviceName, err := createSandboxPod(ctx, clientset, filepath.Base(absPath), string(sourceBytes), spec, port)
	if err != nil {
		return ExecutionResult{}, err
	}

	// Wait for the pod to be Ready (engine compiled and listening) instead of
	// waiting for it to terminate.  Trading engines are long-running servers
	// and will never reach the Succeeded phase on their own.
	phase, waitErr := waitForPodReady(ctx, clientset, "trading-sandbox", podID, 60*time.Second)

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
	}

	if npErr != nil {
		fmt.Printf("Warning: could not retrieve NodePort for %s: %v\n", serviceName, npErr)
	}

	fmt.Printf("Kubernetes pod %s — phase: %s, NodePort: %d\n", podID, phase, nodePort)
	return ExecutionResult{
		PodID:       podID,
		ServiceName: serviceName,
		Phase:       phase,
		Output:      strings.TrimSpace(output),
		NodePort:    nodePort,
	}, nil
}

func createSandboxPod(ctx context.Context, clientset *kubernetes.Clientset, fileName string, sourceCode string, spec sandboxSpec, port int) (string, string, error) {
	namespace := "trading-sandbox"
	podName := fmt.Sprintf("sandbox-%d-%d", port, time.Now().Unix())
	serviceName := podName + "-svc"
	configMapName := podName + "-code"
	labels := map[string]string{
		"app":  "trading-sandbox",
		"pod":  podName,
		"port": strconv.Itoa(port),
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
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
					Image:   spec.image,
					Command: []string{"sh", "-lc"},
					Args:    []string{spec.command},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: int32(port),
							Protocol:      corev1.ProtocolTCP,
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
						InitialDelaySeconds: 3,
						PeriodSeconds:       2,
						TimeoutSeconds:      1,
						FailureThreshold:    25,
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
		return "", "", fmt.Errorf("failed to create pod: %v", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
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
			Type: corev1.ServiceTypeNodePort,
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

// CleanupSandbox deletes the pod, service, and configmap created for a sandbox
// identified by podID.  Best-effort: collects all errors rather than failing
// on the first one.
func CleanupSandbox(podID string) error {
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
		errs = append(errs, fmt.Sprintf("pod: %v", err))
	}
	if err := clientset.CoreV1().Services(namespace).Delete(ctx, serviceName, metav1.DeleteOptions{}); err != nil {
		errs = append(errs, fmt.Sprintf("service: %v", err))
	}
	if err := clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{}); err != nil {
		errs = append(errs, fmt.Sprintf("configmap: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
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
			image:   "golang:1.25",
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
			image:   "python:3.12-slim",
			command: fmt.Sprintf("python /app/%s", fileName),
			port:    port,
		}, nil
	default:
		return sandboxSpec{}, fmt.Errorf("unsupported language: %s", language)
	}
}
