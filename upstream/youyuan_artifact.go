package main

import (
 "context"
 "crypto/sha256"
 "fmt"
 "io"
 "net/http"
 "strings"
)

// Compilers exposed through a user-configured MCP can return an artifact URL.
// Download into this workspace, check its hash, then publish through the normal
// ABI-validated native plugin path. The tool never claims source is executable.
func yyFetchArtifact(ctx context.Context,ws *yyWorkspace,path,address,expected string)(string,bool) {
 if !strings.HasPrefix(address,"https://")&&!strings.HasPrefix(address,"http://"){return "编译产物需提供 HTTP(S) 下载地址",true}
 if _,e:=yyProjectPath(yyWorkspaceRoot(ws),path);e!=nil{return e.Error(),true}
 req,e:=http.NewRequestWithContext(ctx,"GET",address,nil);if e!=nil{return e.Error(),true};response,e:=http.DefaultClient.Do(req);if e!=nil{return "下载编译产物失败",true};defer response.Body.Close()
 if response.StatusCode!=200{return fmt.Sprintf("编译产物 HTTP %d",response.StatusCode),true}
 data,e:=io.ReadAll(io.LimitReader(response.Body,(8<<20)+1));if e!=nil||len(data)>8<<20{return "编译产物读取失败或超过 8 MB",true}
 hash:=fmt.Sprintf("%x",sha256.Sum256(data));if expected!=""&&!strings.EqualFold(hash,expected){return "编译产物 SHA-256 不匹配，未修改项目",true}
 rev,e:=yyWriteProject(ws,path,string(data));if e!=nil{return e.Error(),true}
 return mustJSON(map[string]any{"path":path,"bytes":len(data),"sha256":hash,"revision":rev.ID}),false
}
