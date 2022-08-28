#!/bin/bash

# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This file has been copied and modified from https://sigs.k8s.io/kind/images/base/files/usr/local/bin/entrypoint

set -o errexit
set -o nounset
set -o pipefail

# If /proc/self/uid_map 4294967295 mappings, we are in the initial user namespace, i.e. the host.
# Otherwise we are in a non-initial user namespace.
# https://github.com/opencontainers/runc/blob/v1.0.0-rc92/libcontainer/system/linux.go#L109-L118
userns=""
if grep -Eqv "0[[:space:]]+0[[:space:]]+4294967295" /proc/self/uid_map; then
  userns="1"
  echo 'INFO: running in a user namespace (experimental)'
fi

validate_userns() {
  if [[ -z "${userns}" ]]; then
    return
  fi

  local nofile_hard
  nofile_hard="$(ulimit -Hn)"
  local nofile_hard_expected="64000"
  if [[ "${nofile_hard}" -lt "${nofile_hard_expected}" ]]; then
    echo "WARN: UserNS: expected RLIMIT_NOFILE to be at least ${nofile_hard_expected}, got ${nofile_hard}" >&2
  fi

  if [[ ! -f "/sys/fs/cgroup/cgroup.controllers" ]]; then
    echo "ERROR: UserNS: cgroup v2 needs to be enabled" >&2
    exit 1
  fi
  for f in cpu memory pids; do
    if ! grep -qw $f /sys/fs/cgroup/cgroup.controllers; then
      echo "ERROR: UserNS: $f controller needs to be delegated" >&2
    exit 1
    fi
  done
}

fake_file_with_content(){
  local path="$1"
  local content="$2"
  local base="/run/fake"
  local fake_path="${base}/${path}"
  mkdir -p "$(dirname "${fake_path}")"
  echo "INFO: UserNS: faking ${path} to be \"${content}\" (writable)"
  echo "${content}" > "${fake_path}"
  mount --bind "${fake_path}" "${path}"
}

fake_sysctl() {
  local key="$1"
  local key_slash
  # shellcheck disable=SC2001
  key_slash="$(echo "${key}" | sed -e s@\\.@/@g)"
  local path="/proc/sys/${key_slash}"
  if [[ -f "${path}" ]]; then
    local content
    content="$(cat "${path}")"
    fake_file_with_content "${path}" "${content}"
  fi
}

configure_containerd() {
  local snapshotter=${KIND_EXPERIMENTAL_CONTAINERD_SNAPSHOTTER:-}
  if [[ -n "$userns" ]]; then
    # userns (rootless) configs

    # Adjust oomScoreAdj
    sed -i 's/restrict_oom_score_adj = false/restrict_oom_score_adj = true/' /etc/containerd/config.toml

    # Use fuse-overlayfs by default: https://github.com/kubernetes-sigs/kind/issues/2275
    snapshotter="fuse-overlayfs"

    # To run vanilla kubelet inside UserNS, we need to fake several unwritable sysctl to be writable.
    # Workaround until https://github.com/kubernetes/kubernetes/pull/92863 gets merged in the upstream.
    fake_sysctl "vm.overcommit_memory"
    fake_sysctl "vm.panic_on_oom"
    fake_sysctl "kernel.panic"
    fake_sysctl "kernel.panic_on_oops"
    fake_sysctl "kernel.keys.root_maxkeys"
    fake_sysctl "kernel.keys.root_maxbytes"
  else
    # we need to switch to the 'native' snapshotter on zfs
    if [[ "$(stat -f -c %T /kind)" == 'zfs' ]]; then
      snapshotter="native"
    fi
  fi
  if [[ -n "$snapshotter" ]]; then
    echo "INFO: changing snapshotter from \"overlayfs\" to \"$snapshotter\""
    sed -i "s/snapshotter = \"overlayfs\"/snapshotter = \"$snapshotter\"/" /etc/containerd/config.toml
    if [[ "$snapshotter" = "fuse-overlayfs" ]]; then
      echo 'INFO: enabling containerd-fuse-overlayfs service'
      systemctl enable containerd-fuse-overlayfs
    fi
  fi
}

