package main

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "testing"
 "strings"
 "time"
)

const testAgentToken = "01234567890123456789012345678901"

func TestHealthHandler(t *testing.T) {
 h:=newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 w:=httptest.NewRecorder(); h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,"/health",nil))
 if w.Code!=http.StatusOK { t.Fatalf("want 200, got %d",w.Code) }
}


func TestServerConsoleRequiresNodeBearerToken(t *testing.T) {
 h := newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 w := httptest.NewRecorder()
 h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/servers/server-test/console", nil))
 if w.Code != http.StatusUnauthorized { t.Fatalf("want 401, got %d", w.Code) }
}


func TestServerConsoleRejectsMultilineCommandBeforeDockerAccess(t *testing.T) {
 h := newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 r := httptest.NewRequest(http.MethodPost, "/v1/servers/server-test/console", strings.NewReader(`{"command":"say hello\\nop @a"}`))
 r.Header.Set("Authorization", "Bearer "+testAgentToken)
 r.Header.Set("Content-Type", "application/json")
 w := httptest.NewRecorder()
 h.ServeHTTP(w, r)
 if w.Code != http.StatusBadRequest { t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String()) }
}

func TestServerConsoleRejectsInvalidTailBeforeDockerAccess(t *testing.T) {
 h := newHandler(config{NodeID:"node-test",AgentToken:testAgentToken})
 r := httptest.NewRequest(http.MethodGet, "/v1/servers/server-test/console?tail=201", nil)
 r.Header.Set("Authorization", "Bearer "+testAgentToken)
 w := httptest.NewRecorder()
 h.ServeHTTP(w, r)
 if w.Code != http.StatusBadRequest { t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String()) }
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

func setAgentConfigEnv(t *testing.T) {
 t.Helper()
 t.Setenv("AGENT_LISTEN","127.0.0.1:8081")
 t.Setenv("AGENT_NODE_ID","node-test")
 t.Setenv("AGENT_TOKEN",testAgentToken)
 t.Setenv("AGENT_TLS_CERT","/tmp/agent.crt")
 t.Setenv("AGENT_TLS_KEY","/tmp/agent.key")
 t.Setenv("AGENT_CLIENT_CA","/tmp/panel-ca.crt")
 t.Setenv("AGENT_DATA_DIR","/var/lib/vampire-agent")
 t.Setenv("AGENT_ENABLE_JOB_EXECUTION","false")
 t.Setenv("PANEL_API_URL","")
 t.Setenv("AGENT_DOCKER_SOCKET","")
 t.Setenv("AGENT_JOB_POLL_SECONDS","")
 t.Setenv("AGENT_LEASE_RENEW_SECONDS","")
}

func TestLoadConfigKeepsJobExecutionDisabledByDefault(t *testing.T) {
 setAgentConfigEnv(t)
 t.Setenv("AGENT_ENABLE_JOB_EXECUTION","")
 c, err := loadConfig()
 if err != nil { t.Fatal(err) }
 if c.EnableJobExecution { t.Fatal("job execution must remain opt-in") }
}

func TestLoadConfigEnablesJobsOnlyWithHTTPSAndPersistentDataDirectory(t *testing.T) {
 setAgentConfigEnv(t)
 t.Setenv("AGENT_ENABLE_JOB_EXECUTION","true")
 t.Setenv("PANEL_API_URL","https://panel.example.test")
 t.Setenv("AGENT_JOB_POLL_SECONDS","7")
 t.Setenv("AGENT_LEASE_RENEW_SECONDS","15")
 c, err := loadConfig()
 if err != nil { t.Fatal(err) }
 if !c.EnableJobExecution || c.DockerSocket != "/var/run/docker.sock" || c.JobPollInterval != 7*time.Second || c.LeaseRenewInterval != 15*time.Second { t.Fatalf("unexpected worker config: %+v",c) }
}

func TestLoadConfigRejectsUnsafeJobWorkerConfiguration(t *testing.T) {
 cases := []struct{name string; panelURL string; dataDir string; poll string}{
  {"plaintext panel URL","http://panel.example.test","/var/lib/vampire-agent","5"},
  {"root data directory","https://panel.example.test","/","5"},
  {"invalid poll interval","https://panel.example.test","/var/lib/vampire-agent","0"},
 }
 for _, tc := range cases { t.Run(tc.name,func(t *testing.T) {
  setAgentConfigEnv(t)
  t.Setenv("AGENT_ENABLE_JOB_EXECUTION","true")
  t.Setenv("PANEL_API_URL",tc.panelURL)
  t.Setenv("AGENT_DATA_DIR",tc.dataDir)
  t.Setenv("AGENT_JOB_POLL_SECONDS",tc.poll)
  if _,err:=loadConfig(); err==nil { t.Fatal("expected unsafe configuration rejection") }
 }) }
}
