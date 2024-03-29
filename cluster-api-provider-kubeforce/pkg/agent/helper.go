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

package agent

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k3f.io/kubeforce/agent/pkg/config"
	configutils "k3f.io/kubeforce/agent/pkg/config/utils"
	infrav1 "k3f.io/kubeforce/cluster-api-provider-kubeforce/api/v1beta1"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/repository"
	"k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/secret"
	stringutil "k3f.io/kubeforce/cluster-api-provider-kubeforce/pkg/util/strings"
)

// GetHelper returns agent helper.
func GetHelper(ctx context.Context, client client.Client, storage *repository.Storage, kfAgent *infrav1.KubeforceAgent) (*Helper, error) {
	agentKeys, err := GetKeys(ctx, client, kfAgent)
	if err != nil {
		return nil, err
	}
	server, err := GetServer(*kfAgent.Spec.Addresses)
	if err != nil {
		return nil, err
	}
	agentHelper, err := NewHelper(client, storage, kfAgent, agentKeys, server)
	if err != nil {
		return nil, err
	}
	return agentHelper, nil
}

// NewHelper creates a new agent helper.
func NewHelper(client client.Client, storage *repository.Storage, kfAgent *infrav1.KubeforceAgent, keys *Keys, server string) (*Helper, error) {
	return &Helper{
		client:  client,
		keys:    keys,
		server:  server,
		agent:   kfAgent,
		storage: storage,
	}, nil
}

// Helper helps to install agent on host.
type Helper struct {
	client  client.Client
	storage *repository.Storage
	keys    *Keys
	server  string
	agent   *infrav1.KubeforceAgent
}

func (h *Helper) getAgentFilepath(ctx context.Context) (string, error) {
	if h.agent.Spec.Source == nil || h.agent.Spec.Source.RepoRef == nil {
		return "", errors.Errorf("source is not specified for the agent %v", client.ObjectKeyFromObject(h.agent))
	}
	repo := &infrav1.HTTPRepository{}
	key := client.ObjectKey{
		Namespace: h.agent.Spec.Source.RepoRef.Namespace,
		Name:      h.agent.Spec.Source.RepoRef.Name,
	}
	if key.Namespace == "" {
		key.Namespace = h.agent.Namespace
	}
	err := h.client.Get(ctx, key, repo)
	if err != nil {
		return "", err
	}
	if h.agent.Spec.Source.Version == "" {
		h.agent.Spec.Source.Version = version.Get().GitVersion
	}
	relativePath := fmt.Sprintf("%s/agent-%s-%s", h.agent.Spec.Source.Version, h.agent.Spec.System.Os, h.agent.Spec.System.Arch)
	if h.agent.Spec.Source.Path != "" {
		relativePath = path.Join(h.agent.Spec.Source.Path, relativePath)
	}
	f, err := h.storage.GetHTTPFileGetter(*repo).GetFile(ctx, relativePath)
	if err != nil {
		return "", err
	}
	return f.Path, nil
}

func (h *Helper) copyAgent(ctx context.Context, sshClient *ssh.Client) error {
	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		return err
	}
	defer scpClient.Close()
	agentPath, err := h.getAgentFilepath(ctx)
	if err != nil {
		return err
	}
	agentFile, err := os.Open(filepath.Clean(agentPath))
	if err != nil {
		return err
	}
	defer agentFile.Close()
	ctx, cancelFunc := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelFunc()
	err = scpClient.CopyFromFile(ctx, *agentFile, "agent", "0777")
	if err != nil {
		return errors.Wrap(err, "unable to copy the agent binary to remote machine via ssh")
	}
	return nil
}

func (h *Helper) copyAgentConfig(ctx context.Context, sshClient *ssh.Client) error {
	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		return err
	}
	defer scpClient.Close()
	cfg, err := h.agentConfig()
	if err != nil {
		return err
	}
	ctx, cancelFunc := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelFunc()
	err = scpClient.Copy(ctx, bytes.NewReader(cfg), "config.yaml", "0600", int64(len(cfg)))
	if err != nil {
		return errors.Wrap(err, "unable to copy the agent configuration to remote machine via ssh")
	}
	return nil
}

func (h *Helper) copyAgentClientConfig(ctx context.Context, sshClient *ssh.Client) error {
	scpClient, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		return err
	}
	defer scpClient.Close()
	apiConfig := NewClientKubeconfig(h.keys, h.server)
	content, err := clientcmd.Write(apiConfig)
	if err != nil {
		return err
	}
	ctx, cancelFunc := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelFunc()
	err = scpClient.Copy(ctx, bytes.NewReader(content), "agent-kubeconfig.yaml", "0600", int64(len(content)))
	if err != nil {
		return errors.Wrap(err, "unable to copy kubeconfig for the agent to remote machine via ssh")
	}
	return nil
}

