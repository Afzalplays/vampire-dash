package main

import (
 "bufio"
 "context"
 "crypto/subtle"
 "crypto/tls"
 "crypto/x509"
 "encoding/json"
 "errors"
 "fmt"
 "log"
 "net"
 "net/http"
 "net/url"
 "os"
 "os/signal"
 "path/filepath"
 "runtime"
 "strings"
 "syscall"
 "time"
)

type config struct {
 ListenAddress string
 NodeID string
 AgentToken string
 TLSCertFile string
 TLSKeyFile string
 ClientCAFile string
 DataDirectory string
 EnableJobExecution bool
 PanelAPIURL string
 DockerSocket string
 JobPollInterval time.Duration
 LeaseRenewInterval time.Duration
}
type statusResponse struct {
 Status string `json:"status"`
 NodeID string `json:"nodeId,omitempty"`
 Version string `json:"version,omitempty"`
 Timestamp time.Time `json:"timestamp"`
}
type metricsResponse struct {
 NodeID string `json:"nodeId"`
 Version string `json:"version"`
 CPUCores int `json:"cpuCores"`
 MemoryTotalMB uint64 `json:"memoryTotalMb"`
 MemoryAvailableMB uint64 `json:"memoryAvailableMb"`
 DiskTotalMB uint64 `json:"diskTotalMb"`
 DiskAvailableMB uint64 `json:"diskAvailableMb"`
 DataDirectory string `json:"dataDirectory"`
 SampledAt time.Time `json:"sampledAt"`
}
const version = "0.4.0"

func loadConfig() (config, error) {
 c := config{
  ListenAddress: strings.TrimSpace(os.Getenv("AGENT_LISTEN")),
  NodeID: strings.TrimSpace(os.Getenv("AGENT_NODE_ID")),
  AgentToken: os.Getenv("AGENT_TOKEN"),
  TLSCertFile: strings.TrimSpace(os.Getenv("AGENT_TLS_CERT")),
  TLSKeyFile: strings.TrimSpace(os.Getenv("AGENT_TLS_KEY")),
  ClientCAFile: strings.TrimSpace(os.Getenv("AGENT_CLIENT_CA")),
  DataDirectory: strings.TrimSpace(os.Getenv("AGENT_DATA_DIR")),
  PanelAPIURL: strings.TrimSpace(os.Getenv("PANEL_API_URL")),
  DockerSocket: strings.TrimSpace(os.Getenv("AGENT_DOCKER_SOCKET")),
 }
 if c.ListenAddress == "" { c.ListenAddress = "127.0.0.1:8081" }
 if c.DataDirectory == "" { c.DataDirectory = "/" }
 enableJobs := strings.TrimSpace(os.Getenv("AGENT_ENABLE_JOB_EXECUTION"))
 if enableJobs != "" && enableJobs != "true" && enableJobs != "false" { return c, errors.New("AGENT_ENABLE_JOB_EXECUTION must be true or false") }
 c.EnableJobExecution = enableJobs == "true"
 pollSeconds := 5
 if raw := strings.TrimSpace(os.Getenv("AGENT_JOB_POLL_SECONDS")); raw != "" { parsed, parseErr := parseUint(raw); if parseErr != nil || parsed < 1 || parsed > 60 { return c, errors.New("AGENT_JOB_POLL_SECONDS must be between 1 and 60") }; pollSeconds = int(parsed) }
 renewSeconds := 20
 if raw := strings.TrimSpace(os.Getenv("AGENT_LEASE_RENEW_SECONDS")); raw != "" { parsed, parseErr := parseUint(raw); if parseErr != nil || parsed < 1 || parsed > 30 { return c, errors.New("AGENT_LEASE_RENEW_SECONDS must be between 1 and 30") }; renewSeconds = int(parsed) }
 c.JobPollInterval = time.Duration(pollSeconds) * time.Second
 c.LeaseRenewInterval = time.Duration(renewSeconds) * time.Second
 if c.EnableJobExecution {
  u, parseErr := url.Parse(c.PanelAPIURL)
  if parseErr != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" { return c, errors.New("PANEL_API_URL must be a valid HTTPS URL") }
  if c.DockerSocket == "" { c.DockerSocket = "/var/run/docker.sock" }
  if !filepath.IsAbs(c.DataDirectory) || c.DataDirectory == "/" { return c, errors.New("AGENT_DATA_DIR must be an explicit persistent directory when job execution is enabled") }
 }
 if _, _, err := net.SplitHostPort(c.ListenAddress); err != nil { return c, errors.New("AGENT_LISTEN must be host:port") }
 if c.NodeID == "" { return c, errors.New("AGENT_NODE_ID is required") }
 if len(c.AgentToken) < 32 { return c, errors.New("AGENT_TOKEN must contain at least 32 characters") }
 if c.TLSCertFile == "" || c.TLSKeyFile == "" || c.ClientCAFile == "" {
  return c, errors.New("AGENT_TLS_CERT, AGENT_TLS_KEY and AGENT_CLIENT_CA are required; plaintext mode is disabled")
 }
 return c, nil
}

