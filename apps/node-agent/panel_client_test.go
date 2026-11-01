package main

import (
 "context"
 "crypto/tls"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
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
 client, err := newPanelClient(server.URL, testAgentToken)
 if err != nil { t.Fatal(err) }
 trustTestServer(client, server)
 _, _, err = client.claim(context.Background())
 if err == nil || !strings.Contains(err.Error(), "HTTP 302") { t.Fatalf("expected redirect to remain blocked, got %v", err) }
 if authorization != "Bearer "+testAgentToken { t.Fatalf("unexpected authorization header: %q", authorization) }
}

func TestPanelClientClaimHandlesEmptyQueue(t *testing.T) {
 server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
 defer server.Close()
 client, err := newPanelClient(server.URL, testAgentToken)
 if err != nil { t.Fatal(err) }
 trustTestServer(client, server)
 response, body, err := client.claim(context.Background())
 if err != nil || response != nil || body != nil { t.Fatalf("expected empty queue, response=%v body=%q err=%v", response, body, err) }
}

func TestPanelClientClaimRejectsOversizedBody(t *testing.T) {
 server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusOK)
  _, _ = w.Write([]byte(strings.Repeat("x", maxPanelResponseBytes+1)))
 }))
 defer server.Close()
 client, err := newPanelClient(server.URL, testAgentToken)
 if err != nil { t.Fatal(err) }
 trustTestServer(client, server)
 if _, _, err := client.claim(context.Background()); err == nil { t.Fatal("expected oversized response rejection") }
}
