package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"encoding/json"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
)

type dockerRoundTripFunc func(*http.Request) (*http.Response, error)

func (f dockerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func dockerTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

func dockerTestRuntime(handler dockerRoundTripFunc) *dockerRuntime {
	return &dockerRuntime{
		client: &http.Client{Transport: handler},
		apiBase: "http://docker/" + dockerAPIVersion,
	}
}

const dockerTestContainerID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"


func dockerLogFrame(stream byte, payload string) []byte {
 frame := make([]byte, 8+len(payload))
 frame[0] = stream
 binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
 copy(frame[8:], payload)
 return frame
}

func TestDockerConsoleReadsOnlyLabelOwnedContainerAndDecodesLogs(t *testing.T) {
 expected := dockerLogFrame(1, "2026-11-02T10:00:00Z [Server thread/INFO]: Done!\\n") 
 runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
  switch {
  case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/json":
   if request.URL.Query().Get("all") != "1" { t.Fatal("console must inspect stopped containers too") }
   return dockerTestResponse(http.StatusOK, fmt.Sprintf(`[{"Id":%q,"Labels":{"%s":"server-console"}}]`, dockerTestContainerID, dockerServerLabel)), nil
  case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json":
   return dockerTestResponse(http.StatusOK, fmt.Sprintf(`{"Config":{"Labels":{"%s":"server-console"}},"State":{"Running":true}}`, dockerServerLabel)), nil
  case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/logs":
   if request.URL.Query().Get("stdout") != "1" || request.URL.Query().Get("stderr") != "1" || request.URL.Query().Get("tail") != "50" || request.URL.Query().Get("timestamps") != "1" { t.Fatalf("unsafe or incomplete console query: %v", request.URL.Query()) }
   return &http.Response{StatusCode:http.StatusOK,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(string(expected)))},nil
  default:
   t.Fatalf("unexpected Docker console request: %s %s",request.Method,request.URL.RequestURI())
   return nil,fmt.Errorf("unexpected request")
  }
 })
 output, err := runtime.readServerConsole(context.Background(), "server-console", 50)
 if err != nil { t.Fatal(err) }
 if output != "2026-11-02T10:00:00Z [Server thread/INFO]: Done!\\n" { t.Fatalf("unexpected console output: %q", output) }
}

func TestDockerConsoleRejectsMalformedFramesAndUnsafeLimits(t *testing.T) {
 for _, tc := range []struct{name string; data []byte; tail int}{
  {"truncated header", []byte{1,0,0}, 10},
  {"unknown stream", []byte{9,0,0,0,0,0,0,0}, 10},
  {"truncated payload", []byte{1,0,0,0,0,0,0,4,'x'}, 10},
  {"invalid tail", dockerLogFrame(1,"x"), 201},
 } {
  t.Run(tc.name, func(t *testing.T) {
   runtime := dockerTestRuntime(func(request *http.Request) (*http.Response,error) {
    switch request.URL.Path {
    case "/"+dockerAPIVersion+"/containers/json":
     return dockerTestResponse(http.StatusOK,fmt.Sprintf(`[{"Id":%q,"Labels":{"%s":"server-console"}}]`,dockerTestContainerID,dockerServerLabel)),nil
    case "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json":
     return dockerTestResponse(http.StatusOK,fmt.Sprintf(`{"Config":{"Labels":{"%s":"server-console"}},"State":{"Running":true}}`,dockerServerLabel)),nil
    case "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/logs":
     return &http.Response{StatusCode:http.StatusOK,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(string(tc.data)))},nil
    default: return nil,fmt.Errorf("unexpected Docker request")
    }
   })
   _, err := runtime.readServerConsole(context.Background(),"server-console",tc.tail)
   if err == nil { t.Fatal("unsafe console data or tail limit was accepted") }
  })
 }
}

func TestDockerRuntimeStartsOnlyContainerBoundToServerLabel(t *testing.T) {
	running := false
	var calls []string
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		calls = append(calls, request.Method+" "+request.URL.RequestURI())
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/json":
			filters, err := url.QueryUnescape(request.URL.Query().Get("filters"))
			if err != nil || !strings.Contains(filters, dockerServerLabel+"=server-123") {
				t.Fatalf("server label filter missing: %q err=%v", filters, err)
			}
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`[{"Id":%q,"State":"exited","Labels":{"%s":"server-123"}}]`,
				dockerTestContainerID, dockerServerLabel,
			)), nil
		case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json":
			state := "false"
			if running { state = "true" }
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`{"Config":{"Labels":{"%s":"server-123"}},"State":{"Running":%s}}`,
				dockerServerLabel, state,
			)), nil
		case request.Method == http.MethodPost && request.URL.Path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/start":
			running = true
			return dockerTestResponse(http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected Docker request: %s %s", request.Method, request.URL.RequestURI())
			return nil, fmt.Errorf("unexpected request")
		}
	})
	result, err := runtime.Execute(context.Background(), signedNodeJob{
		ServerID: "server-123", Action: "START", Payload: []byte("{ }"),
	})
	if err != nil { t.Fatal(err) }
	if result["state"] != "RUNNING" { t.Fatalf("unexpected result: %#v", result) }
	if !running { t.Fatal("Docker container did not start") }
	if len(calls) != 4 { t.Fatalf("expected list, inspect, start, inspect; got %v", calls) }
}

