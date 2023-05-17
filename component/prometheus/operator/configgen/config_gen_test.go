package configgen

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/grafana/agent/component/common/config"
	"github.com/grafana/agent/component/common/kubernetes"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
)

var (
	configGen = &ConfigGenerator{
		Secrets: &fakeSecrets{},
	}
)

func TestGenerateK8SSDConfig(t *testing.T) {
	nsDiscovery := promk8s.NamespaceDiscovery{
		Names: []string{""},
	}

	tests := []struct {
		name           string
		client         *kubernetes.ClientArguments
		attachMetadata *promopv1.AttachMetadata
		expected       *promk8s.SDConfig
	}{
		{
			name: "empty",
			client: &kubernetes.ClientArguments{
				APIServer: config.URL{},
			},
			attachMetadata: nil,
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
			},
		},
		{
			name: "kubeconfig",
			client: &kubernetes.ClientArguments{
				KubeConfig: "kubeconfig",
				APIServer:  config.URL{},
			},
			attachMetadata: nil,
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
				KubeConfig:         "kubeconfig",
			},
		},
		{
			name: "attach metadata",
			client: &kubernetes.ClientArguments{
				APIServer: config.URL{},
			},
			attachMetadata: &promopv1.AttachMetadata{
				Node: true,
			},
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
				AttachMetadata:     promk8s.AttachMetadataConfig{Node: true},
			},
		},
		{
			name: "http client config",
			client: &kubernetes.ClientArguments{
				APIServer: config.URL{
					URL: &url.URL{
						Scheme: "https",
						Host:   "localhost:8080",
					},
				},
				HTTPClientConfig: config.HTTPClientConfig{
					BasicAuth: &config.BasicAuth{
						Username: "username",
						Password: "password",
					},
					BearerToken:     "bearer",
					BearerTokenFile: "bearer_file",
					TLSConfig: config.TLSConfig{
						CAFile:   "ca_file",
						CertFile: "cert_file",
					},
					Authorization: &config.Authorization{
						Credentials: "credentials",
					},
				},
			},
			attachMetadata: nil,
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
				APIServer: promConfig.URL{
					URL: &url.URL{
						Scheme: "https",
						Host:   "localhost:8080",
					},
				},
				HTTPClientConfig: promConfig.HTTPClientConfig{
					BasicAuth: &promConfig.BasicAuth{
						Username: "username",
						Password: "password",
					},
					BearerToken:     "bearer",
					BearerTokenFile: "bearer_file",
					TLSConfig: promConfig.TLSConfig{
						CAFile:   "ca_file",
						CertFile: "cert_file",
					},
					Authorization: &promConfig.Authorization{
						Type:        "Bearer",
						Credentials: "credentials",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cg := &ConfigGenerator{
				Client: tt.client,
			}
			got := cg.generateK8SSDConfig(promopv1.NamespaceSelector{}, "", promk8s.RoleEndpoint, tt.attachMetadata)
			assert.Equal(t, tt.expected, got)
		})
	}
}

type fakeSecrets struct{}

func (f *fakeSecrets) GetSecretValue(namespace string, sec corev1.SecretKeySelector) (string, error) {
	return fmt.Sprintf("secret/%s/%s/%s", namespace, sec.Name, sec.Key), nil
}
func (f *fakeSecrets) GetConfigMapValue(namespace string, cm corev1.ConfigMapKeySelector) (string, error) {
	return fmt.Sprintf("cm/%s/%s/%s", namespace, cm.Name, cm.Key), nil
}
func (f *fakeSecrets) SecretOrConfigMapValue(namespace string, socm promopv1.SecretOrConfigMap) (string, error) {
	if socm.Secret != nil {
		return f.GetSecretValue(namespace, *socm.Secret)
	}
	return f.GetConfigMapValue(namespace, *socm.ConfigMap)
}

