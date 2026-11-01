package main

import (
 "encoding/base64"
 "encoding/json"
 "strings"
 "testing"
 "time"
)
func jsonString(v string) string { b,_:=json.Marshal(v);return string(b) }
func claimFixture(now time.Time) []byte {
 job:=signedJobBody(now,"RESTART",`{}`)
 lease:=base64.RawURLEncoding.EncodeToString(make([]byte,32))
 return []byte(`{"job":`+string(job)+`,"signedBody":`+jsonString(string(job))+`,"signature":"sha256="+strings.Repeat("a",64)+`,"leaseToken":`+jsonString(lease)+`,"leaseExpiresAt":`+jsonString(now.Add(time.Minute).Format(time.RFC3339Nano))+`}`)
}
func TestDecodeClaimResponsePreservesExactSignedBytes(t *testing.T) {
 now:=time.Now().UTC();raw:=claimFixture(now)
 response,signed,err:=decodeClaimResponse(raw,now);if err!=nil{t.Fatal(err)}
 if string(signed)!=response.SignedBody||string(signed)!=string(signedJobBody(now,"RESTART",`{}`)){t.Fatal("signed body bytes changed")}
}
func TestDecodeClaimResponseRejectsMismatchedDisplayedJob(t *testing.T) {
 now:=time.Now().UTC();raw:=strings.Replace(string(claimFixture(now)), "RESTART", "STOP", 1)
 if _,_,err:=decodeClaimResponse([]byte(raw),now);err==nil{t.Fatal("expected mismatch rejection")}
}
func TestDecodeClaimResponseRejectsInvalidLeaseAndExpiry(t *testing.T) {
 now:=time.Now().UTC()
 bad:=strings.Replace(string(claimFixture(now)), "sha256=", "sha512=", 1)
 if _,_,err:=decodeClaimResponse([]byte(bad),now);err==nil{t.Fatal("expected signature format rejection")}
 bad=strings.Replace(string(claimFixture(now)), now.Add(time.Minute).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano),1)
 if _,_,err:=decodeClaimResponse([]byte(bad),now);err==nil{t.Fatal("expected expiry rejection")}
}
