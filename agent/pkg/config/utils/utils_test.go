/*
Copyright 2022 The Kubeforce Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k3f.io/kubeforce/agent/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configCase1 = config.Config{
		Spec: config.ConfigSpec{
			Port: 8080,
			TLS: config.TLS{
				CertFile:       "/etc/kubeforce/certs/agent.crt",
				PrivateKeyFile: "/etc/kubeforce/certs/agent.key",
				TLSMinVersion:  "1.2",
				PrivateKeyData: []byte("test"),
				CertData:       []byte("test"),
			},
			Authentication: config.AgentAuthentication{
				X509: config.AgentX509Authentication{
					ClientCAFile: "/etc/kubeforce/certs/ca.crt",
				},
			},
			ShutdownGracePeriod: metav1.Duration{Duration: 30 * time.Second},
			Etcd: config.EtcdConfig{
				DataDir:          "/var/etcd/data",
				CertsDir:         "/etc/kubeforce/etcd/certs",
				ListenPeerURLs:   "http://127.0.0.1:2380",
				ListenClientURLs: "http://127.0.0.1:2379",
			},
			PlaybookPath: "/var/lib/kubeforce/playbooks",
		},
	}
	releaseDataCase1 = strings.TrimSpace(`
apiVersion: config.agent.kubeforce.io/v1alpha1
kind: Config
spec:
  authentication:
    x509:
      clientCAFile: /etc/kubeforce/certs/ca.crt
  etcd:
    certsDir: /etc/kubeforce/etcd/certs
    dataDir: /var/etcd/data
    listenClientURLs: http://127.0.0.1:2379
    listenPeerURLs: http://127.0.0.1:2380
  playbookPath: /var/lib/kubeforce/playbooks
  port: 8080
  shutdownGracePeriod: 30s
  tls:
    certData: dGVzdA==
    certFile: /etc/kubeforce/certs/agent.crt
    privateKeyData: dGVzdA==
    privateKeyFile: /etc/kubeforce/certs/agent.key
    tlsMinVersion: "1.2"
`)
)

func TestUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *config.Config
		wantErr bool
	}{
		{
			name:    "successfully deserialize the release",
			data:    []byte(releaseDataCase1),
			want:    &configCase1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("Unmarshal() got = %v, want %v", got, tt.want)
				t.Errorf(cmp.Diff(got, tt.want))
			}
		})
	}
}

func TestMarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		want    []byte
		wantErr bool
	}{
		{
			name:    "successfully serialize the release",
			config:  configCase1,
			want:    []byte(releaseDataCase1),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got = bytes.TrimSpace(got)
			if !cmp.Equal(got, tt.want) {
				t.Errorf("Marshal() got = \n%v\n, want = \n%v\n", string(got), string(tt.want))
				t.Errorf(cmp.Diff(got, tt.want))
			}
		})
	}
}
