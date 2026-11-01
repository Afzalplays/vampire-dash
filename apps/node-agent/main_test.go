package main

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "testing"
)

const testAgentToken = "01234567890123456789012345678901"

func TestHealthHandler(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/health",nil))
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d",w.Code) }
}

func TestNodeStatusRejectsMissingToken(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/v1/node/status",nil))
 if w.Code!=http.StatusUnauthorized { t.Fatalf("want 401, got %d",w.Code) }
}

func TestNodeStatusAcceptsToken(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 r:=httptest.NewRequest(http.MethodGet,"/v1/node/status",nil); r.Header.Set("Authorization","Bearer "+testAgentToken)
 w:=httptest.NewRecorder(); h.ServeHTTP(w,r)
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d",w.Code) }
}

func TestNodeStatusRejectsMalformedOrIncorrectBearer(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 cases:=[]string{"", "bearer "+testAgentToken, "Bearer", "Bearer "+testAgentToken+"x", "Bearer  "+testAgentToken, "Basic "+testAgentToken, "Bearer 00000000000000000000000000000000"}
 for _, authorization:=range cases {
  t.Run(authorization,func(t *testing.T) {
   r:=httptest.NewRequest(http.MethodGet,"/v1/node/status",nil); r.Header.Set("Authorization",authorization)
   w:=httptest.NewRecorder(); h.ServeHTTP(w,r)
   if w.Code!=http.StatusUnauthorized { t.Fatalf("authorization %q: want 401, got %d",authorization,w.Code) }
  })
 }
}

func TestMetricsRequireToken(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken,DataDirectory:"/"})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/v1/node/metrics",nil))
 if w.Code!=http.StatusUnauthorized { t.Fatalf("want 401, got %d",w.Code) }
}

func TestMetricsReturnsCapacity(t *testing.T) {
 dir:=t.TempDir()
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken,DataDirectory:dir})
 r:=httptest.NewRequest(http.MethodGet,"/v1/node/metrics",nil); r.Header.Set("Authorization","Bearer "+testAgentToken)
 w:=httptest.NewRecorder(); h.ServeHTTP(w,r)
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d: %s",w.Code,w.Body.String()) }
 var got metricsResponse
 if err:=json.Unmarshal(w.Body.Bytes(),&got); err!=nil { t.Fatal(err) }
 if got.NodeID!="node-test" || got.CPUCores<1 || got.MemoryTotalMB==0 || got.DiskTotalMB==0 { t.Fatalf("invalid capacity response: %+v",got) }
 if _,err:=os.Stat(dir); err!=nil { t.Fatal(err) }
}

func TestMetricsUnavailableForInvalidDataDirectory(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken,DataDirectory:"/path/that/does/not/exist/vampire-dash"})
 r:=httptest.NewRequest(http.MethodGet,"/v1/node/metrics",nil); r.Header.Set("Authorization","Bearer "+testAgentToken)
 w:=httptest.NewRecorder(); h.ServeHTTP(w,r)
 if w.Code!=http.StatusServiceUnavailable { t.Fatalf("want 503, got %d",w.Code) }
}
