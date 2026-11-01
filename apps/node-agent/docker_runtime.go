package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	dockerAPIVersion = "v1.45"
	dockerServerLabel = "com.vampirenodes.server_id"
	maxDockerResponseBytes = 1 << 20
)

var dockerContainerIDPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type dockerRuntime struct {
	client *http.Client
	apiBase string
}

type dockerContainerSummary struct {
	ID string `json:"Id"`
	State string `json:"State"`
	Labels map[string]string `json:"Labels"`
}

type dockerContainerInspect struct {
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Running bool `json:"Running"`
	} `json:"State"`
}

// newDockerRuntime connects only to a local Unix-domain Docker socket. The
// socket must already exist and must not be a symlink or a regular file.
func newDockerRuntime(socketPath string) (*dockerRuntime, error) {
	if !strings.HasPrefix(socketPath, "/") {
		return nil, errors.New("Docker socket path must be absolute")
	}
	info, err := os.Lstat(socketPath)
	if err != nil {
		return nil, fmt.Errorf("inspect Docker socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("Docker endpoint must be a Unix socket, not a symlink or regular file")
	}
	transport := &http.Transport{
		Proxy: nil,
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &dockerRuntime{
		client: &http.Client{
			Transport: transport,
			Timeout: 20 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		apiBase: "http://docker/" + dockerAPIVersion,
	}, nil
}

func (d *dockerRuntime) request(ctx context.Context, method, path string) (*http.Response, error) {
	if d == nil || d.client == nil || d.apiBase == "" {
		return nil, errors.New("Docker runtime is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, method, d.apiBase+path, nil)
	if err != nil {
		return nil, errors.New("invalid fixed Docker API request")
	}
	response, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Docker API request failed: %w", err)
	}
	if response.StatusCode >= 300 && response.StatusCode != http.StatusNotModified {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		response.Body.Close()
		return nil, fmt.Errorf("Docker API returned HTTP %d", response.StatusCode)
	}
	return response, nil
}

func readDockerJSON(response *http.Response, target any) error {
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDockerResponseBytes+1))
	if err != nil {
		return errors.New("unable to read Docker API response")
	}
	if len(data) > maxDockerResponseBytes {
		return errors.New("Docker API response exceeds size limit")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return errors.New("Docker API returned invalid JSON")
	}
	return nil
}

func (d *dockerRuntime) resolveServerContainer(ctx context.Context, serverID string) (string, error) {
	if strings.TrimSpace(serverID) == "" || len(serverID) > 128 {
		return "", errors.New("server identity is invalid")
	}
	filter, _ := json.Marshal(map[string][]string{
		"label": {dockerServerLabel + "=" + serverID},
	})
	query := url.Values{}
	query.Set("all", "1")
	query.Set("filters", string(filter))
	response, err := d.request(ctx, http.MethodGet, "/containers/json?"+query.Encode())
	if err != nil {
		return "", err
	}
	var containers []dockerContainerSummary
	if err := readDockerJSON(response, &containers); err != nil {
		return "", err
	}
	var selected string
	for _, container := range containers {
		if container.Labels[dockerServerLabel] != serverID {
			continue
		}
		if !dockerContainerIDPattern.MatchString(container.ID) {
			return "", errors.New("Docker returned an invalid container identity")
		}
		if selected != "" {
			return "", errors.New("multiple Docker containers claim the same server identity")
		}
		selected = container.ID
	}
	if selected == "" {
		return "", errors.New("no Docker container is registered for this server identity")
	}
	return selected, nil
}

func (d *dockerRuntime) inspectServerContainer(ctx context.Context, containerID, serverID string) (bool, error) {
	if !dockerContainerIDPattern.MatchString(containerID) {
		return false, errors.New("container identity is invalid")
	}
	response, err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json")
	if err != nil {
		return false, err
	}
	var inspected dockerContainerInspect
	if err := readDockerJSON(response, &inspected); err != nil {
		return false, err
	}
	if inspected.Config.Labels[dockerServerLabel] != serverID {
		return false, errors.New("Docker container server label does not match the requested server")
	}
	return inspected.State.Running, nil
}

// Execute maps server IDs to exactly one Docker container through a
// panel-owned immutable label. It never accepts container IDs or commands
// from the signed job payload and never invokes a shell.
func (d *dockerRuntime) Execute(ctx context.Context, job signedNodeJob) (map[string]any, error) {
	switch job.Action {
	case "START", "STOP":
		var payload powerJobPayload
		if err := decodeStrictPayload(job.Payload, &payload); err != nil {
			return nil, errors.New("power operation payload must be an empty object")
		}
	case "RESTART":
		var payload powerJobPayload
		if err := decodeStrictPayload(job.Payload, &payload); err != nil { return nil, errors.New("power operation payload must be an empty object") }
	case "UPDATE_PROPERTIES":
		return nil, errors.New("UPDATE_PROPERTIES is disabled until a verified server-file adapter is implemented")
	default:
		return nil, errors.New("runtime action is not allowlisted")
	}

	containerID, err := d.resolveServerContainer(ctx, job.ServerID)
	if err != nil {
		return nil, err
	}
	running, err := d.inspectServerContainer(ctx, containerID, job.ServerID)
	if err != nil {
		return nil, err
	}
	var targetState string
	switch job.Action {
	case "START":
		targetState = "RUNNING"
		if !running {
			response, err := d.request(ctx, http.MethodPost, "/containers/"+containerID+"/start")
			if err != nil {
				return nil, err
			}
			response.Body.Close()
		}
	case "STOP":
		targetState = "STOPPED"
		if running {
			response, err := d.request(ctx, http.MethodPost, "/containers/"+containerID+"/stop?t=30")
			if err != nil { return nil, err }
			response.Body.Close()
		}
	case "RESTART":
		if running {
			response, err := d.request(ctx, http.MethodPost, "/containers/"+containerID+"/stop?t=30")
			if err != nil { return nil, err }
			response.Body.Close()
		}
		response, err := d.request(ctx, http.MethodPost, "/containers/"+containerID+"/start")
		if err != nil { return nil, err }
		response.Body.Close()
		targetState = "RUNNING"
	}
	afterRunning, err := d.inspectServerContainer(ctx, containerID, job.ServerID)
	if err != nil {
		return nil, err
	}
	if (targetState == "RUNNING") != afterRunning {
		return nil, errors.New("Docker container did not reach the requested state")
	}
	return map[string]any{"state": targetState}, nil
}
