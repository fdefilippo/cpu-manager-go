// metrics/prometheus.go
package metrics

import (
  "bufio"
  "context"
  "fmt"
  "net/http"
  "os"
  "path/filepath"
  "strconv"
  "strings"
  "sync"
  "time"

  "github.com/fdefilippo/cpu-manager-go/config"
  "github.com/fdefilippo/cpu-manager-go/logging"
  "github.com/prometheus/client_golang/prometheus"
  "github.com/prometheus/client_golang/prometheus/promauto"
  "github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusExporter esporta metriche in formato Prometheus.
type PrometheusExporter struct {
  cfg      *config.Config
  logger   *logging.Logger
  registry *prometheus.Registry
  server   *http.Server

  // Metriche base
  cpuTotalUsage     prometheus.Gauge
  cpuUserUsage      prometheus.Gauge
  memoryUsage       prometheus.Gauge    // Memoria totale usata
  activeUsers       prometheus.Gauge
  limitedUsers      prometheus.Gauge
  limitsActive      prometheus.Gauge
  systemLoad        prometheus.Gauge
  totalCores        prometheus.Gauge

  // Metriche con label
  userCPUUsage      *prometheus.GaugeVec
  userMemoryUsage   *prometheus.GaugeVec  // NUOVO: Memoria per utente
  userLimited       *prometheus.GaugeVec
  cgroupCPUQuota    *prometheus.GaugeVec
  cgroupCPUPeriod   *prometheus.GaugeVec
  cgroupMemoryUsage *prometheus.GaugeVec  // NUOVO: Memoria cgroup per utente

  // Metriche counter (solo incremento)
  limitsActivatedTotal   prometheus.Counter
  limitsDeactivatedTotal prometheus.Counter
  controlCyclesTotal     prometheus.Counter
  errorsTotal           *prometheus.CounterVec

  // Metriche histogram per tempi di esecuzione
  controlCycleDuration   prometheus.Histogram
  metricsCollectionDuration prometheus.Histogram

  // Cache per evitare aggiornamenti troppo frequenti
  lastUpdate     time.Time
  updateInterval time.Duration
  mu             sync.RWMutex

  // Stato interno
  isRunning bool
  stopChan  chan struct{}
}

// NewPrometheusExporter crea un nuovo esportatore Prometheus.
func NewPrometheusExporter(cfg *config.Config) (*PrometheusExporter, error) {
  // DEBUG: Log per verificare la configurazione
  logger := logging.GetLogger()

  if !cfg.EnablePrometheus {
    logger.Debug("Prometheus exporter disabled by configuration")
    return nil, nil
  }

  logger.Info("Creating Prometheus exporter",
  "host", cfg.PrometheusHost,
  "port", cfg.PrometheusPort,
)

// Verifica che la porta sia valida
if cfg.PrometheusPort <= 0 || cfg.PrometheusPort > 65535 {
  return nil, fmt.Errorf("invalid Prometheus port: %d", cfg.PrometheusPort)
}

exp := &PrometheusExporter{
  cfg:            cfg,
  logger:         logger,
  registry:       prometheus.NewRegistry(),
  updateInterval: 15 * time.Second,
  stopChan:       make(chan struct{}, 1),
}

// Registra metriche
if err := exp.registerMetrics(); err != nil {
  return nil, fmt.Errorf("failed to register metrics: %w", err)
}

// Registra metriche standard di Go
exp.registry.MustRegister(
  prometheus.NewGoCollector(),
  prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
)

logger.Info("Prometheus exporter created successfully")
return exp, nil
}

// registerMetrics registra tutte le metriche Prometheus.
func (exp *PrometheusExporter) registerMetrics() error {
  // Namespace per tutte le metriche
  namespace := "cpu_manager"

  // === Metriche Gauge (valori correnti) ===

  exp.cpuTotalUsage = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "cpu_total_usage_percent",
    Help:      "Total CPU usage percentage across all cores",
  })

  exp.cpuUserUsage = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "cpu_user_usage_percent",
    Help:      "Total CPU usage percentage by non-system users",
  })

  exp.memoryUsage = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "memory_usage_megabytes",
    Help:      "Total memory usage in megabytes",
  })

  exp.activeUsers = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "active_users_count",
    Help:      "Number of active non-system users",
  })

  exp.limitedUsers = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "limited_users_count",
    Help:      "Number of users with CPU limits currently applied",
  })

  exp.limitsActive = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "limits_active",
    Help:      "Whether CPU limits are currently active (1) or not (0)",
  })

  exp.systemLoad = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "system_load_average",
    Help:      "System load average (1 minute)",
  })

  exp.totalCores = promauto.With(exp.registry).NewGauge(prometheus.GaugeOpts{
    Namespace: namespace,
    Name:      "cpu_total_cores",
    Help:      "Total number of CPU cores",
  })

  // === Metriche con label ===

  exp.userCPUUsage = promauto.With(exp.registry).NewGaugeVec(
    prometheus.GaugeOpts{
      Namespace: namespace,
      Name:      "user_cpu_usage_percent",
      Help:      "CPU usage percentage per user",
    },
    []string{"uid", "username"},
  )

  // NUOVA METRICA: Memoria per utente
  exp.userMemoryUsage = promauto.With(exp.registry).NewGaugeVec(
    prometheus.GaugeOpts{
      Namespace: namespace,
      Name:      "user_memory_usage_bytes",
      Help:      "Memory usage in bytes per user",
    },
    []string{"uid", "username"},
  )

  exp.userLimited = promauto.With(exp.registry).NewGaugeVec(
    prometheus.GaugeOpts{
      Namespace: namespace,
      Name:      "user_cpu_limited",
      Help:      "Whether CPU limit is applied for user (1) or not (0)",
    },
    []string{"uid", "username"},
  )

  exp.cgroupCPUQuota = promauto.With(exp.registry).NewGaugeVec(
    prometheus.GaugeOpts{
      Namespace: namespace,
      Name:      "cgroup_cpu_quota_microseconds",
      Help:      "CPU quota in microseconds per period (max = unlimited)",
    },
    []string{"uid", "cgroup_path"},
  )

  exp.cgroupCPUPeriod = promauto.With(exp.registry).NewGaugeVec(
    prometheus.GaugeOpts{
      Namespace: namespace,
      Name:      "cgroup_cpu_period_microseconds",
      Help:      "CPU period in microseconds",
    },
    []string{"uid", "cgroup_path"},
  )

  // NUOVA METRICA: Memoria cgroup per utente
  exp.cgroupMemoryUsage = promauto.With(exp.registry).NewGaugeVec(
    prometheus.GaugeOpts{
      Namespace: namespace,
      Name:      "cgroup_memory_usage_bytes",
      Help:      "Memory usage in bytes per cgroup (user)",
    },
    []string{"uid", "cgroup_path"},
  )

  // === Metriche Counter (solo incremento) ===

  exp.limitsActivatedTotal = promauto.With(exp.registry).NewCounter(prometheus.CounterOpts{
    Namespace: namespace,
    Name:      "limits_activated_total",
    Help:      "Total number of times CPU limits were activated",
  })

  exp.limitsDeactivatedTotal = promauto.With(exp.registry).NewCounter(prometheus.CounterOpts{
    Namespace: namespace,
    Name:      "limits_deactivated_total",
    Help:      "Total number of times CPU limits were deactivated",
  })

  exp.controlCyclesTotal = promauto.With(exp.registry).NewCounter(prometheus.CounterOpts{
    Namespace: namespace,
    Name:      "control_cycles_total",
    Help:      "Total number of control cycles executed",
  })

  exp.errorsTotal = promauto.With(exp.registry).NewCounterVec(
    prometheus.CounterOpts{
      Namespace: namespace,
      Name:      "errors_total",
      Help:      "Total number of errors by type",
    },
    []string{"component", "error_type"},
  )

  // === Metriche Histogram (distribuzione) ===

  exp.controlCycleDuration = promauto.With(exp.registry).NewHistogram(prometheus.HistogramOpts{
    Namespace: namespace,
    Name:      "control_cycle_duration_seconds",
    Help:      "Duration of control cycles in seconds",
    Buckets:   prometheus.DefBuckets,
  })

  exp.metricsCollectionDuration = promauto.With(exp.registry).NewHistogram(prometheus.HistogramOpts{
    Namespace: namespace,
    Name:      "metrics_collection_duration_seconds",
    Help:      "Duration of metrics collection in seconds",
    Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5},
  })

  return nil
}

