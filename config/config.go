/*
 * Copyright (C) 2026 Francesco Defilippo
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */
// config/config.go
package config

import (
    "fmt"
    "os"
    "strconv"
    "strings"
    "reflect"
)

// Config contiene tutti i parametri configurabili dell'applicazione.
type Config struct {
    // Paths
    CgroupRoot         string `config:"CGROUP_ROOT"`
    ScriptCgroupBase   string `config:"SCRIPT_CGROUP_BASE"`
    ConfigFile         string `config:"CONFIG_FILE"` // Ricorsivo, usato all'avvio
    LogFile            string `config:"LOG_FILE"`
    CreatedCgroupsFile string `config:"CREATED_CGROUPS_FILE"`
    MetricsCacheFile   string `config:"METRICS_CACHE_FILE"`
    PrometheusFile     string `config:"PROMETHEUS_FILE"`

    // Timing
    PollingInterval  int `config:"POLLING_INTERVAL"`
    MinActiveTime    int `config:"MIN_ACTIVE_TIME"`
    MetricsCacheTTL  int `config:"METRICS_CACHE_TTL"`

    // Thresholds (percentages)
    CPUThreshold       int `config:"CPU_THRESHOLD"`
    CPUReleaseThreshold int `config:"CPU_RELEASE_THRESHOLD"`

    // CPU limits (cpu.max format: "quota period")
    CPUQuotaNormal   string `config:"CPU_QUOTA_NORMAL"`
    CPUQuotaLimited  string `config:"CPU_QUOTA_LIMITED"`

    // Prometheus
    EnablePrometheus bool   `config:"ENABLE_PROMETHEUS"`
    PrometheusPort   int    `config:"PROMETHEUS_PORT"`
    PrometheusHost   string `config:"PROMETHEUS_HOST"`

    // Prometheus TLS/HTTPS (optional)
    PrometheusTLSEnabled     bool   `config:"PROMETHEUS_TLS_ENABLED"`
    PrometheusTLSCertFile    string `config:"PROMETHEUS_TLS_CERT_FILE"`
    PrometheusTLSKeyFile     string `config:"PROMETHEUS_TLS_KEY_FILE"`
    PrometheusTLSCAFile      string `config:"PROMETHEUS_TLS_CA_FILE"`
    PrometheusTLSMinVersion  string `config:"PROMETHEUS_TLS_MIN_VERSION"`  // 1.0, 1.1, 1.2, 1.3

    // Prometheus Authentication
    PrometheusAuthType     string `config:"PROMETHEUS_AUTH_TYPE"`     // none, basic, jwt, both
    PrometheusAuthUsername string `config:"PROMETHEUS_AUTH_USERNAME"`
    PrometheusAuthPasswordFile string `config:"PROMETHEUS_AUTH_PASSWORD_FILE"`
    PrometheusJWTSecretFile    string `config:"PROMETHEUS_JWT_SECRET_FILE"`
    PrometheusJWTIssuer        string `config:"PROMETHEUS_JWT_ISSUER"`
    PrometheusJWTAudience      string `config:"PROMETHEUS_JWT_AUDIENCE"`
    PrometheusJWTExpiry        int    `config:"PROMETHEUS_JWT_EXPIRY"`  // seconds

    // Logging
    LogLevel   string `config:"LOG_LEVEL"`
    LogMaxSize int    `config:"LOG_MAX_SIZE"` // in bytes
    UseSyslog  bool   `config:"USE_SYSLOG"`

    // System
    MinSystemCores int `config:"MIN_SYSTEM_CORES"`
    SystemUIDMin   int `config:"SYSTEM_UID_MIN"`
    SystemUIDMax   int `config:"SYSTEM_UID_MAX"`

    // Load checking
    IgnoreSystemLoad bool `config:"IGNORE_SYSTEM_LOAD"`
}