configure_proxy() {
  # ensure all processes receive the proxy settings by default
  # https://www.freedesktop.org/software/systemd/man/systemd-system.conf.html
  mkdir -p /etc/systemd/system.conf.d/
  cat <<EOF >/etc/systemd/system.conf.d/proxy-default-environment.conf
[Manager]
DefaultEnvironment="HTTP_PROXY=${HTTP_PROXY:-}" "HTTPS_PROXY=${HTTPS_PROXY:-}" "NO_PROXY=${NO_PROXY:-}"
EOF
}

fix_mount() {
  echo 'INFO: ensuring we can execute mount/umount even with userns-remap' 
  # necessary only when userns-remap is enabled on the host, but harmless
  # The binary /bin/mount should be owned by root and have the setuid bit
  chown root:root "$(which mount)" "$(which umount)"
  chmod -s "$(which mount)" "$(which umount)"

  # This is a workaround to an AUFS bug that might cause `Text file
  # busy` on `mount` command below. See more details in
  # https://github.com/moby/moby/issues/9547
  if [[ "$(stat -f -c %T "$(which mount)")" == 'aufs' ]]; then
    echo 'INFO: detected aufs, calling sync' >&2
    sync
  fi

  if [[ -z "${userns}" ]]; then
    echo 'INFO: remounting /sys read-only'
    # systemd-in-a-container should have read only /sys
    # https://systemd.io/CONTAINER_INTERFACE/
    # however, we need other things from `docker run --privileged` ...
    # and this flag also happens to make /sys rw, amongst other things
    #
    # This step is skipped when running inside UserNS, because it fails with EACCES.
    mount -o remount,ro /sys
  fi

  echo 'INFO: making mounts shared' >&2
  # for mount propagation
  mount --make-rshared /
}

# helper used by fix_cgroup
mount_kubelet_cgroup_root() {
  local cgroup_root=$1
  local subsystem=$2
  if [ -z "${cgroup_root}" ]; then
    return 0
  fi
  mkdir -p "${subsystem}/${cgroup_root}"
  if [ "${subsystem}" == "/sys/fs/cgroup/cpuset" ]; then
    # This is needed. Otherwise, assigning process to the cgroup
    # (or any nested cgroup) would result in ENOSPC.
    cat "${subsystem}/cpuset.cpus" > "${subsystem}/${cgroup_root}/cpuset.cpus"
    cat "${subsystem}/cpuset.mems" > "${subsystem}/${cgroup_root}/cpuset.mems"
  fi
  # We need to perform a self bind mount here because otherwise,
  # systemd might delete the cgroup unintentionally before the
  # kubelet starts.
  mount --bind "${subsystem}/${cgroup_root}" "${subsystem}/${cgroup_root}"
}