// UpdateMetrics aggiorna i valori delle metriche.
func (exp *PrometheusExporter) UpdateMetrics(metrics map[string]float64) {
  if exp == nil {
    return
  }

  exp.mu.Lock()
  defer exp.mu.Unlock()

  // Aggiorna solo se è passato abbastanza tempo dall'ultimo aggiornamento
  now := time.Now()
  if now.Sub(exp.lastUpdate) < exp.updateInterval {
    return
  }
  exp.lastUpdate = now

  // Incrementa il contatore dei cicli
  exp.controlCyclesTotal.Inc()

  // Aggiorna le metriche base
  for key, value := range metrics {
    switch {
    case key == "cpu_total_usage":
      exp.cpuTotalUsage.Set(value)
    case key == "cpu_user_usage":
      exp.cpuUserUsage.Set(value)
    case key == "memory_usage_mb":
      exp.memoryUsage.Set(value)
    case key == "active_users":
      exp.activeUsers.Set(value)
    case key == "limited_users":
      exp.limitedUsers.Set(value)
    case key == "limits_active":
      exp.limitsActive.Set(value)
    case key == "system_load":
      exp.systemLoad.Set(value)
    case key == "total_cores":
      exp.totalCores.Set(value)
    case strings.HasPrefix(key, "user_cpu_usage_"):
      // Formato: user_cpu_usage_1000 (dove 1000 è l'UID)
      parts := strings.Split(key, "_")
      if len(parts) >= 4 {
        uid := parts[3]
        username := exp.getUsernameFromUID(uid)
        exp.userCPUUsage.WithLabelValues(uid, username).Set(value)
      }
    case strings.HasPrefix(key, "user_memory_usage_"):
      // Formato: user_memory_usage_1000 (dove 1000 è l'UID)
      parts := strings.Split(key, "_")
      if len(parts) >= 4 {
        uid := parts[3]
        username := exp.getUsernameFromUID(uid)
        // Converti MB in bytes se necessario
        bytesValue := value
        if strings.HasSuffix(key, "_mb") {
          bytesValue = value * 1024 * 1024
        }
        exp.userMemoryUsage.WithLabelValues(uid, username).Set(bytesValue)
      }
    case strings.HasPrefix(key, "user_limited_"):
      // Formato: user_limited_1000
      parts := strings.Split(key, "_")
      if len(parts) >= 3 {
        uid := parts[2]
        username := exp.getUsernameFromUID(uid)
        exp.userLimited.WithLabelValues(uid, username).Set(value)
      }
    case strings.HasPrefix(key, "cgroup_cpu_quota_"):
      // Formato: cgroup_cpu_quota_1000:/sys/fs/cgroup/...
      exp.updateCgroupMetric(key, value, exp.cgroupCPUQuota)
    case strings.HasPrefix(key, "cgroup_cpu_period_"):
      // Formato: cgroup_cpu_period_1000:/sys/fs/cgroup/...
      exp.updateCgroupMetric(key, value, exp.cgroupCPUPeriod)
    case strings.HasPrefix(key, "cgroup_memory_usage_"):
      // Formato: cgroup_memory_usage_1000:/sys/fs/cgroup/...
      exp.updateCgroupMetric(key, value, exp.cgroupMemoryUsage)
    case key == "control_cycle_duration":
      exp.controlCycleDuration.Observe(value)
    case key == "metrics_collection_duration":
      exp.metricsCollectionDuration.Observe(value)
    }
  }
}

