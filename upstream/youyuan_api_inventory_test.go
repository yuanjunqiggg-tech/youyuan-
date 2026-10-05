package main

import (
 "encoding/json"
 "os"
 "sort"
 "testing"
 lua "github.com/yuin/gopher-lua"
)

// Export the real registered surface for documentation parity checks.
func TestDocumentationInventory(t *testing.T) {
 path := os.Getenv("YY_DOC_INVENTORY")
 if path == "" { t.Skip("inventory export not requested") }
 L := newLuaState(nil); defer L.Close(); defer cancelLuaCtx(L)
 names := map[string][]string{}
 for _, namespace := range yySDKNames {
  if table, ok := L.GetGlobal(namespace).(*lua.LTable); ok {
   table.ForEach(func(k,v lua.LValue){ if v.Type()==lua.LTFunction { names[namespace]=append(names[namespace],k.String()) } })
   sort.Strings(names[namespace])
  }
 }
 data,err := json.MarshalIndent(map[string]any{"namespaces":names,"extra":yyExtraMethods,"resources":yyResourceDocs,"scheduler":yySchedulerDocs,"packets":yyPacketSchema()},"","  ")
 if err != nil { t.Fatal(err) }; if err=os.WriteFile(path,data,0644); err != nil {t.Fatal(err)}
}
