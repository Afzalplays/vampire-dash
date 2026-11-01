package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const maxJobBodyBytes = 16 * 1024
const maxJobClockSkew = 30 * time.Second
const maxJobLifetime = 2 * time.Minute

var jobIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
var noncePattern = regexp.MustCompile(`^[A-Fa-f0-9]{32,128}$`)

type signedNodeJob struct {
	ID        string          `json:"id"`
	NodeID    string          `json:"nodeId"`
	Nonce     string          `json:"nonce"`
	ServerID  string          `json:"serverId"`
	IssuedAt  time.Time       `json:"issuedAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
}

type powerJobPayload struct{}

type provisionJobPayload struct {
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

var dockerImagePattern = regexp.MustCompile(`^(?:docker.io/)?itzg/minecraft-server@sha256:[a-f0-9]{64}$`)

type propertyJobPayload struct {
	MOTD               *string `json:"motd,omitempty"`
	Difficulty         *string `json:"difficulty,omitempty"`
	GameMode           *string `json:"gamemode,omitempty"`
	MaxPlayers         *int    `json:"maxPlayers,omitempty"`
	PVP                *bool   `json:"pvp,omitempty"`
	OnlineMode         *bool   `json:"onlineMode,omitempty"`
	AllowFlight        *bool   `json:"allowFlight,omitempty"`
	ViewDistance       *int    `json:"viewDistance,omitempty"`
	SimulationDistance *int    `json:"simulationDistance,omitempty"`
	SpawnProtection    *int    `json:"spawnProtection,omitempty"`
}

// validateSignedNodeJob authenticates the exact request bytes and validates a
// bounded, typed operation. It deliberately does not execute or dispatch jobs.
func validateSignedNodeJob(body []byte, signature, secret, nodeID string, now time.Time) (signedNodeJob, error) {
	var job signedNodeJob
	if len(body) == 0 || len(body) > maxJobBodyBytes {
		return job, errors.New("job body size is invalid")
	}
	if len(secret) < 32 || nodeID == "" {
		return job, errors.New("node authentication is not configured")
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size {
		return job, errors.New("job signature is malformed")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return job, errors.New("job signature is invalid")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&job); err != nil {
		return job, fmt.Errorf("invalid job envelope: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return job, errors.New("job body must contain one JSON object")
	}
	if !jobIDPattern.MatchString(job.ID) || !noncePattern.MatchString(job.Nonce) {
		return job, errors.New("job ID or nonce is invalid")
	}
	if job.ServerID == "" || len(job.ServerID) > 128 {
		return job, errors.New("server identity is invalid")
	}
	if job.NodeID != nodeID {
		return job, errors.New("job is addressed to another node")
	}
	if job.IssuedAt.IsZero() || job.ExpiresAt.IsZero() || job.ExpiresAt.Before(job.IssuedAt) ||
		job.IssuedAt.After(now.Add(maxJobClockSkew)) || job.ExpiresAt.Sub(job.IssuedAt) > maxJobLifetime ||
		!now.Before(job.ExpiresAt) {
		return job, errors.New("job timestamp or expiry is invalid")
	}
	if len(job.Payload) == 0 || len(job.Payload) > 8*1024 {
		return job, errors.New("job payload size is invalid")
	}

	switch job.Action {
	case "PROVISION_SERVER":
		var payload provisionJobPayload
		if err := decodeStrictPayload(job.Payload, &payload); err != nil { return job, errors.New("invalid provisioning payload") }
		if strings.TrimSpace(payload.Name) == "" || len(payload.Name) > 32 || payload.MemoryMB < 256 || payload.DiskMB < 512 || payload.CPUPercent < 10 || payload.CPUPercent > 6400 || payload.Port < 1024 || payload.Port > 65535 || payload.JavaMajor < 8 || payload.JavaMajor > 30 || !dockerImagePattern.MatchString(payload.DockerImage) { return job, errors.New("provisioning resource or image parameters are invalid") }
		if !oneOf(payload.Software, "VANILLA", "PAPER", "PURPUR", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY", "BUNGEECORD") { return job, errors.New("provisioning software is not supported by the Java runtime") }
		if strings.TrimSpace(payload.MinecraftVersion) == "" || len(payload.MinecraftVersion) > 32 { return job, errors.New("Minecraft version is invalid") }
	case "START", "STOP", "RESTART":
		var payload powerJobPayload
		if err := decodeStrictPayload(job.Payload, &payload); err != nil {
			return job, errors.New("power operation payload must be an empty object")
		}
	case "UPDATE_PROPERTIES":
		var payload propertyJobPayload
		if err := decodeStrictPayload(job.Payload, &payload); err != nil {
			return job, fmt.Errorf("invalid properties payload: %w", err)
		}
		if payload.MOTD == nil && payload.Difficulty == nil && payload.GameMode == nil &&
			payload.MaxPlayers == nil && payload.PVP == nil && payload.OnlineMode == nil &&
			payload.AllowFlight == nil && payload.ViewDistance == nil &&
			payload.SimulationDistance == nil && payload.SpawnProtection == nil {
			return job, errors.New("properties payload is empty")
		}
		if payload.MOTD != nil && len(*payload.MOTD) > 128 { return job, errors.New("motd is too long") }
		if payload.Difficulty != nil && !oneOf(*payload.Difficulty, "peaceful", "easy", "normal", "hard") { return job, errors.New("difficulty is invalid") }
		if payload.GameMode != nil && !oneOf(*payload.GameMode, "survival", "creative", "adventure", "spectator") { return job, errors.New("gamemode is invalid") }
		if payload.MaxPlayers != nil && (*payload.MaxPlayers < 1 || *payload.MaxPlayers > 500) { return job, errors.New("maxPlayers is out of range") }
		if payload.ViewDistance != nil && (*payload.ViewDistance < 2 || *payload.ViewDistance > 32) { return job, errors.New("viewDistance is out of range") }
		if payload.SimulationDistance != nil && (*payload.SimulationDistance < 2 || *payload.SimulationDistance > 32) { return job, errors.New("simulationDistance is out of range") }
		if payload.SpawnProtection != nil && (*payload.SpawnProtection < 0 || *payload.SpawnProtection > 128) { return job, errors.New("spawnProtection is out of range") }
	default:
		return job, errors.New("job action is not allowlisted")
	}
	return job, nil
}

func decodeStrictPayload(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil { return err }
	if err := decoder.Decode(new(any)); err != io.EOF { return errors.New("payload must contain one JSON value") }
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed { if value == candidate { return true } }
	return false
}
