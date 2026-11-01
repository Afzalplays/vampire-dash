package main

import ("testing";"time")

func TestAcceptSignedJobMarksIdenticalRetryAsDuplicate(t *testing.T) {
 now:=time.Now().UTC();body:=signedJobBody(now,"RESTART",`{});path:=t.TempDir()+"/state/replay.jsonl"
 _,duplicate,err:=acceptSignedNodeJob(body,signJob(body),jobTestSecret,"node-a",path,now);if err!=nil||duplicate {t.Fatalf("first accept duplicate=%v err=%v",duplicate,err)}
 _,duplicate,err=acceptSignedNodeJob(body,signJob(body),jobTestSecret,"node-a",path,now);if err!=nil||!duplicate {t.Fatalf("retry duplicate=%v err=%v",duplicate,err)}
 changed:=signedJobBody(now,"STOP",`{});if _,_,err=acceptSignedNodeJob(changed,signJob(changed),jobTestSecret,"node-a",path,now);err==nil {t.Fatal("expected conflicting job ID reuse rejection")}
}
func TestInvalidSignatureDoesNotConsumeReplayKey(t *testing.T) {
 now:=time.Now().UTC();body:=signedJobBody(now,"START",`{});path:=t.TempDir()+"/state/replay.jsonl"
 if _,_,err:=acceptSignedNodeJob(body,"00",jobTestSecret,"node-a",path,now);err==nil {t.Fatal("expected invalid signature")}
 if _,duplicate,err:=acceptSignedNodeJob(body,signJob(body),jobTestSecret,"node-a",path,now);err!=nil||duplicate {t.Fatalf("valid retry duplicate=%v err=%v",duplicate,err)}
}
