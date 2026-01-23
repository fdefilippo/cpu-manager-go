# CPU Manager Go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org/)
[![RPM Package](https://img.shields.io/badge/RPM-Package-red.svg)](https://github.com/fdefilippo/cpu-manager-go/releases)
[![Prometheus](https://img.shields.io/badge/Metrics-Prometheus-orange.svg)](https://prometheus.io/)

Enterprise-grade dynamic CPU resource management tool using Linux cgroups v2. Automatically limits CPU for non-system users based on configurable thresholds.

## ✨ Features

- **Dynamic CPU limiting** for non-system users (UID 1000-...)
- **Configurable thresholds** for activation and release
- **Absolute CPU limits** using `cpu.max` cgroup controller
- **Prometheus metrics** export with comprehensive dashboard
- **Systemd service** integration with hardening
- **Automatic configuration reload** on file changes
- **Detailed process logging** with process name tracking
- **Load average awareness** (optional)
- **Graceful shutdown** with cleanup
- **Complete man page** documentation

## 📊 Architecture Overview

```mermaid
graph TB
    A[CPU Manager] --> B[Metrics Collector]
    A --> C[Cgroup Manager]
    A --> D[State Manager]
    A --> E[Prometheus Exporter]
    
    B --> F[System Metrics<br/>CPU, Memory, Load]
    C --> G[cgroups v2<br/>cpu.max limits]
    D --> H[Decision Engine<br/>Threshold logic]
    E --> I[HTTP Server<br/>/metrics endpoint]
    
    F --> D
    D --> C
    I --> J[Grafana Dashboard]
    
    K[Configuration File] --> A
    L[SIGHUP Signal] --> A
    
    style A fill:#e1f5fe
    style J fill:#f3e5f5
