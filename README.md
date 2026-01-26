# CPU Manager Go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/)
[![RPM Package](https://img.shields.io/badge/RPM-Package-red.svg)](https://github.com/fdefilippo/cpu-manager-go/releases)
[![Prometheus](https://img.shields.io/badge/Metrics-Prometheus-orange.svg)](https://prometheus.io/)

Enterprise-grade dynamic CPU resource management tool using Linux cgroups v2. Automatically limits CPU for non-system users based on configurable thresholds.

## ✨ Features

- **Dynamic CPU limiting** for non-system users (UID >=1000)
- **Configurable thresholds** for activation and release
- **Absolute CPU limits** using `cpu.max` cgroup controller
- **Prometheus metrics** export with comprehensive dashboard
- **Systemd service** integration with hardening
- **Automatic configuration reload** on file changes
- **Detailed process logging** with process name tracking
- **Load average awareness** (optional)
- **Graceful shutdown** with cleanup
- **Complete man page** documentation

## Build RPM package
make rpm
### Install
rpm -ivh ~/rpmbuild/RPMS/\*/cpu-manager-go-\*.rpm
### Configure
vi /etc/cpu-manager.conf
### Start service
systemctl enable --now cpu-manager

## Prerequisites: Enabling cgroups v2 on Enterprise Linux ≥ 8
CPU Manager requires cgroups v2 with CPU and cpuset controllers enabled. 
Here's how to enable them on RHEL/CentOS/Rocky/AlmaLinux ≥ 8:
```
# Enable unified cgroup hierarchy
grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"

# Verify the change
grubby --info=ALL | grep "systemd.unified_cgroup_hierarchy"

# Reboot
reboot

# After reboot, enable CPU controllers
echo "+cpu" | sudo tee -a /sys/fs/cgroup/cgroup.subtree_control
echo "+cpuset" | sudo tee -a /sys/fs/cgroup/cgroup.subtree_control
```

Persistent via Systemd Service (Recommended)
Create /etc/systemd/system/cgroup-tweaks.service:
```
[Unit]
Description=Configure cgroup subtree controls
Before=systemd-user-sessions.service
Before=cpu-manager.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'echo "+cpu" >> /sys/fs/cgroup/cgroup.subtree_control'
ExecStart=/bin/sh -c 'echo "+cpuset" >> /sys/fs/cgroup/cgroup.subtree_control'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
```
