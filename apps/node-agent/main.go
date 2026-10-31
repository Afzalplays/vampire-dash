package main

import (
 "context"
 "crypto/subtle"
 "crypto/tls"
 "crypto/x509"
 "encoding/json"
 "errors"
 "log"
 "net"
 "net/http"
 "os"
 "os/signal"
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
}
type statusResponse struct {
 Status string `json:"status"`
 NodeID string `json:"nodeId,omitempty"`
 Version string `json:"version,omitempty"`
 Timestamp time.Time `json:"timestamp"`
}
const version = "0.2.0"

func loadConfig() (config, error) {
 c := config{
  ListenAddress: strings.TrimSpace(os.Getenv("AGENT_LISTEN")),
  NodeID: strings.TrimSpace(os.Getenv("AGENT_NODE_ID")),
  AgentToken: os.Getenv("AGENT_TOKEN"),
  TLSCertFile: strings.TrimSpace(os.Getenv("AGENT_TLS_CERT")),
  TLSKeyFile: strings.TrimSpace(os.Getenv("AGENT_TLS_KEY")),
  ClientCAFile: strings.TrimSpace(os.Getenv("AGENT_CLIENT_CA")),
 }
 if c.ListenAddress == "" { c.ListenAddress = "127.0.0.1:8081" }
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

func writeJSON(w http.ResponseWriter, code int, value statusResponse) {
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
func newHandler(c config) http.Handler {
 mux := http.NewServeMux()
 mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, statusResponse{Status:"ok", Version:version, Timestamp:time.Now().UTC()})
 })
 mux.Handle("GET /v1/node/status", requireToken(c.AgentToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, statusResponse{Status:"online", NodeID:c.NodeID, Version:version, Timestamp:time.Now().UTC()})
 })))
 return mux
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
 log.Printf("Vampire Dash node agent %s serving HTTPS+mTLS on %s", version, c.ListenAddress)
 if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed { log.Fatalf("server failed: %v", err) }
}