fix_cgroup() {
  if [[ -f "/sys/fs/cgroup/cgroup.controllers" ]]; then
    echo 'INFO: detected cgroup v2'
    # Both Docker and Podman enable CgroupNS on cgroup v2 hosts by default.
    #
    # So mostly we do not need to mess around with the cgroup path stuff,
    # however, we still need to create the "/kubelet" cgroup at least.
    # (Otherwise kubelet fails with `cgroup-root ["kubelet"] doesn't exist` error, see #1969)
    #
    # The "/kubelet" cgroup is created in ExecStartPre of the kubeadm service.
    #
    # [FAQ: Why not create "/kubelet" cgroup here?]
    # We can't create the cgroup with controllers here, because /sys/fs/cgroup/cgroup.subtree_control is empty.
    # And yet we can't write controllers to /sys/fs/cgroup/cgroup.subtree_control by ourselves either, because
    # /sys/fs/cgroup/cgroup.procs is not empty at this moment.
    #
    # After switching from this entrypoint script to systemd, systemd evacuates the processes in the root
    # group to "/init.scope" group, so we can write the root subtree_control and create "/kubelet" cgroup.
    return
  fi
  echo 'INFO: detected cgroup v1'
  echo 'INFO: fix cgroup mounts for all subsystems'
  # See: https://d2iq.com/blog/running-kind-inside-a-kubernetes-cluster-for-continuous-integration
  # Capture initial state before modifying
  #
  # Basically we're looking for the cgroup-path for the cpu controller for the
  # current process. this tells us what cgroup-path the container is in.
  # Then we collect the subsystems that are active on this path.
  # We assume the cpu controller is in use on all node containers.
  #
  # See: https://man7.org/linux/man-pages/man7/cgroups.7.html
  local current_cgroup
  current_cgroup=$(grep -E '^[^:]*:([^:]*,)?cpu(,[^,:]*)?:.*' /proc/self/cgroup | cut -d: -f3)
  local cgroup_subsystems
  cgroup_subsystems=$(findmnt -lun -o source,target -t cgroup | grep "${current_cgroup}" | awk '{print $2}')
  # For each cgroup subsystem, Docker does a bind mount from the current
  # cgroup to the root of the cgroup subsystem. For instance:
  #   /sys/fs/cgroup/memory/docker/<cid> -> /sys/fs/cgroup/memory
  #
  # This will confuse Kubelet and cadvisor and will dump the following error
  # messages in kubelet log:
  #   `summary_sys_containers.go:47] Failed to get system container stats for ".../kubelet.service"`
  #
  # This is because `/proc/<pid>/cgroup` is not affected by the bind mount.
  # The following is a workaround to recreate the original cgroup
  # environment by doing another bind mount for each subsystem.
  local cgroup_mounts
  # xref: https://github.com/kubernetes/minikube/pull/9508
  # Example inputs:
  #
  # Docker:               /docker/562a56986a84b3cd38d6a32ac43fdfcc8ad4d2473acf2839cbf549273f35c206 /sys/fs/cgroup/devices rw,nosuid,nodev,noexec,relatime shared:143 master:23 - cgroup devices rw,devices
  # podman:               /libpod_parent/libpod-73a4fb9769188ae5dc51cb7e24b9f2752a4af7b802a8949f06a7b2f2363ab0e9 ...
  # Cloud Shell:          /kubepods/besteffort/pod3d6beaa3004913efb68ce073d73494b0/accdf94879f0a494f317e9a0517f23cdd18b35ff9439efd0175f17bbc56877c4 /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime master:19 - cgroup cgroup rw,memory
  # GitHub actions #9304: /actions_job/0924fbbcf7b18d2a00c171482b4600747afc367a9dfbeac9d6b14b35cda80399 /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:263 master:24 - cgroup cgroup rw,memory
  cgroup_mounts=$(grep -E -o '/[[:alnum:]].* /sys/fs/cgroup.*.*cgroup' /proc/self/mountinfo || true)
  if [[ -n "${cgroup_mounts}" ]]; then
    local mount_root
    mount_root=$(head -n 1 <<<"${cgroup_mounts}" | cut -d' ' -f1)
    for mount_point in $(echo "${cgroup_mounts}" | cut -d' ' -f 2); do
      # bind mount each mount_point to mount_point + mount_root
      # mount --bind /sys/fs/cgroup/cpu /sys/fs/cgroup/cpu/docker/fb07bb6daf7730a3cb14fc7ff3e345d1e47423756ce54409e66e01911bab2160
      local target="${mount_point}${mount_root}"
      if ! findmnt "${target}"; then
        mkdir -p "${target}"
        mount --bind "${mount_point}" "${target}"
      fi
    done
  fi
  # kubelet will try to manage cgroups / pods that are not owned by it when
  # "nesting" clusters, unless we instruct it to use a different cgroup root.
  # We do this, and when doing so we must fixup this alternative root
  # currently this is hardcoded to be /kubelet
  mount --make-rprivate /sys/fs/cgroup
  echo "${cgroup_subsystems}" |
  while IFS= read -r subsystem; do
    mount_kubelet_cgroup_root "/kubelet" "${subsystem}"
  done
}

fix_machine_id() {
  # Deletes the machine-id embedded in the node image and generates a new one.
  # This is necessary because both kubelet and other components like weave net
  # use machine-id internally to distinguish nodes.
  echo 'INFO: clearing and regenerating /etc/machine-id' >&2
  rm -f /etc/machine-id
  systemd-machine-id-setup
}

