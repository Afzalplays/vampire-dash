package main

import (
 "bytes"
 "context"
 "crypto/tls"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "strings"
 "time"
)

const maxPanelResponseBytes = maxClaimResponseBytes

type panelClient struct {
 baseURL string
 token string
 http *http.Client
}

// newPanelClient accepts only HTTPS and never follows redirects, preventing
// accidental disclosure of the node bearer token to another host.
func newPanelClient(rawURL, token string) (*panelClient, error) {
 if len(token) < 32 { return nil, errors.New("panel bearer token must contain at least 32 characters") }
 u, err := url.Parse(strings.TrimSpace(rawURL))
 if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
  return nil, errors.New("PANEL_API_URL must be an HTTPS URL without credentials, query, or fragment")
 }
 if net.ParseIP(u.Hostname()) == nil && strings.ContainsAny(u.Hostname(), " \t\r\n") {
  return nil, errors.New("panel hostname is invalid")
 }
 u.Path = strings.TrimRight(u.Path, "/")
 transport := http.DefaultTransport.(*http.Transport).Clone()
 transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
 client := &http.Client{
  Transport: transport,
  Timeout: 10 * time.Second,
  CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
 }
 return &panelClient{baseURL: strings.TrimRight(u.String(), "/"), token: token, http: client}, nil
}

// claim performs one bounded claim request. A nil response means the queue is empty.
// The signed body remains untrusted until acceptSignedNodeJob authenticates it.
func (p *panelClient) claim(ctx context.Context) (*claimResponse, []byte, error) {
 req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/agent/jobs/claim", bytes.NewReader([]byte("{}")))
 if err != nil { return nil, nil, fmt.Errorf("build claim request: %w", err) }
 req.Header.Set("Authorization", "Bearer "+p.token)
 req.Header.Set("Content-Type", "application/json")
 req.Header.Set("Accept", "application/json")
 resp, err := p.http.Do(req)
 if err != nil { return nil, nil, fmt.Errorf("panel claim request failed: %w", err) }
 defer resp.Body.Close()
 if resp.StatusCode == http.StatusNoContent {
  _, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
  return nil, nil, nil
 }
 body, err := io.ReadAll(io.LimitReader(resp.Body, maxPanelResponseBytes+1))
 if err != nil { return nil, nil, fmt.Errorf("read panel claim response: %w", err) }
 if len(body) > maxPanelResponseBytes { return nil, nil, errors.New("panel claim response exceeds size limit") }
 if resp.StatusCode != http.StatusOK { return nil, nil, fmt.Errorf("panel claim returned HTTP %d", resp.StatusCode) }
 decoded, signedBody, err := decodeClaimResponse(body, time.Now().UTC())
 if err != nil { return nil, nil, fmt.Errorf("panel claim response rejected: %w", err) }
 return &decoded, signedBody, nil
}