func TestDockerRuntimeStopsMappedContainerAndVerifiesState(t *testing.T) {
	running := true
	stopCalled := false
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/json":
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`[{"Id":%q,"Labels":{"%s":"server-stop"}}]`,
				dockerTestContainerID, dockerServerLabel,
			)), nil
		case request.Method == http.MethodGet && request.URL.Path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json":
			state := "false"
			if running { state = "true" }
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`{"Config":{"Labels":{"%s":"server-stop"}},"State":{"Running":%s}}`,
				dockerServerLabel, state,
			)), nil
		case request.Method == http.MethodPost && request.URL.Path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/stop" && request.URL.Query().Get("t") == "30":
			stopCalled = true
			running = false
			return dockerTestResponse(http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected Docker request: %s %s", request.Method, request.URL.RequestURI())
			return nil, fmt.Errorf("unexpected request")
		}
	})
	result, err := runtime.Execute(context.Background(), signedNodeJob{
		ServerID: "server-stop", Action: "STOP", Payload: []byte("{}"),
	})
	if err != nil { t.Fatal(err) }
	if result["state"] != "STOPPED" || running || !stopCalled {
		t.Fatalf("stop was not verified: result=%#v running=%v stopCalled=%v", result, running, stopCalled)
	}
}

func TestDockerRuntimeStartIsIdempotentWhenAlreadyRunning(t *testing.T) {
	calls := 0
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		calls++
		switch request.URL.Path {
		case "/"+dockerAPIVersion+"/containers/json":
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`[{"Id":%q,"Labels":{"%s":"server-running"}}]`,
				dockerTestContainerID, dockerServerLabel,
			)), nil
		case "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json":
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`{"Config":{"Labels":{"%s":"server-running"}},"State":{"Running":true}}`,
				dockerServerLabel,
			)), nil
		default:
			t.Fatalf("already-running container must not receive a mutation: %s %s", request.Method, request.URL.RequestURI())
			return nil, fmt.Errorf("unexpected request")
		}
	})
	result, err := runtime.Execute(context.Background(), signedNodeJob{
		ServerID: "server-running", Action: "START", Payload: []byte("{}"),
	})
	if err != nil { t.Fatal(err) }
	if result["state"] != "RUNNING" || calls != 3 {
		t.Fatalf("start was not idempotent: result=%#v calls=%d", result, calls)
	}
}