// UpdateUserMetrics aggiorna le metriche specifiche per utente.
func (exp *PrometheusExporter) UpdateUserMetrics(uid int, username string, cpuUsage float64, isLimited bool, cgroupPath, cpuQuota string) {
  if exp == nil {
    return
  }

  exp.mu.Lock()
  defer exp.mu.Unlock()

  uidStr := strconv.Itoa(uid)

  // Se username è vuoto, cerca di ottenerlo
  if username == "" || username == uidStr {
    username = exp.getUsernameFromUID(uidStr)
  }

  // Aggiorna uso CPU dell'utente
  exp.userCPUUsage.WithLabelValues(uidStr, username).Set(cpuUsage)

  // Calcola e aggiorna uso memoria dell'utente
  memoryUsage := exp.getUserMemoryUsage(uid)
  exp.userMemoryUsage.WithLabelValues(uidStr, username).Set(float64(memoryUsage))

  // Aggiorna stato limite
  limitedValue := 0.0
  if isLimited {
    limitedValue = 1.0
  }
  exp.userLimited.WithLabelValues(uidStr, username).Set(limitedValue)

  // Se disponibile, aggiorna le metriche cgroup
  if cgroupPath != "" {
    // Aggiorna quota CPU
    if cpuQuota != "" {
      quota, period := parseCPUQuota(cpuQuota)
      if quota >= 0 {
        exp.cgroupCPUQuota.WithLabelValues(uidStr, cgroupPath).Set(float64(quota))
      }
      if period > 0 {
        exp.cgroupCPUPeriod.WithLabelValues(uidStr, cgroupPath).Set(float64(period))
      }
    }

    // Aggiorna uso memoria del cgroup
    cgroupMemory := exp.getCgroupMemoryUsage(cgroupPath)
    exp.cgroupMemoryUsage.WithLabelValues(uidStr, cgroupPath).Set(float64(cgroupMemory))
  }
}

