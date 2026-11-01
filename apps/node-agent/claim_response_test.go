package main

import (
 "encoding/base64"
 "encoding/json"
 "strings"
 "testing"
 "time"
)

func claimFixture(now time.Time) []byte {
 job:=signedJobBody(now,"RESTART",`{}`)
 lease:=base64.RawURLEncoding.EncodeToString(make([]byte,32))
 response:=map[string]any{
  "job":json.RawMessage(job),
  "signedBody":string(job),
  "signature":"sha256="+strings.Repeat("a",64),
  "leaseToken":lease,
  "leaseExpiresAt":now.Add(time.Minute).Format(time.RFC3339Nano),
 }
 raw,err:=json.Marshal(response);if err!=nil{panic(err)}
 return raw
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

func TestDecodeClaimResponseRejectsMalformedLeaseToken(t *testing.T) {
 now:=time.Now().UTC()
 var response map[string]any
 if err:=json.Unmarshal(claimFixture(now),&response);err!=nil{t.Fatal(err)}
 response["leaseToken"]="not-a-valid-lease"
 raw,err:=json.Marshal(response);if err!=nil{t.Fatal(err)}
 if _,_,err:=decodeClaimResponse(raw,now);err==nil{t.Fatal("expected invalid lease token rejection")}
}

func TestDecodeClaimResponseRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
 now:=time.Now().UTC()
 var response map[string]any
 if err:=json.Unmarshal(claimFixture(now),&response);err!=nil{t.Fatal(err)}
 response["unexpected"]="value"
 raw,err:=json.Marshal(response);if err!=nil{t.Fatal(err)}
 if _,_,err:=decodeClaimResponse(raw,now);err==nil{t.Fatal("expected unknown field rejection")}
 if _,_,err:=decodeClaimResponse(append(claimFixture(now),[]byte("{}")...),now);err==nil{t.Fatal("expected trailing JSON rejection")}
}
