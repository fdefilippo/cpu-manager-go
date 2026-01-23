// main.go
package main

import (
    "context"
    "flag"
    "fmt"
    "net"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/fdefilippo/cpu-manager-go/config"
    "github.com/fdefilippo/cpu-manager-go/cgroup"
    "github.com/fdefilippo/cpu-manager-go/logging"
    "github.com/fdefilippo/cpu-manager-go/metrics"
    "github.com/fdefilippo/cpu-manager-go/state"
    "github.com/fdefilippo/cpu-manager-go/reloader"
)

var version = "dev"

// checkPortAvailable verifica se una porta TCP è disponibile
func checkPortAvailable(host string, port int) bool {
    timeout := time.Second
    conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
    if err != nil {
        return true // Porta disponibile
    }
    if conn != nil {
        conn.Close()
        return false // Porta già in uso
    }
    return true
}

func main() {
    // Parsing dei flag
    configPath := flag.String("config", "/etc/cpu-manager.conf", "Path to configuration file")
    showVersion := flag.Bool("version", false, "Show version and exit")
    flag.Parse()

    if *showVersion {
        fmt.Printf("CPU Manager (Go) %s\n", version)
        return
    }

    // Inizializzazione logger
    logging.InitLogger("INFO", "/var/log/cpu-manager.log", 10*1024*1024)
    logger := logging.GetLogger()
    logger.Info("Starting CPU Manager", "version", version)

    // Caricamento configurazione iniziale
    cfg, err := config.LoadAndValidate(*configPath)
    if err != nil {
        logger.Error("Failed to load configuration", "error", err)
        os.Exit(1)
    }
    logger.Info("Configuration loaded successfully")

    // Riconfigurazione logger con valori dalla config
    logging.InitLogger(cfg.LogLevel, cfg.LogFile, cfg.LogMaxSize)
    logger = logging.GetLogger()
    logger.Debug("Debug logging enabled")

    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Canale per i segnali
    sigChan := make(chan os.Signal, 2) // Buffer di 2 per SIGHUP + SIGINT/TERM
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

    // Inizializzazione componenti
    logger.Info("Initializing components:")

    // 1. Cgroup Manager
    cgroupMgr, err := cgroup.NewManager(cfg)
    if err != nil {
        logger.Error("Failed to initialize cgroup manager", "error", err)
        os.Exit(1)
    }
    logger.Info("Cgroup manager initialized")

    // 2. Metrics Collector
    metricsCollector, err := metrics.NewCollector(cfg)
    if err != nil {
        logger.Error("Failed to initialize metrics collector", "error", err)
        os.Exit(1)
    }

    // 3. Prometheus Exporter
    var prometheusExporter *metrics.PrometheusExporter

    if cfg.EnablePrometheus {
        // Verifica che la porta sia disponibile
        if !checkPortAvailable(cfg.PrometheusHost, cfg.PrometheusPort) {
            logger.Warn("Prometheus port already in use, disabling exporter",
                "host", cfg.PrometheusHost,
                "port", cfg.PrometheusPort,
            )
            cfg.EnablePrometheus = false
        } else {
            prometheusExporter, err = metrics.NewPrometheusExporter(cfg)
            if err != nil {
                logger.Error("Failed to initialize Prometheus exporter", "error", err)
                // Continuiamo senza Prometheus
                prometheusExporter = nil
            } else if prometheusExporter != nil {
                if err := prometheusExporter.Start(ctx); err != nil {
                    logger.Error("Failed to start Prometheus exporter", "error", err)
                    prometheusExporter = nil
                } else {
                    logger.Info("Prometheus exporter started",
                        "host", cfg.PrometheusHost,
                        "port", cfg.PrometheusPort,
                    )
                }
            }
        }
    } else {
        logger.Info("Prometheus exporter disabled by configuration")
    }

    // 4. State Manager
    stateManager, err := state.NewManager(cfg, metricsCollector, cgroupMgr, prometheusExporter)
    if err != nil {
        logger.Error("Failed to initialize state manager", "error", err)
        os.Exit(1)
    }

    // 5. Config Reloader e Watcher
    var configWatcher *config.Watcher

    if *configPath != "" {
      reloader := reloader.NewReloader(stateManager, cgroupMgr, metricsCollector, prometheusExporter)

      // Crea e avvia il watcher di configurazione
      configWatcher, err = config.NewWatcher(*configPath, cfg, reloader)
      if err != nil {
          logger.Warn("Failed to create config watcher, continuing without auto-reload",
              "error", err,
          )
      } else {
          if err := configWatcher.Start(); err != nil {
              logger.Warn("Failed to start config watcher", "error", err)
          } else {
              logger.Info("Configuration auto-reload enabled", "file", *configPath)
          }
      }
    }
    // Goroutine per gestione segnali
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case sig := <-sigChan:
                switch sig {
                case syscall.SIGHUP:
                  logger.Info("Received SIGHUP, forcing configuration reload")
                  if configWatcher != nil {
                    // Usa il metodo pubblico invece del cast
                    go func() {
                      time.Sleep(100 * time.Millisecond)
                      configWatcher.HandleConfigChange()
                    }()
                  } else {
                    logger.Warn("Config watcher not available for SIGHUP reload")
                  }
                case syscall.SIGINT, syscall.SIGTERM:
                    logger.Info("Received termination signal, initiating shutdown",
                        "signal", sig.String(),
                    )
                    cancel()

                    // Timeout per shutdown graceful
                    go func() {
                        time.Sleep(10 * time.Second)
                        logger.Warn("Forced shutdown after timeout")
                        os.Exit(1)
                    }()
                }
            }
        }
    }()

    // Loop principale di controllo
    logger.Info("Entering main control loop", "polling_interval_seconds", cfg.PollingInterval)
    ticker := time.NewTicker(time.Duration(cfg.PollingInterval) * time.Second)
    defer ticker.Stop()

    // Esecuzione immediata del primo controllo
    if err := stateManager.RunControlCycle(ctx); err != nil {
        logger.Error("Error in initial control cycle", "error", err)
    }

    // Main loop
    for {
        select {
        case <-ctx.Done():
            // Shutdown graceful
            logger.Info("Shutting down main control loop")

            // Ferma il watcher di configurazione
            if configWatcher != nil {
                configWatcher.Stop()
            }

            // Pulizia dei componenti
            if err := stateManager.Cleanup(); err != nil {
                logger.Error("Error during cleanup", "error", err)
            }

            logger.Info("Shutdown completed")
            return

        case <-ticker.C:
            // Ciclo di controllo regolare
            startTime := time.Now()
            if err := stateManager.RunControlCycle(ctx); err != nil {
                logger.Error("Error in control cycle", "error", err)
            }

            // Log della durata del ciclo (debug level)
            duration := time.Since(startTime)
            if duration > time.Duration(cfg.PollingInterval/2)*time.Second {
                logger.Warn("Control cycle took longer than expected",
                    "duration_ms", duration.Milliseconds(),
                    "polling_interval_ms", cfg.PollingInterval*1000,
                )
            } else {
                logger.Debug("Control cycle completed",
                    "duration_ms", duration.Milliseconds(),
                )
            }
        }
    }
}