// getUserMemoryUsage calcola l'uso memoria di un utente in bytes
func (exp *PrometheusExporter) getUserMemoryUsage(uid int) int64 {
  var totalMemory int64

  // Itera su tutti i processi in /proc
  procDir := "/proc"
  entries, err := os.ReadDir(procDir)
  if err != nil {
    exp.logger.Warn("Failed to read /proc directory for memory stats", "error", err)
    return 0
  }

  for _, entry := range entries {
    if !entry.IsDir() {
      continue
    }

    // Verifica se è una directory PID
    _, err := strconv.Atoi(entry.Name())
    if err != nil {
      continue
    }

    // Leggi l'UID del processo
    statusFile := filepath.Join(procDir, entry.Name(), "status")
    if procUID, err := exp.getUIDFromStatusFile(statusFile); err == nil && procUID == uid {
      // Leggi l'uso memoria del processo
      statmFile := filepath.Join(procDir, entry.Name(), "statm")
      if data, err := os.ReadFile(statmFile); err == nil {
        fields := strings.Fields(string(data))
        if len(fields) >= 2 {
          // Campo 1 è la dimensione residente in pagine
          pages, err := strconv.ParseInt(fields[1], 10, 64)
          if err == nil {
            // Converti pagine in bytes (tipicamente 4096 bytes per pagina)
            totalMemory += pages * 4096
          }
        }
      }
    }
  }

  return totalMemory
}
// getCgroupMemoryUsage legge l'uso memoria da un cgroup specifico
func (exp *PrometheusExporter) getCgroupMemoryUsage(cgroupPath string) int64 {
  memoryCurrentFile := filepath.Join(cgroupPath, "memory.current")

  if data, err := os.ReadFile(memoryCurrentFile); err == nil {
    if usage, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
      return usage
    }
  }

  return 0
}

