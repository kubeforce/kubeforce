#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CERT_DIR=${CURRENT_DIR}/../tmp/cert
mkdir -p "${CERT_DIR}"

cat > "${CERT_DIR}/ca.json" <<EOF
{
  "CA": {
    "expiry": "127200h",
    "pathlen": 0
  },
  "CN": "ca",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "O": "Kubeforce Inc"
    }
  ]
}
EOF

cat > "${CERT_DIR}/cfssl.json" <<EOF
{
  "signing": {
    "default": {
      "expiry": "8760h"
    },
    "profiles": {
      "intermediate_ca": {
        "usages": [
          "signing",
          "digital signature",
          "key encipherment",
          "cert sign",
          "crl sign",
          "server auth",
          "client auth"
        ],
        "expiry": "8760h",
        "ca_constraint": {
          "is_ca": true,
          "max_path_len": 0,
          "max_path_len_zero": true
        }
      },
      "peer": {
        "usages": [
          "signing",
          "digital signature",
          "key encipherment",
          "client auth",
          "server auth"
        ],
        "expiry": "8760h"
      },
      "server": {
        "usages": [
          "signing",
          "digital signing",
          "key encipherment",
          "server auth"
        ],
        "expiry": "43800h"
      },
      "client": {
        "usages": [
          "signing",
          "key encipherment",
          "client auth"
        ],
        "expiry": "127200h",
        "ca_constraint": {
          "is_ca": false
        }
      }
    }
  }
}
EOF

cat > "${CERT_DIR}/service.json" <<EOF
{
  "CN": "Kubeforce agent",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "O": "Kubeforce Inc"
    }
  ],
  "hosts": [
    "localhost",
    "127.0.0.1"
  ]
}
EOF

cat > "${CERT_DIR}/client.json" <<EOF
{
  "CN": "Kubeforce client",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "O": "admin"
    }
  ]
}
EOF

cfssl gencert -initca "${CERT_DIR}/ca.json" | cfssljson -bare "${CERT_DIR}/ca"
openssl x509  -text -noout -in "${CERT_DIR}/ca.pem"

cfssl gencert -ca "${CERT_DIR}/ca.pem" -ca-key "${CERT_DIR}/ca-key.pem" -config "${CERT_DIR}/cfssl.json" -profile=server "${CERT_DIR}/service.json" | cfssljson -bare "${CERT_DIR}/service"
openssl x509  -text -noout -in "${CERT_DIR}/service.pem"

cfssl gencert -ca "${CERT_DIR}/ca.pem" -ca-key "${CERT_DIR}/ca-key.pem" -config "${CERT_DIR}/cfssl.json" -profile=client "${CERT_DIR}/client.json" | cfssljson -bare "${CERT_DIR}/client"

CERT_AUTH_DATA=$(cat ${CERT_DIR}/ca.pem | base64)
SERVER_CERT_DATA=$(cat ${CERT_DIR}/service.pem | base64)
SERVER_KEY_DATA=$(cat ${CERT_DIR}/service-key.pem | base64)

CLIENT_CERT_DATA=$(cat ${CERT_DIR}/client.pem | base64)
CLIENT_KEY_DATA=$(cat ${CERT_DIR}/client-key.pem | base64)

cat >  "${CERT_DIR}/config.yaml" <<EOF
apiVersion: config.agent.kubeforce.io/v1alpha1
kind: Config
spec:
  port: 5443
  shutdownGracePeriod: 50s
  playbookPath: "tmp/playbook"
  tls:
    certData: ${SERVER_CERT_DATA}
    privateKeyData: ${SERVER_KEY_DATA}
    tlsMinVersion: "VersionTLS13"
  authentication:
    x509:
      clientCAData: ${CERT_AUTH_DATA}
  etcd:
    dataDir: "tmp/etcd-data"
    certsDir: "tmp/etcd-certs"
    listenPeerURLs: "https://127.0.0.1:2380"
    listenClientURLs: "https://127.0.0.1:2379"
EOF

cat >  "${CERT_DIR}/kubeconfig.yaml" <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CERT_AUTH_DATA}
    server: https://127.0.0.1:5443
  name: default
contexts:
- context:
    cluster: default
    user: admin
  name: default
current-context: default
users:
- name: admin
  user:
    client-certificate-data: ${CLIENT_CERT_DATA}
    client-key-data: ${CLIENT_KEY_DATA}
EOF