package com4399

import (
 "context"
 "errors"
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestLoginProvider202IsNotMissingRedirectSuccess(t *testing.T) {
 srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Method==http.MethodGet {w.Header().Set("Content-Type","application/json");w.Write([]byte(`{"result":"https://ptlogin.4399.com/?state=fixture"}`));return}
  w.WriteHeader(http.StatusAccepted);w.Write([]byte("请稍后再试~"))
 }));defer srv.Close()
 c:=NewClient(nil);c.apiBaseURL=srv.URL;c.loginURL=srv.URL
 d:=&Device{DeviceIdentifier:"fixture-device",DeviceIdentifierSM:"fixture-sm",UDID:"fixture-id",client:c}
 _,err:=d.LoginWithPasswordOptions(context.Background(),"fixture-user","not-a-real-password",LoginOptions{ManualCaptcha:true})
 var unavailable *LoginUnavailableError
 if !errors.As(err,&unavailable)||unavailable.StatusCode!=202 {t.Fatalf("expected provider refusal, got %v",err)}
}