// UpdateSystemMetrics aggiorna le metriche di sistema.
func (exp *PrometheusExporter) UpdateSystemMetrics(totalCores int, systemLoad float64) {
  if exp == nil {
    return
  }

  exp.mu.Lock()
  defer exp.mu.Unlock()

  exp.totalCores.Set(float64(totalCores))
  exp.systemLoad.Set(systemLoad)
}

// Helper per leggere UID da file status
func (exp *PrometheusExporter) getUIDFromStatusFile(statusFile string) (int, error) {
  file, err := os.Open(statusFile)
  if err != nil {
    return 0, err
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    line := scanner.Text()
    if strings.HasPrefix(line, "Uid:") {
      fields := strings.Fields(line)
      if len(fields) >= 2 {
        uid, err := strconv.Atoi(fields[1])
        if err != nil {
          return 0, err
        }
        return uid, nil
      }
    }
  }

  return 0, fmt.Errorf("UID not found in status file")
}

// parseCPUQuota estrae quota e period da una stringa "quota period".
func parseCPUQuota(quotaStr string) (quota int64, period int64) {
  parts := strings.Fields(quotaStr)
  if len(parts) != 2 {
    return -1, -1
  }

  if parts[0] == "max" {
    quota = -1 // Indica "max" (illimitato)
  } else {
    if val, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
      quota = val
    }
  }

  if val, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
    period = val
  }

  return quota, period
}

// getUsernameFromUID converte un UID in username.
func (exp *PrometheusExporter) getUsernameFromUID(uidStr string) string {
  uid, err := strconv.Atoi(uidStr)
  if err != nil {
    return "unknown"
  }

  // Prova a leggere da /etc/passwd
  file, err := os.Open("/etc/passwd")
  if err != nil {
    return uidStr
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    line := scanner.Text()
    fields := strings.Split(line, ":")
    if len(fields) >= 3 {
      if strconv.Itoa(uid) == fields[2] {
        return fields[0] // Username
      }
    }
  }

  return uidStr
}

// updateCgroupMetric aggiorna una metrica cgroup con parsing delle label.
func (exp *PrometheusExporter) updateCgroupMetric(key string, value float64, metric *prometheus.GaugeVec) {
  // Formato: cgroup_cpu_quota_1000:/sys/fs/cgroup/cpu_manager/user_1000
  if !strings.Contains(key, ":") {
    return
  }

  // Rimuove il prefisso (es: "cgroup_cpu_quota_")
  prefixEnd := strings.Index(key, "_")
  if prefixEnd == -1 {
    return
  }

  // Estrae UID e path
  remaining := key[prefixEnd+1:]
  colonIndex := strings.Index(remaining, ":")
  if colonIndex == -1 {
    return
  }

  uid := remaining[:colonIndex]
  cgroupPath := remaining[colonIndex+1:]

  metric.WithLabelValues(uid, cgroupPath).Set(value)
}

// IncrementLimitsActivated incrementa il contatore di attivazioni limiti.
func (exp *PrometheusExporter) IncrementLimitsActivated() {
  if exp == nil {
    return
  }
  exp.limitsActivatedTotal.Inc()
}

// IncrementLimitsDeactivated incrementa il contatore di disattivazioni limiti.
func (exp *PrometheusExporter) IncrementLimitsDeactivated() {
  if exp == nil {
    return
  }
  exp.limitsDeactivatedTotal.Inc()
}

// RecordControlCycleDuration registra la durata di un ciclo di controllo.
func (exp *PrometheusExporter) RecordControlCycleDuration(duration time.Duration) {
  if exp == nil {
    return
  }
  exp.controlCycleDuration.Observe(duration.Seconds())
}

// RecordMetricsCollectionDuration registra la durata della raccolta metriche.
func (exp *PrometheusExporter) RecordMetricsCollectionDuration(duration time.Duration) {
  if exp == nil {
    return
  }
  exp.metricsCollectionDuration.Observe(duration.Seconds())
}

// RecordError incrementa il contatore errori per un componente specifico.
func (exp *PrometheusExporter) RecordError(component, errorType string) {
  if exp == nil {
    return
  }
  exp.errorsTotal.WithLabelValues(component, errorType).Inc()
}

