package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const replayTestJob = "job_identifier_1234"
const replayTestNonce = "0123456789abcdef0123456789abcdef"
func replayHash(value string) string { sum:=sha256.Sum256([]byte(value)); return hex.EncodeToString(sum[:]) }

func TestReplayJournalPersistsAndAllowsOnlyIdenticalRetry(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"state","replay.jsonl")
	dup,err:=consumeJobReplayKey(path,replayTestJob,replayTestNonce,replayHash("signed-body")); if err!=nil||dup { t.Fatalf("first consume: duplicate=%v err=%v",dup,err) }
	dup,err=consumeJobReplayKey(path,replayTestJob,replayTestNonce,replayHash("signed-body")); if err!=nil||!dup { t.Fatalf("identical retry: duplicate=%v err=%v",dup,err) }
	if _,err=consumeJobReplayKey(path,replayTestJob,"abcdef0123456789abcdef0123456789",replayHash("other"));err==nil { t.Fatal("expected conflicting job ID rejection") }
	if _,err=consumeJobReplayKey(path,"job_identifier_5678",replayTestNonce,replayHash("other"));err==nil { t.Fatal("expected duplicate nonce rejection") }
	if _,err=consumeJobReplayKey(path,"job_identifier_5678","abcdef0123456789abcdef0123456789",replayHash("other"));err!=nil { t.Fatal(err) }
	info,err:=os.Stat(path);if err!=nil { t.Fatal(err) };if info.Mode().Perm()!=0600 { t.Fatalf("permissions=%o",info.Mode().Perm()) }
}
func TestReplayJournalRejectsInvalidKeysAndPaths(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"replay.jsonl")
	for _,tc:=range []struct{path,id,nonce,hash string}{{path,"bad",replayTestNonce,replayHash("x")},{path,replayTestJob,"bad",replayHash("x")},{path,replayTestJob,replayTestNonce,"bad"},{"relative.jsonl",replayTestJob,replayTestNonce,replayHash("x")}} {
		if _,err:=consumeJobReplayKey(tc.path,tc.id,tc.nonce,tc.hash);err==nil { t.Fatalf("expected rejection: %+v",tc) }
	}
}
func TestReplayJournalFailsClosedOnCorruption(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"replay.jsonl");if err:=os.WriteFile(path,[]byte("{broken\n"),0600);err!=nil { t.Fatal(err) }
	_,err:=consumeJobReplayKey(path,replayTestJob,replayTestNonce,replayHash("x"));if err==nil||!strings.Contains(err.Error(),"corrupt") { t.Fatalf("got %v",err) }
}
func TestReplayJournalConcurrentDuplicateIsIdempotent(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"replay.jsonl");var wg sync.WaitGroup
	type result struct{ duplicate bool; err error };results:=make(chan result,12)
	for i:=0;i<12;i++ { wg.Add(1);go func(){defer wg.Done();d,e:=consumeJobReplayKey(path,replayTestJob,replayTestNonce,replayHash("same"));results<-result{d,e}}() }
	wg.Wait();close(results);fresh,duplicates:=0,0
	for r:=range results { if r.err!=nil { t.Fatal(r.err) };if r.duplicate {duplicates++}else{fresh++} }
	if fresh!=1||duplicates!=11 { t.Fatalf("fresh=%d duplicates=%d",fresh,duplicates) }
}
