# syntax=docker/dockerfile:1.4

# Copyright 2022 The Kubeforce Authors.
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

FROM ubuntu:22.04
LABEL maintainer="brett.dellegrazie@gmail.com"

SHELL ["/bin/bash", "-exo", "pipefail", "-c"]
ENV container=docker LANG=C.UTF-8

# Enable all repositories
#RUN sed -i 's/# deb/deb/g' /etc/apt/sources.list

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
    dbus systemd systemd-cron rsyslog iptables iproute2 ebtables \
    python3 python3-apt sudo bash ca-certificates kmod \
    openssh-server sudo python3-pip \
    curl htop iotop lsof ltrace net-tools strace tcpdump vim wget traceroute

# hadolint ignore=DL3013
RUN pip3 install --no-cache-dir ansible

RUN useradd -rm -d /home/ubuntu -s /bin/bash -u 1000 test
RUN echo "test ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/test

RUN echo 'test:test' | chpasswd

#RUN rm -rf /usr/share/doc/* /usr/share/man/* /var/lib/apt/lists/* /tmp/* /var/tmp/*

#RUN find /lib/systemd/system/sysinit.target.wants/ -name "systemd-tmpfiles-setup.service" -delete \
#        && rm -f /lib/systemd/system/multi-user.target.wants/* \
#        && rm -f /etc/systemd/system/*.wants/* \
#        && rm -f /lib/systemd/system/local-fs.target.wants/* \
#        && rm -f /lib/systemd/system/sockets.target.wants/*udev* \
#        && rm -f /lib/systemd/system/sockets.target.wants/*initctl* \
#        && rm -f /lib/systemd/system/basic.target.wants/* \
#        && echo "ReadKMsg=no" >> /etc/systemd/journald.conf

# Don't start any optional services except for the few we need.
RUN find /etc/systemd/system \
    /lib/systemd/system \
    -path '*.wants/*' \
    -not -name '*dbus*' \
    -not -name '*ssh*' \
    -not -name '*journald*' \
    -not -name '*systemd-tmpfiles*' \
    -not -name '*systemd-user-sessions*' \
    -exec rm {} \;

RUN ln -fs "$(which systemd)" /sbin/init
RUN sed -i 's/^\(module(load="imklog")\)/#\1/' /etc/rsyslog.conf

RUN systemctl set-default multi-user.target
RUN systemctl mask dev-hugepages.mount sys-fs-fuse-connections.mount

COPY --chmod=0755 files/usr/local/bin/* /usr/local/bin/

STOPSIGNAL SIGRTMIN+3

ENTRYPOINT [ "/usr/local/bin/entrypoint.sh", "/sbin/init" ]
