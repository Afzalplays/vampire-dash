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
var replayBodyHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type replayRecord struct {
 JobID string `json:"jobId"`
 Nonce string `json:"nonce"`
 BodyHash string `json:"bodyHash"`
}

// consumeJobReplayKey durably reserves a job ID, nonce and exact signed body.
// Identical retries may re-acknowledge but MUST NOT execute the job again.
func consumeJobReplayKey(journalPath, jobID, nonce, bodyHash string) (bool, error) {
 if !replayJobIDPattern.MatchString(jobID) || !replayNoncePattern.MatchString(nonce) || !replayBodyHashPattern.MatchString(bodyHash) {
  return false, errors.New("invalid replay key")
 }
 if journalPath == "" || !filepath.IsAbs(journalPath) { return false, errors.New("replay journal path must be absolute") }
 if err := os.MkdirAll(filepath.Dir(journalPath), 0700); err != nil { return false, fmt.Errorf("create replay journal directory: %w", err) }
 file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_RDWR, 0600)
 if err != nil { return false, fmt.Errorf("open replay journal: %w", err) }
 defer file.Close()
 if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil { return false, fmt.Errorf("lock replay journal: %w", err) }
 defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
 info, err := file.Stat()
 if err != nil { return false, fmt.Errorf("stat replay journal: %w", err) }
 if info.Size() > maxReplayJournalBytes { return false, errors.New("replay journal is full; refusing new jobs") }
 if _, err := file.Seek(0, 0); err != nil { return false, fmt.Errorf("rewind replay journal: %w", err) }
 scanner := bufio.NewScanner(file)
 scanner.Buffer(make([]byte, 4096), 4096)
 for scanner.Scan() {
  var record replayRecord
  if err := json.Unmarshal(scanner.Bytes(), &record); err != nil { return false, errors.New("replay journal is corrupt; refusing new jobs") }
  if record.JobID == jobID || record.Nonce == nonce {
   if record.JobID == jobID && record.Nonce == nonce && record.BodyHash == bodyHash { return true, nil }
   return false, errors.New("job ID or nonce was reused with different content")
  }
 }
 if err := scanner.Err(); err != nil { return false, fmt.Errorf("read replay journal: %w", err) }
 encoded, err := json.Marshal(replayRecord{JobID:jobID, Nonce:nonce, BodyHash:bodyHash})
 if err != nil { return false, fmt.Errorf("encode replay record: %w", err) }
 if info.Size()+int64(len(encoded)+1) > maxReplayJournalBytes { return false, errors.New("replay journal capacity reached; refusing new jobs") }
 if _, err := file.Seek(0, 2); err != nil { return false, fmt.Errorf("seek replay journal: %w", err) }
 if _, err := file.Write(append(encoded, '\n')); err != nil { return false, fmt.Errorf("append replay record: %w", err) }
 if err := file.Sync(); err != nil { return false, fmt.Errorf("sync replay journal: %w", err) }
 return false, nil
}
