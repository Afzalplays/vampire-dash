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

func TestDockerRuntimeFailsClosedForRestartAndProperties(t *testing.T) {
	calls := 0
	runtime := dockerTestRuntime(func(request *http.Request) (*http.Response, error) {
		calls++
		return dockerTestResponse(http.StatusOK, "[]"), nil
	})
	for _, action := range []string{"RESTART", "UPDATE_PROPERTIES"} {
		_, err := runtime.Execute(context.Background(), signedNodeJob{ServerID: "server-123", Action: action, Payload: []byte("{}")})
		if err == nil { t.Fatalf("%s should remain disabled", action) }
	}
	if calls != 0 { t.Fatalf("disabled actions must not contact Docker, calls=%d", calls) }
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