// DefaultConfig restituisce la configurazione predefinita (come nel tuo script Bash).
func DefaultConfig() *Config {
    // Lettura dinamica del pid_max per il default di SYSTEM_UID_MAX
    pidMax := 60000 // valore di fallback
    if data, err := os.ReadFile("/proc/sys/kernel/pid_max"); err == nil {
        if val, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
            pidMax = val
        }
    }

    return &Config{
        CgroupRoot:         "/sys/fs/cgroup",
        ScriptCgroupBase:   "cpu_manager",
        ConfigFile:         "/etc/cpu-manager.conf",
        LogFile:            "/var/log/cpu-manager.log",
        CreatedCgroupsFile: "/var/run/cpu-manager-cgroups.txt",
        MetricsCacheFile:   "/var/run/cpu-manager-metrics.cache",
        PrometheusFile:     "/var/run/cpu-manager-metrics.prom",

        PollingInterval:  30,
        MinActiveTime:    60,
        MetricsCacheTTL:  15,

        CPUThreshold:       75,
        CPUReleaseThreshold: 40,

        CPUQuotaNormal:  "max 100000",
        CPUQuotaLimited: "50000 100000", // 0.5 core

        EnablePrometheus: false,
        PrometheusPort:   9101,
        PrometheusHost:   "127.0.0.1",

        // Prometheus TLS (disabled by default)
        PrometheusTLSEnabled:     false,
        PrometheusTLSCertFile:    "/etc/cpu-manager/tls/server.crt",
        PrometheusTLSKeyFile:     "/etc/cpu-manager/tls/server.key",
        PrometheusTLSCAFile:      "",
        PrometheusTLSMinVersion:  "1.2",  // TLS 1.2 minimum recommended

        // Prometheus Authentication (disabled by default)
        PrometheusAuthType:     "none",
        PrometheusAuthUsername: "",
        PrometheusAuthPasswordFile: "",
        PrometheusJWTSecretFile:    "",
        PrometheusJWTIssuer:        "cpu-manager",
        PrometheusJWTAudience:      "prometheus",
        PrometheusJWTExpiry:        3600,

        LogLevel:   "INFO",
        LogMaxSize: 10 * 1024 * 1024, // 10MB
        UseSyslog:  false,

        MinSystemCores: 1,
        SystemUIDMin:   1000,
        SystemUIDMax:   pidMax,
        IgnoreSystemLoad: false,
    }
}

// LoadAndValidate carica la configurazione da file e variabili d'ambiente,
// sovrascrivendo i default, e poi la valida.
func LoadAndValidate(configPath string) (*Config, error) {
    cfg := DefaultConfig()

    // 1. Carica dal file di configurazione (se esiste)
    if err := loadFromFile(configPath, cfg); err != nil {
        return nil, fmt.Errorf("loading config file %s: %w", configPath, err)
    }

    // 2. Sovrascrivi con le variabili d'ambiente
    loadFromEnvironment(cfg)

    // 3. Valida
    if err := validateConfig(cfg); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return cfg, nil
}

// loadFromFile legge un file di configurazione in formato chiave=valore.
func loadFromFile(path string, cfg *Config) error {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            // File non esistente è ok, useremo default/env
            return nil
        }
        return err
    }

    lines := strings.Split(string(data), "\n")
    for i, line := range lines {
        line = strings.TrimSpace(line)
        // Salta commenti e righe vuote
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            return fmt.Errorf("malformed config line %d: %s", i+1, line)
        }

        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        // Rimuovi eventuali virgolette
        value = strings.Trim(value, `"'`)

        if err := setConfigField(cfg, key, value); err != nil {
            return fmt.Errorf("setting key %s on line %d: %w", key, i+1, err)
        }
    }
    return nil
}

