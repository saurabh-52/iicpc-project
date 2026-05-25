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
}

func getKubernetesConfig() (*rest.Config, error) {
	// Use kubeconfig only; the backend is run outside the cluster.
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

	phase, waitErr := waitForPodCompletion(ctx, clientset, "trading-sandbox", podID, 45*time.Second)
	output, logErr := getPodLogs(ctx, clientset, "trading-sandbox", podID)

	if waitErr != nil {
		if logErr == nil && strings.TrimSpace(output) != "" {
			output = strings.TrimSpace(output) + "\nwait_error: " + waitErr.Error()
		} else if logErr != nil {
			output = "wait_error: " + waitErr.Error() + " ; log_error: " + logErr.Error()
		} else {
			output = "wait_error: " + waitErr.Error()
		}
		phase = "Unknown"
	}

	fmt.Printf("Kubernetes pod started successfully with ID: %s\n", podID)
	return ExecutionResult{
		PodID:       podID,
		ServiceName: serviceName,
		Phase:       phase,
		Output:      strings.TrimSpace(output),
	}, nil
}

func createSandboxPod(ctx context.Context, clientset *kubernetes.Clientset, fileName string, sourceCode string, spec sandboxSpec, port int) (string, string, error) {
	namespace := "trading-sandbox"
	podName := fmt.Sprintf("sandbox-%d-%d", port, time.Now().Unix())
	serviceName := podName + "-svc"
	configMapName := podName + "-code"
	labels := map[string]string{
		"app":  "trading-sandbox",
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
			Selector: map[string]string{
				"app": "trading-sandbox",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       int32(port),
					TargetPort: intstr.FromInt(int(port)),
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

func waitForPodCompletion(ctx context.Context, clientset *kubernetes.Clientset, namespace string, podName string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return "Unknown", fmt.Errorf("failed to get pod status: %v", err)
		}

		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return string(pod.Status.Phase), nil
		}

		time.Sleep(1 * time.Second)
	}

	return "Running", fmt.Errorf("timed out waiting for pod completion after %s", timeout)
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
