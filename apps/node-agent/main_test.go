package main

import (
 "net/http"
 "net/http/httptest"
 "testing"
)
func TestHealthIsPublic(t *testing.T) {
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