// loadFromEnvironment sovrascrive i valori con le variabili d'ambiente.
func loadFromEnvironment(cfg *Config) {
    cfgType := reflect.TypeOf(*cfg)
    cfgValue := reflect.ValueOf(cfg).Elem()

    for i := 0; i < cfgType.NumField(); i++ {
        field := cfgType.Field(i)

        // Ottieni il tag 'config' per il nome della variabile d'ambiente
        envKey := field.Tag.Get("config")
        if envKey == "" {
            continue
        }

        // Cerca la variabile d'ambiente
        envValue := os.Getenv(envKey)
        if envValue == "" {
            continue
        }

        // Imposta il valore in base al tipo
        fieldValue := cfgValue.Field(i)
        if !fieldValue.CanSet() {
            continue
        }

        switch field.Type.Kind() {
        case reflect.String:
            fieldValue.SetString(envValue)
        case reflect.Int:
            if intVal, err := strconv.Atoi(envValue); err == nil {
                fieldValue.SetInt(int64(intVal))
            }
        case reflect.Bool:
            lowerVal := strings.ToLower(envValue)
            boolVal := false
            switch lowerVal {
            case "true", "1", "yes", "on":
                boolVal = true
            case "false", "0", "no", "off":
                boolVal = false
            }
            fieldValue.SetBool(boolVal)
        }
    }
}

// setConfigField imposta il valore di un campo nella struct Config basandosi sul tag `config`.
func setConfigField(cfg *Config, key, value string) error {
    switch key {
    // Paths
    case "CGROUP_ROOT":
        cfg.CgroupRoot = value
    case "SCRIPT_CGROUP_BASE":
        cfg.ScriptCgroupBase = value
    case "CONFIG_FILE":
        cfg.ConfigFile = value
    case "LOG_FILE":
        cfg.LogFile = value
    case "CREATED_CGROUPS_FILE":
        cfg.CreatedCgroupsFile = value
    case "METRICS_CACHE_FILE":
        cfg.MetricsCacheFile = value
    case "PROMETHEUS_FILE":
        cfg.PrometheusFile = value

    // Timing
    case "POLLING_INTERVAL":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.PollingInterval = i
        }
    case "MIN_ACTIVE_TIME":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.MinActiveTime = i
        }
    case "METRICS_CACHE_TTL":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.MetricsCacheTTL = i
        }

    // Thresholds
    case "CPU_THRESHOLD":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.CPUThreshold = i
        }
    case "CPU_RELEASE_THRESHOLD":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.CPUReleaseThreshold = i
        }

    // CPU limits
    case "CPU_QUOTA_NORMAL":
        cfg.CPUQuotaNormal = value
    case "CPU_QUOTA_LIMITED":
        cfg.CPUQuotaLimited = value

    // Prometheus
    case "ENABLE_PROMETHEUS":
        switch strings.ToLower(value) {
        case "true", "1", "yes", "on":
            cfg.EnablePrometheus = true
        case "false", "0", "no", "off":
            cfg.EnablePrometheus = false
        default:
            cfg.EnablePrometheus = false
        }
    case "PROMETHEUS_PORT":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.PrometheusPort = i
        }
    case "PROMETHEUS_HOST":
        cfg.PrometheusHost = value

    // Prometheus TLS
    case "PROMETHEUS_TLS_ENABLED":
        switch strings.ToLower(value) {
        case "true", "1", "yes", "on":
            cfg.PrometheusTLSEnabled = true
        case "false", "0", "no", "off":
            cfg.PrometheusTLSEnabled = false
        default:
            cfg.PrometheusTLSEnabled = false
        }
    case "PROMETHEUS_TLS_CERT_FILE":
        cfg.PrometheusTLSCertFile = value
    case "PROMETHEUS_TLS_KEY_FILE":
        cfg.PrometheusTLSKeyFile = value
    case "PROMETHEUS_TLS_CA_FILE":
        cfg.PrometheusTLSCAFile = value
    case "PROMETHEUS_TLS_MIN_VERSION":
        cfg.PrometheusTLSMinVersion = strings.ToUpper(value)

    // Prometheus Authentication
    case "PROMETHEUS_AUTH_TYPE":
        cfg.PrometheusAuthType = strings.ToLower(value)
    case "PROMETHEUS_AUTH_USERNAME":
        cfg.PrometheusAuthUsername = value
    case "PROMETHEUS_AUTH_PASSWORD_FILE":
        cfg.PrometheusAuthPasswordFile = value
    case "PROMETHEUS_JWT_SECRET_FILE":
        cfg.PrometheusJWTSecretFile = value
    case "PROMETHEUS_JWT_ISSUER":
        cfg.PrometheusJWTIssuer = value
    case "PROMETHEUS_JWT_AUDIENCE":
        cfg.PrometheusJWTAudience = value
    case "PROMETHEUS_JWT_EXPIRY":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.PrometheusJWTExpiry = i
        }

    // Logging
    case "LOG_LEVEL":
        cfg.LogLevel = strings.ToUpper(value)
    case "LOG_MAX_SIZE":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.LogMaxSize = i
        }
    case "USE_SYSLOG":
        switch strings.ToLower(value) {
        case "true", "1", "yes", "on":
            cfg.UseSyslog = true
        case "false", "0", "no", "off":
            cfg.UseSyslog = false
        default:
            cfg.UseSyslog = false
        }

    // System
    case "MIN_SYSTEM_CORES":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.MinSystemCores = i
        }
    case "SYSTEM_UID_MIN":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.SystemUIDMin = i
        }
    case "SYSTEM_UID_MAX":
        if i, err := strconv.Atoi(value); err == nil {
            cfg.SystemUIDMax = i
        }
    // Load checking
    case "IGNORE_SYSTEM_LOAD":
        switch strings.ToLower(value) {
        case "true", "1", "yes", "on":
            cfg.IgnoreSystemLoad = true
        case "false", "0", "no", "off":
            cfg.IgnoreSystemLoad = false
        default:
            cfg.IgnoreSystemLoad = false
        }
    default:
        return nil
    }

    return nil
}