fix_product_name() {
  # this is a small fix to hide the underlying hardware and fix issue #426
  # https://github.com/kubernetes-sigs/kind/issues/426
  if [[ -f /sys/class/dmi/id/product_name ]]; then
    echo 'INFO: faking /sys/class/dmi/id/product_name to be "kind"' >&2
    echo 'kind' > /kind/product_name
    mount -o ro,bind /kind/product_name /sys/class/dmi/id/product_name
  fi
}

fix_product_uuid() {
  # The system UUID is usually read from DMI via sysfs, the problem is that
  # in the kind case this means that all (container) nodes share the same
  # system/product uuid, as they share the same DMI.
  # Note: The UUID is read from DMI, this tool is overwriting the sysfs files
  # which should fix the attached issue, but this workaround does not address
  # the issue if a tool is reading directly from DMI.
  # https://github.com/kubernetes-sigs/kind/issues/1027
  [[ ! -f /kind/product_uuid ]] && cat /proc/sys/kernel/random/uuid > /kind/product_uuid
  if [[ -f /sys/class/dmi/id/product_uuid ]]; then
    echo 'INFO: faking /sys/class/dmi/id/product_uuid to be random' >&2
    mount -o ro,bind /kind/product_uuid /sys/class/dmi/id/product_uuid
  fi
  if [[ -f /sys/devices/virtual/dmi/id/product_uuid ]]; then
    echo 'INFO: faking /sys/devices/virtual/dmi/id/product_uuid as well' >&2
    mount -o ro,bind /kind/product_uuid /sys/devices/virtual/dmi/id/product_uuid
  fi
}

fix_kmsg() {
  # In environments where /dev/kmsg is not available, the kubelet (1.15+) won't
  # start because it cannot open /dev/kmsg when starting the kmsgparser in the
  # OOM parser.
  # To support those environments, we link /dev/kmsg to /dev/console.
  # https://github.com/kubernetes-sigs/kind/issues/662
  if [[ ! -e /dev/kmsg ]]; then
    if [[ -e /dev/console ]]; then
      echo 'WARN: /dev/kmsg does not exist, symlinking /dev/console' >&2
      ln -s /dev/console /dev/kmsg
    else
      echo 'WARN: /dev/kmsg does not exist, nor does /dev/console!' >&2
    fi
  elif [[ -n "${userns}" ]]; then
    if [[ -f "/proc/sys/kernel/dmesg_restrict" ]]; then
      if [[ "$(cat /proc/sys/kernel/dmesg_restrict)" = "1" ]]; then
        echo 'WARN: UserNS: /dev/kmsg is not readable, faking with /dev/null (hint: set sysctl value "kernel.dmesg_restrict" to 0)' >&2
        mount --bind /dev/null /dev/kmsg
      fi
    fi
  fi
}

select_iptables() {
  # based on: https://github.com/kubernetes-sigs/iptables-wrappers/blob/97b01f43a8e8db07840fc4b95e833a37c0d36b12/iptables-wrapper-installer.sh
  local mode num_legacy_lines num_nft_lines
  num_legacy_lines=$( (iptables-legacy-save || true; ip6tables-legacy-save || true) 2>/dev/null | grep '^-' | wc -l || true)
  num_nft_lines=$( (timeout 5 sh -c "iptables-nft-save; ip6tables-nft-save" || true) 2>/dev/null | grep '^-' | wc -l || true)
  if [ "${num_legacy_lines}" -ge "${num_nft_lines}" ]; then
    mode=legacy
  else
    mode=nft
  fi
  echo "INFO: setting iptables to detected mode: ${mode}" >&2
  update-alternatives --set iptables "/usr/sbin/iptables-${mode}" > /dev/null
  update-alternatives --set ip6tables "/usr/sbin/ip6tables-${mode}" > /dev/null
}