func TestDockerRuntimeProvisionsDigestPinnedContainerWithEnforcedLimitsAndPort(t *testing.T) {
 listener, err := net.Listen("tcp", "127.0.0.1:0")
 if err != nil { t.Fatal(err) }
 port := listener.Addr().(*net.TCPAddr).Port
 _ = listener.Close()
 running := false
 pullSeen, createSeen, startSeen := false, false, false
 image := "itzg/minecraft-server@sha256:" + strings.Repeat("a",64)
 payload := fmt.Sprintf(`{"name":"Survival","software":"PAPER","minecraftVersion":"1.21.1","dockerImage":%q,"javaMajor":21,"memoryMb":1024,"diskMb":2048,"cpuPercent":200,"port":%d}`,image,port)
 runtime := dockerTestRuntime(func(request *http.Request) (*http.Response,error) {
  switch {
  case request.Method==http.MethodPost && request.URL.Path=="/"+dockerAPIVersion+"/images/create":
   pullSeen=true
   if request.URL.Query().Get("fromImage")!=image { t.Fatalf("wrong image pull: %s",request.URL.Query().Get("fromImage")) }
   return dockerTestResponse(http.StatusOK,"{\"status\":\"Downloaded\"}\n"),nil
  case request.Method==http.MethodPost && request.URL.Path=="/"+dockerAPIVersion+"/containers/create":
   createSeen=true
   var body map[string]any
   if err:=json.NewDecoder(request.Body).Decode(&body);err!=nil { t.Fatal(err) }
   if body["Image"]!=image { t.Fatalf("wrong image in create request: %#v",body["Image"]) }
   labels:=body["Labels"].(map[string]any)
   if labels[dockerServerLabel]!="server-provision-123" || labels[dockerNodeLabel]!="node-123" { t.Fatalf("ownership labels missing: %#v",labels) }
   host:=body["HostConfig"].(map[string]any)
   if host["Memory"]!=float64(1024*1024*1024) || host["NanoCpus"]!=float64(2_000_000_000) { t.Fatalf("resource limits missing: %#v",host) }
   if host["StorageOpt"].(map[string]any)["size"]!="2048m" { t.Fatalf("disk quota missing: %#v",host["StorageOpt"]) }
   bindings:=host["PortBindings"].(map[string]any)["25565/tcp"].([]any)
   if bindings[0].(map[string]any)["HostPort"]!=strconv.Itoa(port) { t.Fatalf("port binding mismatch: %#v",bindings) }
   return dockerTestResponse(http.StatusCreated,`{"Id":"`+dockerTestContainerID+`"}`),nil
  case request.Method==http.MethodGet && request.URL.Path=="/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json":
   state:="false";if running {state="true"}
   return dockerTestResponse(http.StatusOK,fmt.Sprintf(`{"Config":{"Labels":{"%s":"server-provision-123","%s":"node-123","com.vampirenodes.managed":"true"}},"HostConfig":{"Memory":%d,"NanoCpus":%d,"StorageOpt":{"size":"2048m"},"PortBindings":{"25565/tcp":[{"HostIp":"0.0.0.0","HostPort":"%d"}]}},"State":{"Running":%s}}`,dockerServerLabel,dockerNodeLabel,int64(1024*1024*1024),int64(2_000_000_000),port,state)),nil
  case request.Method==http.MethodPost && request.URL.Path=="/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/start":
   startSeen=true;running=true;return dockerTestResponse(http.StatusNoContent,""),nil
  default: t.Fatalf("unexpected Docker request: %s %s",request.Method,request.URL.RequestURI());return nil,fmt.Errorf("unexpected request")
  }
 })
 result,err:=runtime.Execute(context.Background(),signedNodeJob{ID:"job-provision-123456",NodeID:"node-123",ServerID:"server-provision-123",Action:"PROVISION_SERVER",Payload:[]byte(payload)})
 if err!=nil {t.Fatal(err)}
 if !pullSeen||!createSeen||!startSeen||!running {t.Fatalf("provisioning sequence incomplete: pull=%v create=%v start=%v running=%v",pullSeen,createSeen,startSeen,running)}
 if result["state"]!="RUNNING"||result["port"]!=port||result["diskLimitMb"]!=int64(2048) {t.Fatalf("unexpected provisioning result: %#v",result)}
}

func TestDockerRuntimeRejectsUnpinnedImageBeforeDockerAccess(t *testing.T) {
 calls:=0
 runtime:=dockerTestRuntime(func(request *http.Request)(*http.Response,error){calls++;return dockerTestResponse(http.StatusOK,"{}"),nil})
 payload:=[]byte(`{"name":"Survival","software":"PAPER","minecraftVersion":"1.21.1","dockerImage":"itzg/minecraft-server:latest","javaMajor":21,"memoryMb":1024,"diskMb":2048,"cpuPercent":100,"port":25565}`)
 _,err:=runtime.Execute(context.Background(),signedNodeJob{NodeID:"node-123",ServerID:"server-123",Action:"PROVISION_SERVER",Payload:payload})
 if err==nil||!strings.Contains(err.Error(),"pinned by SHA-256") {t.Fatalf("expected unpinned image rejection, got %v",err)}
 if calls!=0 {t.Fatalf("unpinned image must not contact Docker, calls=%d",calls)}
}

func TestDockerRuntimeDoesNotStartAmbiguousServerIdentity(t *testing.T) {
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		return dockerTestResponse(http.StatusOK, fmt.Sprintf(
			`[{"Id":%q,"Labels":{"%s":"server-123"}},{"Id":%q,"Labels":{"%s":"server-123"}}]`,
			dockerTestContainerID, dockerServerLabel, strings.Repeat("b", 64), dockerServerLabel,
		)), nil
	})
	_, err := runtime.Execute(context.Background(), signedNodeJob{ServerID: "server-123", Action: "START", Payload: []byte("{}")})
	if err == nil || !strings.Contains(err.Error(), "multiple Docker containers") {
		t.Fatalf("expected ambiguous mapping rejection, got %v", err)
	}
}

