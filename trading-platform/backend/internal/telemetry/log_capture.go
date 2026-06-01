package telemetry

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// StreamPodLogs follows the stdout/stderr of the named pod and publishes each
// line as a TelemetryEvent with Action="LOG" until the context is cancelled.
// It retries on EOF to handle pods that haven't fully started yet.
func StreamPodLogs(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	namespace, podName, submissionID string,
	redisClient *redis.Client,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
			Follow: true,
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			// If pod isn't found/ready, back off and retry
			select {
			case <-time.After(1 * time.Second):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			event := TelemetryEvent{
				SubmissionID: submissionID,
				Action:       "LOG",
				EngineOutput: Truncate512(line),
				Timestamp:    time.Now().UTC(),
			}
			pubCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			_ = PublishEvent(pubCtx, redisClient, event)
			cancel()
		}

		stream.Close()

		if err := scanner.Err(); err != nil && err != io.EOF {
			return err
		}

		// If we hit EOF but the context isn't done, the pod might have restarted
		// or is still initializing. Back off slightly and retry.
		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