enable_network_magic(){
  mkdir -p /kind
  # well-known docker embedded DNS is at 127.0.0.11:53
  local docker_embedded_dns_ip='127.0.0.11'

  # first we need to detect an IP to use for reaching the docker host
  local docker_host_ip
  docker_host_ip="$( (head -n1 <(getent ahostsv4 'host.docker.internal') | cut -d' ' -f1) || true)"
  # if the ip doesn't exist or is a loopback address use the default gateway
  if [[ -z "${docker_host_ip}" ]] || [[ $docker_host_ip =~ ^127\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    docker_host_ip=$(ip -4 route show default | cut -d' ' -f3)
  fi

  # patch docker's iptables rules to switch out the DNS IP
  iptables-save \
    | sed \
      `# switch docker DNS DNAT rules to our chosen IP` \
      -e "s/-d ${docker_embedded_dns_ip}/-d ${docker_host_ip}/g" \
      `# we need to also apply these rules to non-local traffic (from pods)` \
      -e 's/-A OUTPUT \(.*\) -j DOCKER_OUTPUT/\0\n-A PREROUTING \1 -j DOCKER_OUTPUT/' \
      `# switch docker DNS SNAT rules rules to our chosen IP` \
      -e "s/--to-source :53/--to-source ${docker_host_ip}:53/g"\
    | iptables-restore

  # now we can ensure that DNS is configured to use our IP
  cp /etc/resolv.conf /etc/resolv.conf.original
  sed -e "s/${docker_embedded_dns_ip}/${docker_host_ip}/g" /etc/resolv.conf.original >/etc/resolv.conf

  # fixup IPs in manifests ...
  curr_ipv4="$( (head -n1 <(getent ahostsv4 "$(hostname)") | cut -d' ' -f1) || true)"
  echo "INFO: Detected IPv4 address: ${curr_ipv4}" >&2
  if [ -f /kind/old-ipv4 ]; then
      old_ipv4=$(cat /kind/old-ipv4)
      echo "INFO: Detected old IPv4 address: ${old_ipv4}" >&2
      # sanity check that we have a current address
      if [[ -z $curr_ipv4 ]]; then
        echo "ERROR: Have an old IPv4 address but no current IPv4 address (!)" >&2
        exit 1
      fi
      # kubernetes manifests are only present on control-plane nodes
      sed -i "s#${old_ipv4}#${curr_ipv4}#" /etc/kubernetes/manifests/*.yaml || true
      # this is no longer required with autodiscovery
      sed -i "s#${old_ipv4}#${curr_ipv4}#" /var/lib/kubelet/kubeadm-flags.env || true
  fi
  if [[ -n $curr_ipv4 ]]; then
    echo -n "${curr_ipv4}" >/kind/old-ipv4
  fi

  # do IPv6
  curr_ipv6="$( (head -n1 <(getent ahostsv6 "$(hostname)") | cut -d' ' -f1) || true)"
  echo "INFO: Detected IPv6 address: ${curr_ipv6}" >&2
  if [ -f /kind/old-ipv6 ]; then
      old_ipv6=$(cat /kind/old-ipv6)
      echo "INFO: Detected old IPv6 address: ${old_ipv6}" >&2
      # sanity check that we have a current address
      if [[ -z $curr_ipv6 ]]; then
        echo "ERROR: Have an old IPv6 address but no current IPv6 address (!)" >&2
      fi
      # kubernetes manifests are only present on control-plane nodes
      sed -i "s#${old_ipv6}#${curr_ipv6}#" /etc/kubernetes/manifests/*.yaml || true
      # this is no longer required with autodiscovery
      sed -i "s#${old_ipv6}#${curr_ipv6}#" /var/lib/kubelet/kubeadm-flags.env || true
  fi
  if [[ -n $curr_ipv6 ]]; then
    echo -n "${curr_ipv6}" >/kind/old-ipv6
  fi
}

# validate state
validate_userns

# run pre-init fixups
# NOTE: it's important that we do configure* first in this order to avoid races
configure_containerd
configure_proxy
fix_kmsg
fix_mount
fix_cgroup
fix_machine_id
#fix_product_name
#fix_product_uuid
select_iptables
enable_network_magic

# we want the command (expected to be systemd) to be PID1, so exec to it
exec "$@"
