package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const maxOperationJournalBytes = 32 * 1024 * 1024
const maxJournalResultBytes = 16 * 1024

type operationStage string

const (
	stageAccepted         operationStage = "ACCEPTED"
	stageAckConfirmed     operationStage = "ACK_CONFIRMED"
	stageExecutionStarted operationStage = "EXECUTION_STARTED"
	stageResultPending    operationStage = "RESULT_PENDING"
	stageResultConfirmed  operationStage = "RESULT_CONFIRMED"
)

type operationJournalRecord struct {
	JobID        string          `json:"jobId"`
	NodeID       string          `json:"nodeId"`
	Nonce        string          `json:"nonce"`
	IdentityHash string          `json:"identityHash"`
	Stage        operationStage  `json:"stage"`
	ResultHash   string          `json:"resultHash,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
}

func validOperationStage(stage operationStage) bool {
	switch stage {
	case stageAccepted, stageAckConfirmed, stageExecutionStarted, stageResultPending, stageResultConfirmed:
		return true
	default:
		return false
	}
}

func operationStageNumber(stage operationStage) int {
	switch stage {
	case stageAccepted:
		return 1
	case stageAckConfirmed:
		return 2
	case stageExecutionStarted:
		return 3
	case stageResultPending:
		return 4
	case stageResultConfirmed:
		return 5
	default:
		return 0
	}
}

func validateOperationJournalRecord(record operationJournalRecord) error {
	if !replayJobIDPattern.MatchString(record.JobID) || !replayNoncePattern.MatchString(record.Nonce) ||
		!replayBodyHashPattern.MatchString(record.IdentityHash) || !validOperationStage(record.Stage) {
		return errors.New("invalid operation journal record")
	}
	if record.Stage == stageResultPending || record.Stage == stageResultConfirmed {
		if len(record.Result) == 0 || len(record.Result) > maxJournalResultBytes || !replayBodyHashPattern.MatchString(record.ResultHash) {
			return errors.New("invalid pending result in operation journal")
		}
		var value any
		if err := json.Unmarshal(record.Result, &value); err != nil {
			return errors.New("invalid result JSON in operation journal")
		}
		canonical, err := json.Marshal(value)
		if err != nil || string(canonical) != string(record.Result) {
			return errors.New("operation journal result must be canonical JSON")
		}
		sum := sha256.Sum256(record.Result)
		if hex.EncodeToString(sum[:]) != record.ResultHash {
			return errors.New("operation journal result hash mismatch")
		}
	} else if len(record.Result) != 0 || record.ResultHash != "" {
		return errors.New("result is not valid before RESULT_PENDING")
	}
	return nil
}

// advanceOperationJournal durably appends a monotonic state transition.
// It never authorizes execution; callers must separately enforce the protocol.
func advanceOperationJournal(path string, next operationJournalRecord) error {
	if err := validateOperationJournalRecord(next); err != nil {
		return err
	}
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("operation journal path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create operation journal directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fmt.Errorf("open operation journal: %w", err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock operation journal: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat operation journal: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("operation journal must be a regular file")
	}
	if err := file.Chmod(0600); err != nil {
		return fmt.Errorf("secure operation journal permissions: %w", err)
	}
	if info.Size() > maxOperationJournalBytes {
		return errors.New("operation journal is full; refusing state transition")
	}

	var current *operationJournalRecord
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("rewind operation journal: %w", err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), maxJournalResultBytes+4096)
	for scanner.Scan() {
		var record operationJournalRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil || validateOperationJournalRecord(record) != nil {
			return errors.New("operation journal is corrupt; refusing state transition")
		}
		if record.JobID != next.JobID {
			continue
		}
		if record.NodeID != next.NodeID || record.Nonce != next.Nonce || record.IdentityHash != next.IdentityHash {
			return errors.New("operation journal identity conflict")
		}
		if current == nil || operationStageNumber(record.Stage) > operationStageNumber(current.Stage) {
			copy := record
			current = &copy
		} else if operationStageNumber(record.Stage) == operationStageNumber(current.Stage) &&
			(record.ResultHash != current.ResultHash || string(record.Result) != string(current.Result)) {
			return errors.New("operation journal contains contradictory state")
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read operation journal: %w", err)
	}

	if current != nil {
		oldStage, newStage := operationStageNumber(current.Stage), operationStageNumber(next.Stage)
		if newStage == oldStage && current.ResultHash == next.ResultHash && string(current.Result) == string(next.Result) {
			return nil
		}
		if newStage != oldStage+1 {
			return errors.New("operation journal transition is not monotonic")
		}
	} else if next.Stage != stageAccepted {
		return errors.New("first operation journal state must be ACCEPTED")
	}

	encoded, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("encode operation journal record: %w", err)
	}
	if info.Size()+int64(len(encoded)+1) > maxOperationJournalBytes {
		return errors.New("operation journal capacity reached; refusing state transition")
	}
	if _, err := file.Seek(0, 2); err != nil {
		return fmt.Errorf("seek operation journal: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("append operation journal record: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync operation journal: %w", err)
	}
	return nil
}
