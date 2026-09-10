package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Debajyoti0-0/aether/pkg/plugins/sdk"
)

// kubernetesProvider implements sdk.Provider for Kubernetes clusters:
// service-account token validation, namespace listing, and pod staging
// for exec with a stolen SA token.
type kubernetesProvider struct {
	token     string
	base      string // https://<api-server>:6443
	namespace string
	http      *http.Client
}

// New builds a Kubernetes provider plugin from a service-account token
// (e.g., a compromised pod's mounted token).
func New(apiServer, token, namespace string) sdk.Plugin {
	return sdk.NewAdapter("kubernetes", "1.0.0", &kubernetesProvider{
		token:     token,
		base:      strings.TrimRight(apiServer, "/"),
		namespace: namespace,
		http:      &http.Client{Timeout: 15 * time.Second},
	})
}

func (k *kubernetesProvider) Name() string { return "kubernetes" }

// ValidateToken checks the service-account token via SelfSubjectReview.
func (k *kubernetesProvider) ValidateToken(ctx context.Context) error {
	body, err := k.api(ctx, http.MethodPost, "/apis/authentication.k8s.io/v1/selfsubjectreviews", `{"apiVersion":"authentication.k8s.io/v1","kind":"SelfSubjectReview"}`)
	if err != nil {
		return err
	}
	var review map[string]any
	if err := json.Unmarshal(body, &review); err != nil {
		return err
	}
	if _, ok := review["status"]; !ok {
		return fmt.Errorf("kubernetes token validation failed")
	}
	return nil
}

// K8sPod is a minimal pod record.
type K8sPod struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ServiceAc string `json:"service_account"`
}

// ListPods enumerates pods in the configured (or given) namespace.
func (k *kubernetesProvider) ListPods(ctx context.Context, namespace string) ([]K8sPod, error) {
	path := "/api/v1/pods"
	if namespace == "" {
		namespace = k.namespace
	}
	if namespace != "" && namespace != "all" {
		path = "/api/v1/namespaces/" + namespace + "/pods"
	}

	body, err := k.api(ctx, http.MethodGet, path, "")
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				ServiceAccountName string `json:"serviceAccountName"`
			} `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}

	pods := make([]K8sPod, 0, len(list.Items))
	for _, p := range list.Items {
		pods = append(pods, K8sPod{Name: p.Metadata.Name, Namespace: p.Metadata.Namespace, ServiceAc: p.Spec.ServiceAccountName})
	}
	return pods, nil
}

// Execute stages exec against a target pod using the SA token context.
// target = "namespace/pod", command = intended command.
func (k *kubernetesProvider) Execute(ctx context.Context, target, command string) (*sdk.Result, error) {
	parts := strings.SplitN(target, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("target must be namespace/pod, got %q", target)
	}

	body, err := k.api(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", parts[0], parts[1]), "")
	if err != nil {
		return nil, err
	}

	var pod struct {
		Spec struct {
			ServiceAccountName string `json:"serviceAccountName"`
			NodeName           string `json:"nodeName"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(body, &pod); err != nil {
		return nil, err
	}

	return &sdk.Result{
		Output: fmt.Sprintf("pod %s/%s (sa=%s node=%s) staged for exec: %s",
			parts[0], parts[1], pod.Spec.ServiceAccountName, pod.Spec.NodeName, command),
		ExitCode: -1,
		Status:   "staged",
	}, nil
}

func (k *kubernetesProvider) api(ctx context.Context, method, path, body string) ([]byte, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, k.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+k.token)
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := k.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kubernetes request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kubernetes http %d: %s", resp.StatusCode, truncateStr(string(data), 256))
	}
	return data, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