func loadTLSConfig(c config) (*tls.Config, error) {
 certificate, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile)
 if err != nil { return nil, errors.New("unable to load agent TLS certificate/key: " + err.Error()) }
 caPEM, err := os.ReadFile(c.ClientCAFile)
 if err != nil { return nil, errors.New("unable to read panel client CA: " + err.Error()) }
 clientCAs := x509.NewCertPool()
 if ok := clientCAs.AppendCertsFromPEM(caPEM); !ok { return nil, errors.New("AGENT_CLIENT_CA contains no valid certificates") }
 return &tls.Config{
  MinVersion: tls.VersionTLS12,
  Certificates: []tls.Certificate{certificate},
  ClientAuth: tls.RequireAndVerifyClientCert,
  ClientCAs: clientCAs,
 }, nil
}

func writeJSON(w http.ResponseWriter, code int, value any) {
 w.Header().Set("Content-Type", "application/json; charset=utf-8")
 w.Header().Set("Cache-Control", "no-store")
 w.Header().Set("X-Content-Type-Options", "nosniff")
 w.WriteHeader(code)
 _ = json.NewEncoder(w).Encode(value)
}

func requireToken(token string, next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  raw := r.Header.Get("Authorization")
  provided := strings.TrimPrefix(raw, "Bearer ")
  if !strings.HasPrefix(raw, "Bearer ") || len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
   http.Error(w, "unauthorized", http.StatusUnauthorized); return
  }
  next.ServeHTTP(w, r)
 })
}

func readMemory() (uint64, uint64, error) {
 file, err := os.Open("/proc/meminfo")
 if err != nil { return 0, 0, err }
 defer file.Close()
 var total, available, free uint64
 scanner := bufio.NewScanner(file)
 for scanner.Scan() {
  fields := strings.Fields(scanner.Text())
  if len(fields) < 2 { continue }
  value, parseErr := parseUint(fields[1])
  if parseErr != nil { continue }
  switch strings.TrimSuffix(fields[0], ":") {
  case "MemTotal": total = value
  case "MemAvailable": available = value
  case "MemFree": free = value
  }
 }
 if err := scanner.Err(); err != nil { return 0, 0, err }
 if total == 0 { return 0, 0, errors.New("MemTotal is missing from /proc/meminfo") }
 if available == 0 { available = free }
 return total / 1024, available / 1024, nil
}

func parseUint(value string) (uint64, error) {
 var result uint64
 if value == "" { return 0, errors.New("empty integer") }
 for _, ch := range value {
  if ch < '0' || ch > '9' { return 0, fmt.Errorf("invalid integer") }
  result = result*10 + uint64(ch-'0')
 }
 return result, nil
}

func readDisk(path string) (uint64, uint64, error) {
 var stat syscall.Statfs_t
 if err := syscall.Statfs(path, &stat); err != nil { return 0, 0, err }
 blockSize := uint64(stat.Bsize)
 return (stat.Blocks * blockSize) / (1024 * 1024), (stat.Bavail * blockSize) / (1024 * 1024), nil
}