// Install installs agent to the host.
func (h *Helper) Install(ctx context.Context) error {
	sshClient, err := h.getSSHClient(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get ssh client")
	}
	defer sshClient.Close()

	if err := h.copyAgent(ctx, sshClient); err != nil {
		return err
	}
	if err := h.copyAgentConfig(ctx, sshClient); err != nil {
		return err
	}
	if err := h.copyAgentClientConfig(ctx, sshClient); err != nil {
		return err
	}
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, time.Minute)
	defer cancelFunc()
	cmd := "sudo ./agent init --config config.yaml && rm agent config.yaml"
	if out, err := h.runCommand(ctxTimeout, sshClient, cmd); err != nil {
		msg := fmt.Sprintf("unable to install agent, command: %q", cmd)
		ctrl.LoggerFrom(ctx).Error(err, msg, "out", out)
		return errors.Wrap(err, msg)
	}
	return nil
}

func (h *Helper) getSSHClient(ctx context.Context) (*ssh.Client, error) {
	host := stringutil.Find(stringutil.IsNotEmpty, h.agent.Spec.Addresses.ExternalDNS, h.agent.Spec.Addresses.ExternalIP)
	if host == "" {
		return nil, errors.Errorf("unable to find host address for agent %s", h.agent.Name)
	}
	if h.agent.Spec.SSH.Port < 0 {
		return nil, errors.Errorf("port can not be negative. port: %d", h.agent.Spec.SSH.Port)
	}
	if h.agent.Spec.SSH.Port == 0 {
		h.agent.Spec.SSH.Port = 22
	}
	addr := net.JoinHostPort(host, strconv.Itoa(h.agent.Spec.SSH.Port))
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	sshConfig, err := h.GetSSHConfig(ctx)
	if err != nil {
		return nil, err
	}
	sshCon, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(sshCon, chans, reqs), nil
}

// GetSSHAuthMethod returns ssh authentication method for the KubeforceAgent.
func (h *Helper) GetSSHAuthMethod(ctx context.Context) (ssh.AuthMethod, error) {
	key := types.NamespacedName{
		Namespace: h.agent.Namespace,
		Name:      h.agent.Spec.SSH.SecretName,
	}
	s := &corev1.Secret{}
	err := h.client.Get(ctx, key, s)
	if err != nil {
		return nil, err
	}
	sshPassword := s.Data[secret.SSHAuthPassword]
	if len(sshPassword) > 0 {
		return ssh.Password(string(sshPassword)), nil
	}
	sshPrivateKey := s.Data[secret.SSHAuthPrivateKey]
	if len(sshPrivateKey) == 0 {
		return nil, errors.Errorf("one of fields 'ssh-password' or 'ssh-privatekey' is required for secret %v", key)
	}
	sshPassphrase := s.Data[secret.SSHAuthPassphrase]
	if sshPassphrase != nil {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(sshPrivateKey, sshPassphrase)
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	}
	signer, err := ssh.ParsePrivateKey(sshPrivateKey)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

// GetSSHConfig returns ClientConfig to configure a Client.
func (h *Helper) GetSSHConfig(ctx context.Context) (*ssh.ClientConfig, error) {
	authMethod, err := h.GetSSHAuthMethod(ctx)
	if err != nil {
		return nil, err
	}
	if h.agent.Spec.SSH.Username == "" {
		return nil, errors.Errorf("user for ssh connection is not defined")
	}

	conf := &ssh.ClientConfig{
		User: h.agent.Spec.SSH.Username,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		Timeout: 15 * time.Second,
		//nolint:gosec
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return conf, nil
}

const (
	term  = "unknown"
	termH = 40
	termW = 80
)

var (
	termModes = ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
)

func (h *Helper) runCommand(ctx context.Context, client *ssh.Client, cmd string) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("execution command", "cmd", cmd)

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	err = session.RequestPty(term, termH, termW, termModes)
	if err != nil {
		return "", err
	}

	out := new(bytes.Buffer)
	session.Stdout = out
	session.Stderr = out

	exit := make(chan struct{}, 1)
	defer close(exit)

	go func() {
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGINT)
			_ = session.Close()
		case <-exit:
		}
	}()

	err = session.Run(cmd)
	if err != nil {
		switch err.(type) {
		case *ssh.ExitError:
			log.Error(err, "Command failed", "cmd", cmd)
			return out.String(), err
		case *ssh.ExitMissingError:
			log.Error(err, "Session aborted unexpectedly (node destroyed?)", "cmd", cmd)
			return out.String(), err
		default:
			log.Error(err, "Unexpected error.", "cmd", cmd)
			return out.String(), err
		}
	}
	return out.String(), nil
}

func (h *Helper) agentConfig() ([]byte, error) {
	cfg := &config.Config{
		Spec: config.ConfigSpec{
			Port: 5443,
			TLS: config.TLS{
				CertData:       h.keys.TLS.Cert,
				PrivateKeyData: h.keys.TLS.Key,
				TLSMinVersion:  "VersionTLS13",
			},
			Authentication: config.AgentAuthentication{
				X509: config.AgentX509Authentication{
					ClientCAData: h.keys.AuthClient.CA,
				},
			},
			ShutdownGracePeriod: metav1.Duration{
				Duration: 30 * time.Second,
			},
			Etcd: config.EtcdConfig{
				DataDir:          "/var/lib/kubeforce/etcd",
				CertsDir:         "/etc/kubeforce/etcd/certs",
				ListenPeerURLs:   "https://127.0.0.1:3380",
				ListenClientURLs: "https://127.0.0.1:3379",
			},
			PlaybookPath: "/var/lib/kubeforce/playbooks",
		},
	}
	return configutils.Marshal(cfg)
}
