package main

import (
 "context"
 "crypto/tls"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
)

func trustTestServer(client *panelClient, server *httptest.Server) {
 client.http.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
  MinVersion: tls.VersionTLS12,
  RootCAs: server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs,
 }
}

func TestNewPanelClientRequiresHTTPSAndStrongToken(t *testing.T) {
 if _, err := newPanelClient("http://panel.example", testAgentToken); err == nil { t.Fatal("expected plaintext URL rejection") }
 if _, err := newPanelClient("https://panel.example", "short"); err == nil { t.Fatal("expected weak token rejection") }
 if _, err := newPanelClient("https://user:pass@panel.example", testAgentToken); err == nil { t.Fatal("expected URL credentials rejection") }
}

func TestPanelClientClaimUsesBearerAndRejectsRedirect(t *testing.T) {
 var authorization string
 server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  authorization = r.Header.Get("Authorization")
  http.Redirect(w, r, "https://attacker.invalid/collect", http.StatusFound)
 }))
 defer server.Close()
 client, err := newPanelClient(server.URL, testAgentToken); if err != nil { t.Fatal(err) }; trustTestServer(client, server)
 _, _, err = client.claim(context.Background())
 if err == nil || !strings.Contains(err.Error(), "HTTP 302") { t.Fatalf("expected redirect blocked, got %v", err) }
 if authorization != "Bearer "+testAgentToken { t.Fatalf("unexpected authorization header: %q", authorization) }
}

func TestPanelClientClaimHandlesEmptyQueue(t *testing.T) {
 server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
 defer server.Close()
 client, err := newPanelClient(server.URL, testAgentToken); if err != nil { t.Fatal(err) }; trustTestServer(client, server)
 response, body, err := client.claim(context.Background())
 if err != nil || response != nil || body != nil { t.Fatalf("expected empty queue, response=%v body=%q err=%v", response, body, err) }
}

func TestPanelClientClaimRejectsOversizedBody(t *testing.T) {
 server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte(strings.Repeat("x", maxPanelResponseBytes+1))) }))
 defer server.Close()
 client, err := newPanelClient(server.URL, testAgentToken); if err != nil { t.Fatal(err) }; trustTestServer(client, server)
 if _, _, err := client.claim(context.Background()); err == nil { t.Fatal("expected oversized response rejection") }
}

const testLeaseToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestPanelClientLeaseLifecycleRequests(t *testing.T) {
 expires := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
 var seen []string
 server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if r.Header.Get("Authorization") != "Bearer "+testAgentToken { t.Errorf("missing bearer auth") }
  seen = append(seen, r.URL.Path)
  switch {
  case strings.HasSuffix(r.URL.Path, "/ack"):
   w.Header().Set("Content-Type", "application/json"); _, _ = w.Write([]byte(`{"ok":true,"status":"ACKNOWLEDGED"}`))
  case strings.HasSuffix(r.URL.Path, "/renew"):
   w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(map[string]any{"ok":true,"leaseExpiresAt":expires.Format(time.RFC3339)})
  case strings.HasSuffix(r.URL.Path, "/result"):
   w.Header().Set("Content-Type", "application/json"); _, _ = w.Write([]byte(`{"ok":true,"status":"SUCCEEDED"}`))
  default: http.NotFound(w,r)
  }
 }))
 defer server.Close()
 client, err := newPanelClient(server.URL, testAgentToken); if err != nil { t.Fatal(err) }; trustTestServer(client, server)
 if err := client.acknowledge(context.Background(), "job_123456789012", testLeaseToken); err != nil { t.Fatal(err) }
 got, err := client.renew(context.Background(), "job_123456789012", testLeaseToken); if err != nil { t.Fatal(err) }
 if !got.Equal(expires) { t.Fatalf("renew expiry mismatch: %v != %v", got, expires) }
 if err := client.reportResult(context.Background(), "job_123456789012", jobResultRequest{LeaseToken:testLeaseToken, Status:"SUCCEEDED", Result:map[string]any{"ok":true}}); err != nil { t.Fatal(err) }
 if len(seen)!=3 || !strings.HasSuffix(seen[0],"/ack") || !strings.HasSuffix(seen[1],"/renew") || !strings.HasSuffix(seen[2],"/result") { t.Fatalf("unexpected request sequence: %v",seen) }
}

func TestPanelClientRejectsInvalidResultBeforeNetwork(t *testing.T) {
 client, err := newPanelClient("https://panel.example", testAgentToken); if err != nil { t.Fatal(err) }
 if err := client.reportResult(context.Background(), "job_123456789012", jobResultRequest{LeaseToken:testLeaseToken,Status:"FAILED"}); err == nil { t.Fatal("failed result without error code must be rejected") }
 if err := client.acknowledge(context.Background(), "../escape", testLeaseToken); err == nil { t.Fatal("invalid job id must be rejected") }
}