// validateConfig esegue tutte le validazioni come nello script Bash.
func validateConfig(cfg *Config) error {
    var errors []string

    // Validate CPU thresholds
    if cfg.CPUThreshold < 1 || cfg.CPUThreshold > 100 {
        errors = append(errors, "CPU_THRESHOLD must be between 1 and 100")
    }
    if cfg.CPUReleaseThreshold < 1 || cfg.CPUReleaseThreshold > 100 {
        errors = append(errors, "CPU_RELEASE_THRESHOLD must be between 1 and 100")
    }
    if cfg.CPUThreshold <= cfg.CPUReleaseThreshold {
        errors = append(errors, "CPU_THRESHOLD must be greater than CPU_RELEASE_THRESHOLD")
    }

    // Validate polling interval
    if cfg.PollingInterval < 5 {
        errors = append(errors, "POLLING_INTERVAL must be at least 5 seconds")
    }

    // Validate CPU quota format
    if !isValidCPUQuota(cfg.CPUQuotaLimited) {
        errors = append(errors, "CPU_QUOTA_LIMITED must be in format 'quota period' or 'max period'")
    }

    // Validate log level
    validLogLevels := map[string]bool{"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true}
    if !validLogLevels[cfg.LogLevel] {
        errors = append(errors, "LOG_LEVEL must be one of: DEBUG, INFO, WARN, ERROR")
    }

    // Validate UID ranges
    if cfg.SystemUIDMin < 0 {
        errors = append(errors, "SYSTEM_UID_MIN cannot be negative")
    }
    if cfg.SystemUIDMax < cfg.SystemUIDMin {
        errors = append(errors, "SYSTEM_UID_MAX must be greater than SYSTEM_UID_MIN")
    }

    if len(errors) > 0 {
        return fmt.Errorf("%s", strings.Join(errors, "; "))
    }
    return nil
}

// isValidCPUQuota verifica il formato "quota period" o "max period".
func isValidCPUQuota(quota string) bool {
    parts := strings.Split(quota, " ")
    if len(parts) != 2 {
        return false
    }
    if parts[0] == "max" {
        _, err := strconv.Atoi(parts[1])
        return err == nil
    }
    // Entrambi devono essere numeri
    _, err1 := strconv.Atoi(parts[0])
    _, err2 := strconv.Atoi(parts[1])
    return err1 == nil && err2 == nil
}