// Start avvia il server HTTP per Prometheus.
func (exp *PrometheusExporter) Start(ctx context.Context) error {
  if exp == nil {
    return nil
  }

  exp.mu.Lock()
  if exp.isRunning {
    exp.mu.Unlock()
    return fmt.Errorf("exporter already running")
  }
  exp.isRunning = true
  exp.mu.Unlock()

  mux := http.NewServeMux()

  // Handler per le metriche
  mux.Handle("/metrics", promhttp.HandlerFor(
    exp.registry,
    promhttp.HandlerOpts{
      Registry:          exp.registry,
      EnableOpenMetrics: true,
    },
  ))

  // Health check endpoint
  mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
  })

  // Root endpoint
  mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
      http.NotFound(w, r)
      return
    }
    w.Header().Set("Content-Type", "text/html")
    fmt.Fprintf(w, `<html><body><h1>CPU Manager Metrics</h1><p><a href="/metrics">Metrics</a></p><p><a href="/health">Health</a></p></body></html>`)
  })

  addr := fmt.Sprintf("%s:%d", exp.cfg.PrometheusHost, exp.cfg.PrometheusPort)
  exp.server = &http.Server{
    Addr:    addr,
    Handler: mux,
  }

  exp.logger.Info("Starting Prometheus HTTP server", "address", addr)

  // Avvia il server in una goroutine
  listenErr := make(chan error, 1)
  go func() {
    if err := exp.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
      exp.logger.Error("Prometheus HTTP server error", "error", err)
      listenErr <- err
    }
  }()

  // Verifica che il server sia effettivamente in ascolto
  go func() {
    time.Sleep(500 * time.Millisecond)
    resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
    if err == nil && resp.StatusCode == 200 {
      exp.logger.Info("Prometheus server verified as running")
      resp.Body.Close()
    } else {
      exp.logger.Warn("Could not verify Prometheus server", "error", err)
    }
  }()

  // Gestione shutdown
  go func() {
    select {
    case <-ctx.Done():
      exp.logger.Info("Context cancelled, shutting down Prometheus server")
      exp.shutdown()
    case err := <-listenErr:
      exp.logger.Error("Server listen error", "error", err)
      exp.shutdown()
    case <-exp.stopChan:
      exp.logger.Info("Stop signal received")
      exp.shutdown()
    }
  }()

  return nil
}

// shutdown esegue lo shutdown graceful del server.
func (exp *PrometheusExporter) shutdown() {
  exp.mu.Lock()
  defer exp.mu.Unlock()

  if !exp.isRunning || exp.server == nil {
    return
  }

  shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()

  exp.logger.Info("Shutting down Prometheus HTTP server")
  if err := exp.server.Shutdown(shutdownCtx); err != nil {
    exp.logger.Error("Error during Prometheus server shutdown", "error", err)
    // Forza la chiusura se lo shutdown graceful fallisce
    exp.server.Close()
  }

  exp.isRunning = false
  exp.logger.Info("Prometheus HTTP server stopped")
}

// Stop ferma il server Prometheus.
func (exp *PrometheusExporter) Stop() error {
  if exp == nil {
    return nil
  }

  select {
  case exp.stopChan <- struct{}{}:
    return nil
  default:
    return fmt.Errorf("stop already in progress")
  }
}

// IsRunning restituisce true se l'esportatore è in esecuzione.
func (exp *PrometheusExporter) IsRunning() bool {
  if exp == nil {
    return false
  }

  exp.mu.RLock()
  defer exp.mu.RUnlock()
  return exp.isRunning
}

// GetMetricsEndpoint restituisce l'endpoint delle metriche.
func (exp *PrometheusExporter) GetMetricsEndpoint() string {
  if exp == nil {
    return ""
  }
  return fmt.Sprintf("http://%s:%d/metrics", exp.cfg.PrometheusHost, exp.cfg.PrometheusPort)
}
