package main

import (
	"bufio"
	"encoding/binary"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	dockerAPIVersion = "v1.45"
	dockerServerLabel = "com.vampirenodes.server_id"
	dockerNodeLabel = "com.vampirenodes.node_id"
	maxDockerResponseBytes = 1 << 20
)

var dockerContainerIDPattern = regexp.MustCompile("^[a-f0-9]{64}$")
var serverIDPattern = regexp.MustCompile("^[A-Za-z0-9_-]{1,128}$")

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
		Image string `json:"Image"`
		Env []string `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		Memory int64 `json:"Memory"`
		NanoCpus int64 `json:"NanoCpus"`
		StorageOpt map[string]string `json:"StorageOpt"`
		PortBindings map[string][]struct { HostIP string `json:"HostIp"`; HostPort string `json:"HostPort"` } `json:"PortBindings"`
	} `json:"HostConfig"`
	State struct {
		Running bool `json:"Running"`
	} `json:"State"`
	NetworkSettings struct { Networks map[string]struct { IPAddress string `json:"IPAddress"` } `json:"Networks"` } `json:"NetworkSettings"`
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
			Timeout: 30 * time.Minute,
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

func (d *dockerRuntime) requestBody(ctx context.Context, method, path string, payload any) (*http.Response, error) {
 if d == nil || d.client == nil || d.apiBase == "" { return nil, errors.New("Docker runtime is not configured") }
 encoded, err := json.Marshal(payload)
 if err != nil { return nil, errors.New("unable to encode fixed Docker request") }
 req, err := http.NewRequestWithContext(ctx, method, d.apiBase+path, bytes.NewReader(encoded))
 if err != nil { return nil, errors.New("invalid fixed Docker API request") }
 req.Header.Set("Content-Type", "application/json")
 response, err := d.client.Do(req)
 if err != nil { return nil, fmt.Errorf("Docker API request failed: %w", err) }
 if response.StatusCode >= 300 && response.StatusCode != http.StatusNotModified { _, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096)); response.Body.Close(); return nil, fmt.Errorf("Docker API returned HTTP %d", response.StatusCode) }
 return response, nil
}

func (d *dockerRuntime) pullImage(ctx context.Context, image string) error {
 query := url.Values{}; query.Set("fromImage", image)
 response, err := d.request(ctx, http.MethodPost, "/images/create?"+query.Encode())
 if err != nil { return err }; defer response.Body.Close()
 scanner := bufio.NewScanner(response.Body)
 scanner.Buffer(make([]byte, 64*1024), 256*1024)
 for scanner.Scan() {
  var event struct { Error string `json:"error"`; ErrorDetail struct { Message string `json:"message"` } `json:"errorDetail"` }
  if err := json.Unmarshal(scanner.Bytes(), &event); err != nil { return errors.New("Docker image pull returned invalid progress data") }
  if event.Error != "" { return fmt.Errorf("Docker image pull failed: %s", boundedDockerError(event.Error)) }
  if event.ErrorDetail.Message != "" { return fmt.Errorf("Docker image pull failed: %s", boundedDockerError(event.ErrorDetail.Message)) }
 }
 if err := scanner.Err(); err != nil { return fmt.Errorf("Docker image pull stream failed: %w", err) }
 return nil
}

func boundedDockerError(value string) string { if len(value) > 300 { return value[:300] }; return value }

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

func (d *dockerRuntime) inspectServerContainerDetails(ctx context.Context, containerID, serverID string) (dockerContainerInspect, error) {
 var inspected dockerContainerInspect
 if !dockerContainerIDPattern.MatchString(containerID) { return inspected, errors.New("container identity is invalid") }
 response, err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/json")
 if err != nil { return inspected, err }
 if err := readDockerJSON(response, &inspected); err != nil { return inspected, err }
 if inspected.Config.Labels[dockerServerLabel] != serverID { return inspected, errors.New("Docker container server label does not match the requested server") }
 return inspected, nil
}

func (d *dockerRuntime) inspectServerContainer(ctx context.Context, containerID, serverID string) (bool, error) {
 inspected, err := d.inspectServerContainerDetails(ctx, containerID, serverID)
 if err != nil { return false, err }
 return inspected.State.Running, nil
}

type dockerProvisionPayload struct {
 Name string `json:"name"`
 Software string `json:"software"`
 MinecraftVersion string `json:"minecraftVersion"`
 DockerImage string `json:"dockerImage"`
 JavaMajor int `json:"javaMajor"`
 MemoryMB int64 `json:"memoryMb"`
 DiskMB int64 `json:"diskMb"`
 CPUPercent int64 `json:"cpuPercent"`
 Port int `json:"port"`
}
type dockerPortBinding struct { HostIP string `json:"HostIp"`; HostPort string `json:"HostPort"` }
type dockerRestartPolicy struct { Name string `json:"Name"` }
type dockerCreateRequest struct {
 Image string `json:"Image"`
 Env []string `json:"Env"`
 ExposedPorts map[string]map[string]any `json:"ExposedPorts"`
 Labels map[string]string `json:"Labels"`
 HostConfig struct {
  Memory int64 `json:"Memory"`
  NanoCpus int64 `json:"NanoCpus"`
  StorageOpt map[string]string `json:"StorageOpt"`
  PortBindings map[string][]dockerPortBinding `json:"PortBindings"`
  RestartPolicy dockerRestartPolicy `json:"RestartPolicy"`
  CapDrop []string `json:"CapDrop"`
  SecurityOpt []string `json:"SecurityOpt"`
  PidsLimit int64 `json:"PidsLimit"`
  Init bool `json:"Init"`
 } `json:"HostConfig"`
}

func (d *dockerRuntime) provisionServer(ctx context.Context, job signedNodeJob) (map[string]any, error) {
 var payload dockerProvisionPayload
 if err := decodeStrictPayload(job.Payload, &payload); err != nil { return nil, errors.New("invalid provisioning payload") }
 if !dockerImagePattern.MatchString(payload.DockerImage) || !strings.Contains(payload.DockerImage, "@sha256:") { return nil, errors.New("container image must be pinned by SHA-256 digest") }
 if strings.TrimSpace(job.ServerID) == "" || strings.TrimSpace(job.NodeID) == "" { return nil, errors.New("server and node identity are required") }
 if strings.TrimSpace(payload.Name) == "" || len(payload.Name) > 32 || strings.TrimSpace(payload.MinecraftVersion) == "" || len(payload.MinecraftVersion) > 32 || payload.JavaMajor < 8 || payload.JavaMajor > 30 || payload.MemoryMB < 256 || payload.MemoryMB > 4_194_304 || payload.DiskMB < 512 || payload.DiskMB > 16_777_216 || payload.CPUPercent < 10 || payload.CPUPercent > 6400 || payload.Port < 1024 || payload.Port > 65535 { return nil, errors.New("provisioning parameters are outside supported bounds") }
 if !oneOf(payload.Software, "VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD") { return nil, errors.New("unsupported Java server software") }
 if !serverIDPattern.MatchString(job.ServerID) { return nil, errors.New("server ID is not safe for a Docker resource name") }
 listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(payload.Port)))
 if err != nil { return nil, fmt.Errorf("allocated TCP port is already in use: %w", err) }; _ = listener.Close()
 if err := d.pullImage(ctx, payload.DockerImage); err != nil { return nil, err }
 var rconSecret [32]byte
 if _, err := rand.Read(rconSecret[:]); err != nil { return nil, errors.New("unable to generate server console credential") }
 rconPassword := base64.RawURLEncoding.EncodeToString(rconSecret[:])
 request := dockerCreateRequest{Image: payload.DockerImage, Env: []string{"EULA=TRUE", "TYPE="+payload.Software, "VERSION="+payload.MinecraftVersion, "JAVA_VERSION="+strconv.Itoa(payload.JavaMajor), "MEMORY="+strconv.FormatInt(payload.MemoryMB*85/100,10)+"M", "SERVER_PORT=25565", "ENABLE_RCON=true", "RCON_PASSWORD="+rconPassword, "RCON_PORT=25575"}, ExposedPorts: map[string]map[string]any{"25565/tcp": {}}, Labels: map[string]string{dockerServerLabel:job.ServerID, dockerNodeLabel:job.NodeID, "com.vampirenodes.managed":"true"}}
 request.HostConfig.Memory = payload.MemoryMB * 1024 * 1024
 request.HostConfig.NanoCpus = payload.CPUPercent * 10_000_000
 request.HostConfig.StorageOpt = map[string]string{"size":strconv.FormatInt(payload.DiskMB,10)+"m"}
 request.HostConfig.PortBindings = map[string][]dockerPortBinding{"25565/tcp": {{HostIP:"0.0.0.0",HostPort:strconv.Itoa(payload.Port)}}}
 request.HostConfig.RestartPolicy = dockerRestartPolicy{Name:"unless-stopped"}
 request.HostConfig.CapDrop = []string{"ALL"}
 request.HostConfig.SecurityOpt = []string{"no-new-privileges:true"}
 request.HostConfig.PidsLimit = 512
 request.HostConfig.Init = true
 query := url.Values{}; query.Set("name","vampire-"+job.ServerID)
 response, err := d.requestBody(ctx,http.MethodPost,"/containers/create?"+query.Encode(),request)
 if err != nil { return nil,fmt.Errorf("create quota-limited server container: %w",err) }
 var created struct { ID string `json:"Id"` }
 if err := readDockerJSON(response,&created); err != nil { return nil,err }
 if !dockerContainerIDPattern.MatchString(created.ID) { return nil,errors.New("Docker returned an invalid container ID") }
 inspected, err := d.inspectProvisionedContainer(ctx,created.ID,job.ServerID,job.NodeID,payload)
 if err != nil { return nil,err }
 if inspected.State.Running { return nil,errors.New("new server container unexpectedly started before provisioning completed") }
 response, err = d.request(ctx,http.MethodPost,"/containers/"+created.ID+"/start")
 if err != nil { return nil,fmt.Errorf("start provisioned server container: %w",err) }; response.Body.Close()
 running, err := d.inspectProvisionedContainer(ctx,created.ID,job.ServerID,job.NodeID,payload)
 if err != nil { return nil,err }
 if !running.State.Running { return nil,errors.New("provisioned Minecraft container did not reach running state") }
 return map[string]any{"state":"RUNNING","port":payload.Port,"diskLimitMb":payload.DiskMB,"memoryLimitMb":payload.MemoryMB,"cpuPercent":payload.CPUPercent},nil
}

func (d *dockerRuntime) inspectProvisionedContainer(ctx context.Context, containerID, serverID, nodeID string, payload dockerProvisionPayload) (dockerContainerInspect,error) {
 var inspected dockerContainerInspect
 if !dockerContainerIDPattern.MatchString(containerID) { return inspected,errors.New("container identity is invalid") }
 response,err:=d.request(ctx,http.MethodGet,"/containers/"+url.PathEscape(containerID)+"/json"); if err!=nil{return inspected,err}
 if err:=readDockerJSON(response,&inspected);err!=nil{return inspected,err}
 if inspected.Config.Labels[dockerServerLabel]!=serverID || inspected.Config.Labels[dockerNodeLabel]!=nodeID || inspected.Config.Labels["com.vampirenodes.managed"]!="true" { return inspected,errors.New("provisioned container ownership labels do not match") }
 if dockerEnvValue(inspected.Config.Env,"ENABLE_RCON")!="true" || dockerEnvValue(inspected.Config.Env,"RCON_PASSWORD")=="" || dockerEnvValue(inspected.Config.Env,"RCON_PORT")!="25575" { return inspected, errors.New("Docker did not apply private console configuration") }
 if inspected.HostConfig.Memory!=payload.MemoryMB*1024*1024 || inspected.HostConfig.NanoCpus!=payload.CPUPercent*10_000_000 || inspected.HostConfig.StorageOpt["size"]!=strconv.FormatInt(payload.DiskMB,10)+"m" { return inspected,errors.New("Docker did not apply requested memory, CPU and disk limits") }
 binding:=inspected.HostConfig.PortBindings["25565/tcp"]; if len(binding)!=1 || binding[0].HostPort!=strconv.Itoa(payload.Port) { return inspected,errors.New("Docker did not apply reserved TCP port") }
 return inspected,nil
}


// readServerConsole returns a bounded, decoded Docker stdout/stderr tail. The
// server is selected only by its immutable panel-issued label.

func validateConsoleCommand(command string) (string, error) {
 command = strings.TrimSpace(command)
 if command == "" || len(command) > 512 { return "", errors.New("console command must contain 1 to 512 bytes") }
 for _, ch := range command { if ch == 0 || ch == '\n' || ch == '\r' || ch == '\u2028' || ch == '\u2029' || ch < 0x20 || ch == 0x7f { return "", errors.New("console command must be a single line without control characters") } }
 return command, nil
}

func dockerEnvValue(values []string, key string) string {
 prefix := key + "="
 for _, value := range values { if strings.HasPrefix(value, prefix) { return strings.TrimPrefix(value, prefix) } }
 return ""
}

func writeRCONPacket(writer io.Writer, id, packetType int32, body string) error {
 length := 4 + 4 + len(body) + 2
 if length > 8192 { return errors.New("RCON packet exceeds size limit") }
 packet := make([]byte, 4+length)
 binary.LittleEndian.PutUint32(packet[:4], uint32(length))
 binary.LittleEndian.PutUint32(packet[4:8], uint32(id))
 binary.LittleEndian.PutUint32(packet[8:12], uint32(packetType))
 copy(packet[12:], body)
 returnWrite := 12 + len(body)
 packet[returnWrite], packet[returnWrite+1] = 0, 0
 for len(packet) > 0 { written, err := writer.Write(packet); if err != nil { return err }; if written == 0 { return io.ErrShortWrite }; packet = packet[written:] }
 return nil
}

func readRCONPacket(reader io.Reader) (int32, int32, string, error) {
 var header [4]byte
 if _, err := io.ReadFull(reader, header[:]); err != nil { return 0, 0, "", err }
 length := int(binary.LittleEndian.Uint32(header[:]))
 if length < 10 || length > 8192 { return 0, 0, "", errors.New("RCON response length is invalid") }
 packet := make([]byte, length)
 if _, err := io.ReadFull(reader, packet); err != nil { return 0, 0, "", err }
 if packet[length-1] != 0 || packet[length-2] != 0 { return 0, 0, "", errors.New("RCON response terminator is invalid") }
 return int32(binary.LittleEndian.Uint32(packet[:4])), int32(binary.LittleEndian.Uint32(packet[4:8])), string(packet[8:length-2]), nil
}

func executeRCON(conn net.Conn, password, command string) (string, error) {
 if err := writeRCONPacket(conn, 1, 3, password); err != nil { return "", errors.New("unable to authenticate to private server console") }
 authID, authType, _, err := readRCONPacket(conn)
 if err != nil || authID == -1 || authID != 1 || authType != 2 { return "", errors.New("private server console authentication failed") }
 if err := writeRCONPacket(conn, 2, 2, command); err != nil { return "", errors.New("unable to send server console command") }
 responseID, responseType, output, err := readRCONPacket(conn)
 if err != nil || responseID != 2 || responseType != 0 { return "", errors.New("server console did not return a valid command response") }
 if len(output) > 8*1024 { output = output[:8*1024] }
 return output, nil
}

func (d *dockerRuntime) sendServerCommand(ctx context.Context, serverID, nodeID, command string) (string, error) {
 if !serverIDPattern.MatchString(serverID) || !serverIDPattern.MatchString(nodeID) { return "", errors.New("server or node identity is invalid") }
 normalizedCommand, err := validateConsoleCommand(command)
 if err != nil { return "", err }
 command = normalizedCommand
 containerID, err := d.resolveServerContainer(ctx, serverID)
 if err != nil { return "", err }
 inspected, err := d.inspectServerContainerDetails(ctx, containerID, serverID)
 if err != nil { return "", err }
 if inspected.Config.Labels[dockerNodeLabel] != nodeID { return "", errors.New("Docker container node label does not match the registered node") }
 if !inspected.State.Running { return "", errors.New("server must be running to accept console commands") }
 password := dockerEnvValue(inspected.Config.Env, "RCON_PASSWORD")
 if password == "" || dockerEnvValue(inspected.Config.Env, "ENABLE_RCON") != "true" { return "", errors.New("private RCON is not configured for this server") }
 var address string
 for _, network := range inspected.NetworkSettings.Networks {
  ip := net.ParseIP(network.IPAddress)
  if ip != nil && ip.IsPrivate() { address = ip.String(); break }
 }
 if address == "" { return "", errors.New("server has no private Docker network address") }
 conn, err := (&net.Dialer{Timeout: 3*time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(address, "25575"))
 if err != nil { return "", fmt.Errorf("connect to private server console: %w", err) }
 defer conn.Close()
 deadline := time.Now().Add(5*time.Second)
 if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) { deadline = ctxDeadline }
 _ = conn.SetDeadline(deadline)
 return executeRCON(conn, password, command)
}

func (d *dockerRuntime) readServerConsole(ctx context.Context, serverID, nodeID string, tail int) (string, error) {
 if !serverIDPattern.MatchString(serverID) || !serverIDPattern.MatchString(nodeID) || tail < 1 || tail > 200 { return "", errors.New("console request is invalid") }
 containerID, err := d.resolveServerContainer(ctx, serverID)
 if err != nil { return "", err }
 inspected, err := d.inspectServerContainerDetails(ctx, containerID, serverID)
 if err != nil { return "", err }
 if inspected.Config.Labels[dockerNodeLabel] != nodeID { return "", errors.New("Docker container node label does not match the registered node") }
 query := url.Values{}
 query.Set("stdout", "1"); query.Set("stderr", "1"); query.Set("tail", strconv.Itoa(tail)); query.Set("timestamps", "1")
 response, err := d.request(ctx, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/logs?"+query.Encode())
 if err != nil { return "", err }
 defer response.Body.Close()
 data, err := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
 if err != nil { return "", errors.New("unable to read container console output") }
 if len(data) > 64*1024 { return "", errors.New("container console output exceeds size limit") }
 // Docker's non-TTY log stream is multiplexed: 8-byte header followed by
 // payload. Reject malformed frames rather than displaying binary framing.
 var output strings.Builder
 for offset := 0; offset < len(data); {
  if len(data)-offset < 8 { return "", errors.New("Docker returned a truncated console frame") }
  streamType := data[offset]
  if streamType != 1 && streamType != 2 { return "", errors.New("Docker returned an unknown console stream") }
  if data[offset+1] != 0 || data[offset+2] != 0 || data[offset+3] != 0 { return "", errors.New("Docker returned an invalid console frame header") }
  frameLength := int(binary.BigEndian.Uint32(data[offset+4:offset+8]))
  offset += 8
  if frameLength < 0 || frameLength > len(data)-offset { return "", errors.New("Docker returned an invalid console frame length") }
  if output.Len()+frameLength > 48*1024 { return "", errors.New("decoded console output exceeds size limit") }
  output.Write(data[offset:offset+frameLength])
  offset += frameLength
 }
 return output.String(), nil
}

// Execute maps server IDs to exactly one Docker container through a
// panel-owned immutable label. It never accepts container IDs or commands
// from the signed job payload and never invokes a shell.
func (d *dockerRuntime) Execute(ctx context.Context, job signedNodeJob) (map[string]any, error) {
	switch job.Action {
	case "PROVISION_SERVER":
		return d.provisionServer(ctx, job)
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