func collectMetrics(c config) (metricsResponse, error) {
 memoryTotal, memoryAvailable, err := readMemory()
 if err != nil { return metricsResponse{}, err }
 diskTotal, diskAvailable, err := readDisk(c.DataDirectory)
 if err != nil { return metricsResponse{}, fmt.Errorf("read data directory disk capacity: %w", err) }
 return metricsResponse{
  NodeID:c.NodeID, Version:version, CPUCores:runtime.NumCPU(),
  MemoryTotalMB:memoryTotal, MemoryAvailableMB:memoryAvailable,
  DiskTotalMB:diskTotal, DiskAvailableMB:diskAvailable,
  DataDirectory:c.DataDirectory, SampledAt:time.Now().UTC(),
 }, nil
}

func newHandler(c config) http.Handler {
 mux := http.NewServeMux()
 mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, statusResponse{Status:"ok", Version:version, Timestamp:time.Now().UTC()})
 })
 mux.Handle("GET /v1/node/status", requireToken(c.AgentToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, statusResponse{Status:"online", NodeID:c.NodeID, Version:version, Timestamp:time.Now().UTC()})
 })))
 mux.Handle("GET /v1/node/metrics", requireToken(c.AgentToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  metrics, err := collectMetrics(c)
  if err != nil {
   http.Error(w, "node metrics unavailable", http.StatusServiceUnavailable)
   return
  }
  writeJSON(w, http.StatusOK, metrics)
 })))
 return mux
}

func startJobWorker(ctx context.Context, c config) error {
 if !c.EnableJobExecution { return nil }
 if err := os.MkdirAll(c.DataDirectory, 0700); err != nil { return fmt.Errorf("create agent data directory: %w", err) }
 if err := os.Chmod(c.DataDirectory, 0700); err != nil { return fmt.Errorf("secure agent data directory: %w", err) }
 info, err := os.Lstat(c.DataDirectory)
 if err != nil { return fmt.Errorf("inspect agent data directory: %w", err) }
 if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 { return errors.New("AGENT_DATA_DIR must be a real directory, not a symlink") }
 docker, err := newDockerRuntime(c.DockerSocket)
 if err != nil { return fmt.Errorf("configure Docker runtime: %w", err) }
 panel, err := newPanelClient(c.PanelAPIURL, c.AgentToken)
 if err != nil { return fmt.Errorf("configure panel job client: %w", err) }
 poller := &nodeJobPoller{panel: panel, executor: docker, nodeID: c.NodeID, signingKey: c.AgentToken, journalPath: filepath.Join(c.DataDirectory, "jobs.replay.jsonl"), interval: c.JobPollInterval, leaseRenewInterval: c.LeaseRenewInterval}
 if err := poller.validate(); err != nil { return err }
 go func() { if err := poller.run(ctx); err != nil && !errors.Is(err, context.Canceled) { log.Printf("node job worker stopped: %v", err) } }()
 log.Printf("node job worker enabled; only uniquely labelled server containers can receive START/STOP")
 return nil
}

func main() {
 c, err := loadConfig(); if err != nil { log.Fatalf("configuration error: %v", err) }
 tlsConfig, err := loadTLSConfig(c); if err != nil { log.Fatalf("TLS configuration error: %v", err) }
 srv := &http.Server{
  Addr:c.ListenAddress, Handler:newHandler(c), TLSConfig:tlsConfig,
  ReadHeaderTimeout:5*time.Second, ReadTimeout:10*time.Second,
  WriteTimeout:10*time.Second, IdleTimeout:60*time.Second, MaxHeaderBytes:1<<20,
 }
 signals := make(chan os.Signal, 1); signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
 go func() { <-signals; ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel(); if err := srv.Shutdown(ctx); err != nil { log.Printf("graceful shutdown: %v", err) } }()
 workerCtx, cancelWorker := context.WithCancel(context.Background())
 defer cancelWorker()
 if err := startJobWorker(workerCtx, c); err != nil { log.Fatalf("job worker configuration error: %v", err) }
 log.Printf("Vampire Dash node agent %s serving HTTPS+mTLS on %s", version, c.ListenAddress)
 if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed { log.Fatalf("server failed: %v", err) }
}
