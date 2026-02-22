# Changelog

Tutti i cambiamenti significativi a questo progetto sono documentati in questo file.

Il formato è basato su [Keep a Changelog](https://keepachangelog.com/it/1.0.0/),
e questo progetto aderisce al [Semantic Versioning](https://semver.org/lang/it/).

## [1.0.0] - 2026-02-22

### Aggiunto

#### Metriche Prometheus per utente
- Nuova metrica `cpu_manager_user_memory_usage_bytes{uid, username}` - Memoria RAM utilizzata per utente (in bytes)
- Nuova metrica `cpu_manager_user_process_count{uid, username}` - Numero di processi per utente
- Nuova metrica `cpu_manager_user_cpu_limited{uid, username}` - Stato limite CPU per utente
- Nuova metrica `cpu_manager_active_users_count` - Numero totale di utenti non-sistema attivi
- Nuova metrica `cpu_manager_system_load_average` - Load average di sistema (1 minuto)
- Nuova metrica `cpu_manager_memory_usage_megabytes` - Memoria totale di sistema utilizzata

#### Dashboard Grafana
- Aggiunto pannello "Memory Usage Per User" per visualizzare memoria per utente
- Aggiunto pannello "Total User Processes" per totale processi utente
- Aggiunto pannello "Processes Per User" per processi per singolo utente
- Aggiunta variabile templating `username` per filtrare per nome utente
- Riorganizzato layout del dashboard per migliore visualizzazione

#### Documentazione
- Aggiornato manuale `docs/cpu-manager.8` con tutte le nuove metriche
- Aggiunti esempi di query Prometheus per utente
- Aggiornato `docs/dashboard-grafana.json` con nuovi pannelli
- Creato file `CHANGELOG.md` per tracciare i cambiamenti

### Corretto

#### Bug fix
- Corretto errore `fmt.Errorf` in `config/config.go` (riga 372) - aggiunto format string costante
- Risolti problemi di compilazione Makefile per pacchetto Debian
- Rimossi loop bash problematici nel Makefile che causavano errori di processi figli

#### Build e Packaging
- Semplificato target `deb-binary` per build sequenziale invece che parallela
- Semplificato target `deb-prepare` per evitare race condition
- Corretto campo `DEB_MAINTAINER` per evitare warning di dpkg-deb
- Build Debian ora completa con successo per architettura amd64

### Modificato

#### API e Interfacce
- Aggiornata interfaccia `MetricsCollector` con nuovo metodo `GetAllUserMetrics()`
- Aggiornata interfaccia `PrometheusExporter` con metodo `CleanupUserMetrics()`
- Modificato `UpdateUserMetrics()` per accettare memoryUsage e processCount come parametri
- Aggiunta struct `UserMetrics` per raggruppare CPU, memoria e processi per utente

#### Implementazione
- `metrics/collector.go`: Implementato `GetAllUserMetrics()` per raccolta efficiente in una sola scansione /proc
- `metrics/collector.go`: Implementato `GetUserMemoryUsage()` per lettura VmRSS da /proc/[pid]/status
- `metrics/collector.go`: Implementato `GetUserProcessCount()` per conteggio processi per UID
- `state/manager.go`: Aggiornato `collectSystemMetrics()` per usare `GetAllUserMetrics()`
- `state/manager.go`: Aggiornato `updatePrometheusMetrics()` per esporre metriche complete per utente

### Rimosso

- Nessun cambiamento di rottura in questa versione

### Note di migrazione

Questa versione è **retrocompatibile**. Tutte le metriche esistenti sono mantenute:

- Le nuove metriche per utente sono additive e non sostituiscono quelle esistenti
- Il dashboard Grafana è stato aggiornato ma rimane importabile come nuovo dashboard
- La configurazione esistente non richiede modifiche

### Requisiti di sistema

Nessun cambiamento nei requisiti di sistema:

- Linux kernel 4.5+ con cgroups v2
- Accesso in scrittura a `/sys/fs/cgroup`
- Privilegi root o capacità CAP_SYS_ADMIN

---

## [0.9.0] - 2026-01-15

### Aggiunto

- Supporto per cgroups v2 con controller CPU e cpuset
- Export metriche Prometheus di base
- Configurazione dinamica con auto-reload
- Graceful shutdown con cleanup
- Logging strutturato con rotazione file
- Supporto syslog opzionale

### Modificato

- Migliorata gestione errori durante il controllo dei cicli
- Ottimizzata cache delle metriche con TTL configurabile

---

## [0.1.0] - 2025-12-01

### Aggiunto

- Implementazione iniziale del daemon CPU Manager
- Controllo soglie CPU per attivazione/disattivazione limiti
- Integrazione con systemd service
- Documentazione base e man page

---

## Formato delle versioni

Il formato delle versioni è `MAJOR.MINOR.PATCH`:

- **MAJOR**: Cambiamenti incompatibili con le versioni precedenti
- **MINOR**: Nuove funzionalità in modo retrocompatibile
- **PATCH**: Correzioni di bug in modo retrocompatibile

## Link

- [1.0.0]: https://github.com/fdefilippo/cpu-manager-go/compare/v0.9.0...v1.0.0
- [0.9.0]: https://github.com/fdefilippo/cpu-manager-go/compare/v0.1.0...v0.9.0
- [0.1.0]: https://github.com/fdefilippo/cpu-manager-go/releases/tag/v0.1.0
