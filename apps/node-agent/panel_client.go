package main

import (
 "bytes"
 "context"
 "crypto/tls"
 "crypto/x509"
 "os"
 "encoding/json"
 "errors"
 "fmt"
 "io"
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

type leaseRequest struct { LeaseToken string `json:"leaseToken"` }
type leaseRenewal struct { OK bool `json:"ok"`; LeaseExpiresAt time.Time `json:"leaseExpiresAt"` }
type jobResultRequest struct {
 LeaseToken string `json:"leaseToken"`
 Status string `json:"status"`
 Result map[string]any `json:"result,omitempty"`
 ErrorCode string `json:"errorCode,omitempty"`
 ErrorMessage string `json:"errorMessage,omitempty"`
}

// newPanelClient accepts only HTTPS and never follows redirects, preventing
// accidental disclosure of the node bearer token to another host.
func newPanelClient(rawURL, token string) (*panelClient, error) { return newPanelClientWithCA(rawURL, token, "") }

// newPanelClientWithCA optionally adds a private panel CA while retaining system roots.
func newPanelClientWithCA(rawURL, token, caFile string) (*panelClient, error) {
 if len(token) < 32 { return nil, errors.New("panel bearer token must contain at least 32 characters") }
 u, err := url.Parse(strings.TrimSpace(rawURL))
 if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
  return nil, errors.New("PANEL_API_URL must be an HTTPS URL without credentials, query, or fragment")
 }
 u.Path = strings.TrimRight(u.Path, "/")
 transport := http.DefaultTransport.(*http.Transport).Clone()
 roots, err := x509.SystemCertPool()
 if err != nil || roots == nil { roots = x509.NewCertPool() }
 if caFile != "" {
  pem, readErr := os.ReadFile(caFile)
  if readErr != nil { return nil, fmt.Errorf("read PANEL_SERVER_CA: %w", readErr) }
  if ok := roots.AppendCertsFromPEM(pem); !ok { return nil, errors.New("PANEL_SERVER_CA contains no valid certificates") }
 }
 transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
 client := &http.Client{Transport: transport, Timeout: 10 * time.Second,
  CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
 return &panelClient{baseURL: strings.TrimRight(u.String(), "/"), token: token, http: client}, nil
}

func (p *panelClient) postJSON(ctx context.Context, path string, payload any) ([]byte, int, error) {
 encoded, err := json.Marshal(payload)
 if err != nil { return nil, 0, fmt.Errorf("encode panel request: %w", err) }
 req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(encoded))
 if err != nil { return nil, 0, fmt.Errorf("build panel request: %w", err) }
 req.Header.Set("Authorization", "Bearer "+p.token)
 req.Header.Set("Content-Type", "application/json")
 req.Header.Set("Accept", "application/json")
 resp, err := p.http.Do(req)
 if err != nil { return nil, 0, fmt.Errorf("panel request failed: %w", err) }
 defer resp.Body.Close()
 body, err := io.ReadAll(io.LimitReader(resp.Body, maxPanelResponseBytes+1))
 if err != nil { return nil, resp.StatusCode, fmt.Errorf("read panel response: %w", err) }
 if len(body) > maxPanelResponseBytes { return nil, resp.StatusCode, errors.New("panel response exceeds size limit") }
 return body, resp.StatusCode, nil
}

// claim performs one bounded claim request. A nil response means the queue is empty.
// The signed body remains untrusted until acceptSignedNodeJob authenticates it.
func (p *panelClient) claim(ctx context.Context) (*claimResponse, []byte, error) {
 body, status, err := p.postJSON(ctx, "/api/agent/jobs/claim", map[string]any{})
 if err != nil { return nil, nil, err }
 if status == http.StatusNoContent { return nil, nil, nil }
 if status != http.StatusOK { return nil, nil, fmt.Errorf("panel claim returned HTTP %d", status) }
 decoded, signedBody, err := decodeClaimResponse(body, time.Now().UTC())
 if err != nil { return nil, nil, fmt.Errorf("panel claim response rejected: %w", err) }
 return &decoded, signedBody, nil
}