func TestDockerRuntimeRejectsContainerWithMismatchedInspectLabel(t *testing.T) {
	calls := 0
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Path == "/"+dockerAPIVersion+"/containers/json" {
			return dockerTestResponse(http.StatusOK, fmt.Sprintf(
				`[{"Id":%q,"Labels":{"%s":"server-123"}}]`,
				dockerTestContainerID, dockerServerLabel,
			)), nil
		}
		return dockerTestResponse(http.StatusOK, fmt.Sprintf(
			`{"Config":{"Labels":{"%s":"other-server"}},"State":{"Running":false}}`,
			dockerServerLabel,
		)), nil
	})
	_, err := runtime.Execute(context.Background(), signedNodeJob{ServerID: "server-123", Action: "START", Payload: []byte("{}")})
	if err == nil || !strings.Contains(err.Error(), "label does not match") { t.Fatalf("expected inspect label mismatch, got %v", err) }
	if calls != 2 { t.Fatalf("runtime should stop after failed inspect, calls=%d", calls) }
}

func TestDockerRuntimeFailsClosedForProperties(t *testing.T) {
	calls := 0
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		calls++
		return dockerTestResponse(http.StatusOK, "[]"), nil
	})
	for _, action := range []string{"UPDATE_PROPERTIES"} {
		_, err := runtime.Execute(context.Background(), signedNodeJob{ServerID: "server-123", Action: action, Payload: []byte("{}")})
		if err == nil { t.Fatalf("%s should remain disabled", action) }
	}
	if calls != 0 { t.Fatalf("disabled actions must not contact Docker, calls=%d", calls) }
}

func TestDockerRuntimeRestartStopsThenStartsAndVerifies(t *testing.T) {
 running := true
 var mutations []string
 runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
  path := request.URL.Path
  if path == "/"+dockerAPIVersion+"/containers/json" { return dockerTestResponse(http.StatusOK, fmt.Sprintf(`[{"Id":%q,"Labels":{"%s":"server-restart"}}]`, dockerTestContainerID, dockerServerLabel)), nil }
  if path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/json" { state := "false"; if running { state = "true" }; return dockerTestResponse(http.StatusOK, fmt.Sprintf(`{"Config":{"Labels":{"%s":"server-restart"}},"State":{"Running":%s}}`, dockerServerLabel, state)), nil }
  if request.Method == http.MethodPost && path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/stop" { mutations = append(mutations,"STOP"); running=false; return dockerTestResponse(http.StatusNoContent,""),nil }
  if request.Method == http.MethodPost && path == "/"+dockerAPIVersion+"/containers/"+dockerTestContainerID+"/start" { mutations = append(mutations,"START"); running=true; return dockerTestResponse(http.StatusNoContent,""),nil }
  t.Fatalf("unexpected Docker request: %s %s",request.Method,request.URL.RequestURI()); return nil,fmt.Errorf("unexpected request")
 })
 result, err := runtime.Execute(context.Background(), signedNodeJob{ServerID:"server-restart",Action:"RESTART",Payload:[]byte("{}")})
 if err != nil { t.Fatal(err) }
 if result["state"] != "RUNNING" || !running || strings.Join(mutations,",") != "STOP,START" { t.Fatalf("restart not verified: result=%#v running=%v mutations=%v",result,running,mutations) }
}

func TestDockerRuntimeRejectsPowerPayloadAndUnknownAction(t *testing.T) {
	calls := 0
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		calls++
		return dockerTestResponse(http.StatusOK, "[]"), nil
	})
	for _, job := range []signedNodeJob{
		{ServerID: "server-123", Action: "START", Payload: []byte(`{"containerId":"attacker"}`)},
		{ServerID: "server-123", Action: "RUN_COMMAND", Payload: []byte("{}")},
	} {
		if _, err := runtime.Execute(context.Background(), job); err == nil { t.Fatalf("expected %s rejection", job.Action) }
	}
	if calls != 0 { t.Fatalf("invalid actions must not contact Docker, calls=%d", calls) }
}

func TestNewDockerRuntimeRejectsNonSocketAndSymlink(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "docker.sock")
	if err := os.WriteFile(regular, []byte("not a socket"), 0600); err != nil { t.Fatal(err) }
	if _, err := newDockerRuntime(regular); err == nil { t.Fatal("expected regular file rejection") }
	link := filepath.Join(dir, "docker-link.sock")
	if err := os.Symlink(regular, link); err != nil { t.Fatal(err) }
	if _, err := newDockerRuntime(link); err == nil { t.Fatal("expected symlink rejection") }
	if _, err := newDockerRuntime("relative/docker.sock"); err == nil { t.Fatal("expected relative path rejection") }
}
