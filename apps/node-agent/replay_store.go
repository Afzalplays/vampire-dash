package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

const maxReplayJournalBytes = 32 * 1024 * 1024

var replayJobIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
var replayNoncePattern = regexp.MustCompile(`^[A-Fa-f0-9]{32,128}$`)

type replayRecord struct {
	JobID string `json:"jobId"`
	Nonce string `json:"nonce"`
}

// consumeJobReplayKey durably reserves a job ID and nonce before any future
// handler is allowed to acknowledge or execute the job. The journal is
// append-only; a full or corrupt journal fails closed.
func consumeJobReplayKey(journalPath, jobID, nonce string) error {
	if !replayJobIDPattern.MatchString(jobID) || !replayNoncePattern.MatchString(nonce) {
		return errors.New("invalid replay key")
	}
	if journalPath == "" || !filepath.IsAbs(journalPath) {
		return errors.New("replay journal path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(journalPath), 0700); err != nil {
		return fmt.Errorf("create replay journal directory: %w", err)
	}
	file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil { return fmt.Errorf("open replay journal: %w", err) }
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock replay journal: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	info, err := file.Stat()
	if err != nil { return fmt.Errorf("stat replay journal: %w", err) }
	if info.Size() > maxReplayJournalBytes {
		return errors.New("replay journal is full; refusing new jobs")
	}
	if _, err := file.Seek(0, 0); err != nil { return fmt.Errorf("rewind replay journal: %w", err) }
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4096)
	for scanner.Scan() {
		var record replayRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return errors.New("replay journal is corrupt; refusing new jobs")
		}
		if record.JobID == jobID || record.Nonce == nonce {
			return errors.New("job ID or nonce has already been consumed")
		}
	}
	if err := scanner.Err(); err != nil { return fmt.Errorf("read replay journal: %w", err) }
	encoded, err := json.Marshal(replayRecord{JobID: jobID, Nonce: nonce})
	if err != nil { return fmt.Errorf("encode replay record: %w", err) }
	if info.Size()+int64(len(encoded)+1) > maxReplayJournalBytes {
		return errors.New("replay journal capacity reached; refusing new jobs")
	}
	if _, err := file.Seek(0, 2); err != nil { return fmt.Errorf("seek replay journal: %w", err) }
	if _, err := file.Write(append(encoded, '\n')); err != nil { return fmt.Errorf("append replay record: %w", err) }
	if err := file.Sync(); err != nil { return fmt.Errorf("sync replay journal: %w", err) }
	return nil
}