func (p *panelClient) acknowledge(ctx context.Context, jobID, leaseToken string) error {
 if !jobIDPattern.MatchString(jobID) || len(leaseToken) != 43 { return errors.New("invalid job acknowledgement parameters") }
 _, status, err := p.postJSON(ctx, "/api/agent/jobs/"+url.PathEscape(jobID)+"/ack", leaseRequest{LeaseToken: leaseToken})
 if err != nil { return err }
 if status != http.StatusOK { return fmt.Errorf("panel acknowledgement returned HTTP %d", status) }
 return nil
}

func (p *panelClient) renew(ctx context.Context, jobID, leaseToken string) (time.Time, error) {
 if !jobIDPattern.MatchString(jobID) || len(leaseToken) != 43 { return time.Time{}, errors.New("invalid lease renewal parameters") }
 body, status, err := p.postJSON(ctx, "/api/agent/jobs/"+url.PathEscape(jobID)+"/renew", leaseRequest{LeaseToken: leaseToken})
 if err != nil { return time.Time{}, err }
 if status != http.StatusOK { return time.Time{}, fmt.Errorf("panel lease renewal returned HTTP %d", status) }
 var response leaseRenewal
 if err := json.Unmarshal(body, &response); err != nil || !response.OK || response.LeaseExpiresAt.IsZero() {
  return time.Time{}, errors.New("panel returned an invalid lease renewal response")
 }
 return response.LeaseExpiresAt, nil
}


type reconciliationResponse struct {
 OK bool `json:"ok"`
 Status string `json:"status"`
}

func (p *panelClient) reconcile(ctx context.Context, jobID, identityHash, state, resultHash string, terminalResult map[string]any) (string, error) {
 if !jobIDPattern.MatchString(jobID) || !replayBodyHashPattern.MatchString(identityHash) {
  return "", errors.New("invalid operation reconciliation identity")
 }
 if state != string(stageExecutionStarted) && state != string(stageResultPending) {
  return "", errors.New("invalid operation reconciliation state")
 }
 payload := map[string]any{"identityHash": identityHash, "state": state}
 if state == string(stageResultPending) {
  if !replayBodyHashPattern.MatchString(resultHash) || terminalResult == nil {
   return "", errors.New("pending result reconciliation requires a valid result hash and payload")
  }
  payload["resultHash"] = resultHash
  payload["terminalResult"] = terminalResult
 } else if resultHash != "" || terminalResult != nil {
  return "", errors.New("execution-started reconciliation cannot include a result")
 }
 body, status, err := p.postJSON(ctx, "/api/agent/jobs/"+url.PathEscape(jobID)+"/reconcile", payload)
 if err != nil { return "", err }
 if status != http.StatusOK { return "", fmt.Errorf("panel reconciliation returned HTTP %d", status) }
 var response reconciliationResponse
 if err := json.Unmarshal(body, &response); err != nil || !response.OK {
  return "", errors.New("panel returned an invalid reconciliation response")
 }
 switch response.Status {
 case "RESULT_COMMITTED", "RESULT_ACCEPTED", "IN_PROGRESS", "UNCERTAIN":
  return response.Status, nil
 default:
  return "", errors.New("panel returned an unsupported reconciliation status")
 }
}

func (p *panelClient) reportResult(ctx context.Context, jobID string, result jobResultRequest) error {
 if !jobIDPattern.MatchString(jobID) || len(result.LeaseToken) != 43 || (result.Status != "SUCCEEDED" && result.Status != "FAILED") {
  return errors.New("invalid job result parameters")
 }
 if result.Status == "FAILED" && result.ErrorCode == "" { return errors.New("failed job result requires an error code") }
 _, status, err := p.postJSON(ctx, "/api/agent/jobs/"+url.PathEscape(jobID)+"/result", result)
 if err != nil { return err }
 if status != http.StatusOK { return fmt.Errorf("panel result submission returned HTTP %d", status) }
 return nil
}
