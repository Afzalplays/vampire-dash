package main

import (
 "bytes"
 "encoding/base64"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "time"
)

const maxClaimResponseBytes = 32 * 1024

type claimResponse struct {
 Job json.RawMessage \`json:"job"\`
 SignedBody string \`json:"signedBody"\`
 Signature string \`json:"signature"\`
 LeaseToken string \`json:"leaseToken"\`
 LeaseExpiresAt time.Time \`json:"leaseExpiresAt"\`
}

// decodeClaimResponse preserves the exact signedBody bytes supplied by the API.
func decodeClaimResponse(body []byte, now time.Time) (claimResponse, []byte, error) {
 var response claimResponse
 if len(body)==0 || len(body)>maxClaimResponseBytes { return response,nil,errors.New("claim response size is invalid") }
 decoder:=json.NewDecoder(bytes.NewReader(body)); decoder.DisallowUnknownFields()
 if err:=decoder.Decode(&response);err!=nil{return response,nil,fmt.Errorf("invalid claim response: %w",err)}
 if err:=decoder.Decode(new(any));err!=io.EOF{return response,nil,errors.New("claim response must contain one JSON object")}
 if len(response.SignedBody)==0 || len(response.SignedBody)>maxJobBodyBytes{return response,nil,errors.New("signed job body size is invalid")}
 if len(response.Signature)!=len("sha256=")+64 || response.Signature[:len("sha256=")]!="sha256=" {return response,nil,errors.New("claim signature format is invalid")}
 signature,err:=hex.DecodeString(response.Signature[len("sha256="):]);if err!=nil||len(signature)!=32{return response,nil,errors.New("claim signature format is invalid")}
 lease,err:=base64.RawURLEncoding.DecodeString(response.LeaseToken);if err!=nil||len(lease)!=32{return response,nil,errors.New("claim lease token is invalid")}
 if response.LeaseExpiresAt.IsZero()||!response.LeaseExpiresAt.After(now)||response.LeaseExpiresAt.After(now.Add(2*time.Minute)){return response,nil,errors.New("claim lease expiry is invalid")}
 var signedJob,displayedJob signedNodeJob
 if err:=json.Unmarshal([]byte(response.SignedBody),&signedJob);err!=nil{return response,nil,errors.New("signed job body is invalid JSON")}
 if err:=json.Unmarshal(response.Job,&displayedJob);err!=nil{return response,nil,errors.New("displayed job is invalid JSON")}
 if signedJob.ID!=displayedJob.ID||signedJob.NodeID!=displayedJob.NodeID||signedJob.Nonce!=displayedJob.Nonce||
 !signedJob.IssuedAt.Equal(displayedJob.IssuedAt)||!signedJob.ExpiresAt.Equal(displayedJob.ExpiresAt)||
 signedJob.Action!=displayedJob.Action||!bytes.Equal(signedJob.Payload,displayedJob.Payload){return response,nil,errors.New("displayed job does not match signed body")}
 if signedJob.ExpiresAt.After(response.LeaseExpiresAt){return response,nil,errors.New("job expiry exceeds its lease")}
 return response,[]byte(response.SignedBody),nil
}