// convenience functions for generating references
func s(name, key string) *k8sv1.SecretKeySelector {
	return &k8sv1.SecretKeySelector{
		Key: key,
		LocalObjectReference: k8sv1.LocalObjectReference{
			Name: name,
		},
	}
}
func cm(name, key string) *k8sv1.ConfigMapKeySelector {
	return &k8sv1.ConfigMapKeySelector{
		Key: key,
		LocalObjectReference: k8sv1.LocalObjectReference{
			Name: name,
		},
	}
}
func TestGenerateSafeTLSConfig(t *testing.T) {
	tests := []struct {
		name       string
		tlsConfig  promopv1.SafeTLSConfig
		hasErr     bool
		serverName string
		ca         string
		cert       string
		key        promConfig.Secret
	}{
		{
			name: "empty",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: true,
				ServerName:         "test",
			},
			hasErr:     false,
			serverName: "test",
		},
		{
			name: "ca_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: true,
				CA:                 promopv1.SecretOrConfigMap{Secret: s("secrets", "ca_file")},
			},
			hasErr:     false,
			serverName: "",
			ca:         "secret/ns/secrets/ca_file",
		},
		{
			name: "ca_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: true,
				CA:                 promopv1.SecretOrConfigMap{ConfigMap: cm("non-secrets", "ca_file")},
			},
			hasErr:     false,
			serverName: "",
			ca:         "cm/ns/non-secrets/ca_file",
		},
		{
			name: "cert_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: true,
				Cert:               promopv1.SecretOrConfigMap{Secret: s("secrets", "cert_file")},
			},
			hasErr:     false,
			serverName: "",
			cert:       "secret/ns/secrets/cert_file",
		},
		{
			name: "cert_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: true,
				Cert:               promopv1.SecretOrConfigMap{ConfigMap: cm("non-secrets", "cert_file")},
			},
			hasErr:     false,
			serverName: "",
			cert:       "cm/ns/non-secrets/cert_file",
		},
		{
			name: "key_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: true,
				KeySecret:          s("secrets", "key_file"),
			},
			hasErr:     false,
			serverName: "",
			key:        "secret/ns/secrets/key_file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configGen.generateSafeTLS(tt.tlsConfig, "ns")
			assert.Equal(t, tt.hasErr, err != nil)
			assert.True(t, got.InsecureSkipVerify)
			assert.Equal(t, tt.serverName, got.ServerName)
			assert.Equal(t, tt.ca, got.CA)
			assert.Equal(t, tt.cert, got.Cert)
			assert.Equal(t, tt.key, got.Key)
		})
	}
}

func TestGenerateBasicAuth(t *testing.T) {

}

func TestRelabelerAdd(t *testing.T) {
	relabeler := &relabeler{}

	cfgs := []*relabel.Config{
		{
			Action:      "",
			Separator:   "",
			Regex:       relabel.Regexp{},
			Replacement: "",
		},
	}
	expected := relabel.Config{
		Action:      relabel.DefaultRelabelConfig.Action,
		Separator:   relabel.DefaultRelabelConfig.Separator,
		Regex:       relabel.DefaultRelabelConfig.Regex,
		Replacement: relabel.DefaultRelabelConfig.Replacement,
	}
	relabeler.add(cfgs...)
	cfg := cfgs[0]

	assert.Equal(t, expected.Action, cfg.Action)
	assert.Equal(t, expected.Separator, cfg.Separator)
	assert.Equal(t, expected.Regex, cfg.Regex)
	assert.Equal(t, expected.Replacement, cfg.Replacement)
}

func TestRelabelerAddFromV1(t *testing.T) {
	relabeler := &relabeler{}

	cfgs := []*promopv1.RelabelConfig{
		{
			SourceLabels: []promopv1.LabelName{"__meta_kubernetes_pod_label_app"},
			Separator:    ";",
			TargetLabel:  "app",
			Regex:        "(.*)",
			Modulus:      1,
			Replacement:  "$1",
			Action:       "replace",
		},
	}
	expected := relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_pod_label_app"},
		Separator:    ";",
		TargetLabel:  "app",
		Regex:        relabel.MustNewRegexp("(.*)"),
		Modulus:      1,
		Replacement:  "$1",
		Action:       relabel.Replace,
	}
	relabeler.addFromV1(cfgs...)
	cfg := relabeler.configs[0]

	assert.Equal(t, expected.SourceLabels, cfg.SourceLabels)
	assert.Equal(t, expected.Separator, cfg.Separator)
	assert.Equal(t, expected.TargetLabel, cfg.TargetLabel)
	assert.Equal(t, expected.Regex, cfg.Regex)
	assert.Equal(t, expected.Modulus, cfg.Modulus)
	assert.Equal(t, expected.Replacement, cfg.Replacement)
	assert.Equal(t, expected.Action, cfg.Action)
}
