package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDoRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	oldServerURL := *serverURL
	oldAPIKey := *apiKey
	*serverURL = server.URL
	*apiKey = "test"
	defer func() { *serverURL = oldServerURL; *apiKey = oldAPIKey }()
	resp, err := doRequest("GET", "/test", nil)
	if err != nil || !strings.Contains(string(resp), "ok") {
		t.Errorf("expected ok response, got %v, %s", err, resp)
	}
}

func TestDoRequest_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`error`))
	}))
	defer server.Close()
	oldServerURL := *serverURL
	oldAPIKey := *apiKey
	*serverURL = server.URL
	*apiKey = "test"
	defer func() { *serverURL = oldServerURL; *apiKey = oldAPIKey }()
	_, err := doRequest("GET", "/fail", nil)
	if err == nil {
		t.Error("expected error for 400 response")
	}
}

func TestGenerateAndValidateJWT(t *testing.T) {
	tok, err := generateJWT("secret", "HS256", "iss", "aud", "role", "scope", 1)
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}
	if err := validateJWT(tok, "secret", "HS256"); err != nil {
		t.Errorf("validateJWT failed: %v", err)
	}
}

func TestBatchInfer_JSON(t *testing.T) {
	f, err := ioutil.TempFile("", "batchinfer-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	inputs := []map[string]interface{}{{"a": 1}, {"b": 2}}
	for _, in := range inputs {
		b, _ := json.Marshal(in)
		f.Write(b)
		f.Write([]byte("\n"))
	}
	f.Close()
	oldDoRequest := doRequest
	doRequest = func(method, path string, body interface{}) ([]byte, error) {
		return []byte(`{"result":42}`), nil
	}
	defer func() { doRequest = oldDoRequest }()
	batchInfer(f.Name(), "model", "", "json")
}

func filterTestFlags(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "-test.") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func TestCLI_UsageAndUnknownCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	args := filterTestFlags([]string{"goedgeinferctl"})
	code := runCLI(args, buf)
	if code != 1 || !bytes.Contains(buf.Bytes(), []byte("Usage")) {
		t.Errorf("expected usage message and exit code 1, got %d, %s", code, buf.String())
	}
	buf.Reset()
	args = filterTestFlags([]string{"goedgeinferctl", "--server", "foo", "--apikey", "bar", "badcmd"})
	code = runCLI(args, buf)
	if code != 1 || !bytes.Contains(buf.Bytes(), []byte("Unknown command")) {
		t.Errorf("expected unknown command message and exit code 1, got %d, %s", code, buf.String())
	}
}

func withExitCapture(f func() int) (code int) {
	oldExit := exitFunc
	exitFunc = func(c int) { code = c; panic("exit") }
	defer func() { exitFunc = oldExit; recover() }()
	return f()
}

func TestCLI_ListModels_Error(t *testing.T) {
	buf := &bytes.Buffer{}
	oldDoRequest := doRequest
	doRequest = func(method, path string, body interface{}) ([]byte, error) {
		return nil, fmt.Errorf("api error")
	}
	defer func() { doRequest = oldDoRequest }()
	args := filterTestFlags([]string{"goedgeinferctl", "--server", "foo", "--apikey", "bar", "list-models"})
	code := withExitCapture(func() int { return runCLI(args, buf) })
	if code != 1 && !bytes.Contains(buf.Bytes(), []byte("Error:")) {
		t.Errorf("expected error message for API error, got %d, %s", code, buf.String())
	}
}

func TestCLI_LoadModel_MissingArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	args := filterTestFlags([]string{"goedgeinferctl", "--server", "foo", "--apikey", "bar", "load-model"})
	code := withExitCapture(func() int { return runCLI(args, buf) })
	if code != 1 && !bytes.Contains(buf.Bytes(), []byte("Usage: load-model")) {
		t.Errorf("expected usage message for missing args, got %d, %s", code, buf.String())
	}
}

func TestCLI_JWT_InvalidUsage(t *testing.T) {
	buf := &bytes.Buffer{}
	args := filterTestFlags([]string{"goedgeinferctl", "--server", "foo", "--apikey", "bar", "jwt"})
	code := withExitCapture(func() int { return runCLI(args, buf) })
	if code != 1 && !bytes.Contains(buf.Bytes(), []byte("Usage: jwt")) {
		t.Errorf("expected usage message for jwt, got %d, %s", code, buf.String())
	}
	buf.Reset()
	args = filterTestFlags([]string{"goedgeinferctl", "--server", "foo", "--apikey", "bar", "jwt", "validate"})
	code = withExitCapture(func() int { return runCLI(args, buf) })
	if code != 1 && !bytes.Contains(buf.Bytes(), []byte("Usage: jwt validate")) {
		t.Errorf("expected usage message for jwt validate, got %d, %s", code, buf.String())
	}
}

func TestCLI_BatchInfer_Error(t *testing.T) {
	buf := &bytes.Buffer{}
	args := filterTestFlags([]string{"goedgeinferctl", "--server", "foo", "--apikey", "bar", "batch-infer"})
	code := withExitCapture(func() int { return runCLI(args, buf) })
	if code != 1 && !bytes.Contains(buf.Bytes(), []byte("Usage: batch-infer")) {
		t.Errorf("expected usage message for batch-infer, got %d, %s", code, buf.String())
	}
}
