package main

import (
 "context"
 "crypto/subtle"
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

type config struct { ListenAddress, NodeID, AgentToken string }
type statusResponse struct {
 Status string `json:"status"`
 NodeID string `json:"nodeId,omitempty"`
 Version string `json:"version,omitempty"`
 Timestamp time.Time `json:"timestamp"`
}
const version = "0.1.0"

func loadConfig() (config, error) {
 c := config{ListenAddress: strings.TrimSpace(os.Getenv("AGENT_LISTEN")), NodeID: strings.TrimSpace(os.Getenv("AGENT_NODE_ID")), AgentToken: os.Getenv("AGENT_TOKEN")}
 if c.ListenAddress == "" { c.ListenAddress = "127.0.0.1:8081" }
 if _, _, err := net.SplitHostPort(c.ListenAddress); err != nil { return c, errors.New("AGENT_LISTEN must be host:port") }
 if c.NodeID == "" { return c, errors.New("AGENT_NODE_ID is required") }
 if len(c.AgentToken) < 32 { return c, errors.New("AGENT_TOKEN must contain at least 32 characters") }
 return c, nil
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
 srv := &http.Server{Addr:c.ListenAddress, Handler:newHandler(c), ReadHeaderTimeout:5*time.Second, ReadTimeout:10*time.Second, WriteTimeout:10*time.Second, IdleTimeout:60*time.Second, MaxHeaderBytes:1<<20}
 signals := make(chan os.Signal, 1); signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
 go func() { <-signals; ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel(); if err := srv.Shutdown(ctx); err != nil { log.Printf("graceful shutdown: %v", err) } }()
 log.Printf("Vampire Dash node agent %s listening on %s", version, c.ListenAddress)
 if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatalf("server failed: %v", err) }
}
