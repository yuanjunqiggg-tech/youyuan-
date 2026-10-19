package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestObservationAndAccountPoolMethodDocsReadableAllLanguages(t *testing.T) {
	mux := http.NewServeMux()
	yyDocumentationRoutes(mux)
	for _, lang := range []string{"lua-runtime", "python-runtime", "cpp-runtime"} {
		for _, method := range []string{"observerBridge.watchStart", "observerBridge.watchStatus", "observerBridge.watchStop", "accountsPool.add", "accountsPool.start", "accountsPool.invoke", "accountsPool.observationPlan", "accountsPool.handoff"} {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/youyuan/docs?language="+lang+"&method="+method, nil))
			var result struct {
				OK      bool   `json:"ok"`
				Content string `json:"content"`
				Error   string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.OK || !strings.Contains(result.Content, method) {
				t.Fatal(lang, method, result.Error, err)
			}
		}
	}
}
