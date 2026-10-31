package main

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "testing"
)

func TestHealthHandler(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:"01234567890123456789012345678901"})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/health",nil))
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d",w.Code) }
}
func TestNodeStatusRejectsMissingToken(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:"01234567890123456789012345678901"})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/v1/node/status",nil))
 if w.Code!=http.StatusUnauthorized { t.Fatalf("want 401, got %d",w.Code) }
}
func TestNodeStatusAcceptsToken(t *testing.T) {
 token:="01234567890123456789012345678901"
 h:=newHandler(config{NodeID:"node-test",AgentToken:token})
 r:=httptest.NewRequest(http.MethodGet,"/v1/node/status",nil); r.Header.Set("Authorization","Bearer "+token)
 w:=httptest.NewRecorder(); h.ServeHTTP(w,r)
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d",w.Code) }
}
func TestMetricsRequireToken(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:"01234567890123456789012345678901",DataDirectory:"/"})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/v1/node/metrics",nil))
 if w.Code!=http.StatusUnauthorized { t.Fatalf("want 401, got %d",w.Code) }
}
func TestMetricsReturnsCapacity(t *testing.T) {
 token:="01234567890123456789012345678901"
 dir:=t.TempDir()
 h:=newHandler(config{NodeID:"node-test",AgentToken:token,DataDirectory:dir})
 r:=httptest.NewRequest(http.MethodGet,"/v1/node/metrics",nil); r.Header.Set("Authorization","Bearer "+token)
 w:=httptest.NewRecorder(); h.ServeHTTP(w,r)
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d: %s",w.Code,w.Body.String()) }
 var got metricsResponse
 if err:=json.Unmarshal(w.Body.Bytes(),&got); err!=nil { t.Fatal(err) }
 if got.NodeID!="node-test" || got.CPUCores<1 || got.MemoryTotalMB==0 || got.DiskTotalMB==0 { t.Fatalf("invalid capacity response: %+v",got) }
 if _,err:=os.Stat(dir); err!=nil { t.Fatal(err) }
}
