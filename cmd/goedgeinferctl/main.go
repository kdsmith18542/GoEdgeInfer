package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	serverURL = flag.String("server", os.Getenv("GOEDGEINFER_SERVER"), "GoEdgeInfer server URL (e.g. http://localhost:8080)")
	apiKey    = flag.String("apikey", os.Getenv("GOEDGEINFER_APIKEY"), "API key for authentication")
)

var doRequest = func(method, path string, body interface{}) ([]byte, error) {
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, *serverURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", *apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s", respBody)
	}
	return respBody, nil
}

func generateJWT(secret, algorithm, issuer, audience, role, scope string, expireMinutes int) (string, error) {
	claims := jwt.MapClaims{
		"iss":   issuer,
		"aud":   audience,
		"role":  role,
		"scope": scope,
		"exp":   time.Now().Add(time.Duration(expireMinutes) * time.Minute).Unix(),
	}
	var token *jwt.Token
	if algorithm == "HS256" {
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		return token.SignedString([]byte(secret))
	}
	return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
}

func validateJWT(tokenStr, secret, algorithm string) error {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if algorithm == "HS256" {
			return []byte(secret), nil
		}
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
	_, err := jwt.Parse(tokenStr, keyFunc)
	return err
}

func batchInfer(inputFile, modelID, version string, outputFormat string) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()
	dec := json.NewDecoder(file)
	var results []map[string]interface{}
	for dec.More() {
		var input map[string]interface{}
		if err := dec.Decode(&input); err != nil {
			return err
		}
		body := map[string]interface{}{
			"model_id": modelID,
			"input":    input,
		}
		if version != "" {
			body["version"] = version
		}
		resp, err := doRequest("POST", "/predict/"+modelID, body)
		if err != nil {
			results = append(results, map[string]interface{}{"error": err.Error()})
		} else {
			var out map[string]interface{}
			json.Unmarshal(resp, &out)
			results = append(results, out)
		}
	}
	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	} else if outputFormat == "table" {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for i, r := range results {
			fmt.Fprintf(w, "#%d\t%v\n", i+1, r)
		}
		w.Flush()
	} else if outputFormat == "quiet" {
		for _, r := range results {
			if err, ok := r["error"]; ok {
				fmt.Println("error:", err)
			} else {
				fmt.Println("ok")
			}
		}
	}
	return nil
}

var exitFunc = os.Exit

func runCLI(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stdout)
	serverURL := fs.String("server", os.Getenv("GOEDGEINFER_SERVER"), "GoEdgeInfer server URL (e.g. http://localhost:8080)")
	apiKey := fs.String("apikey", os.Getenv("GOEDGEINFER_APIKEY"), "API key for authentication")
	if err := fs.Parse(args[1:]); err != nil {
		stdout.Write([]byte(err.Error() + "\n"))
		return 1
	}
	if *serverURL == "" || *apiKey == "" || len(fs.Args()) == 0 {
		stdout.Write([]byte("Usage: goedgeinferctl --server <url> --apikey <key> <command> [args...]\n"))
		return 1
	}
	switch fs.Arg(0) {
	case "list-models":
		resp, err := doRequest("GET", "/models", nil)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "load-model":
		if len(fs.Args()) < 3 {
			fmt.Fprintln(stdout, "Usage: load-model <model_id> <model_path> [version]")
			exitFunc(1)
		}
		body := map[string]interface{}{"model_id": fs.Arg(1), "model_path": fs.Arg(2)}
		if len(fs.Args()) > 3 {
			body["version"] = fs.Arg(3)
		}
		resp, err := doRequest("POST", "/models", body)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "unload-model":
		if len(fs.Args()) < 2 {
			fmt.Fprintln(stdout, "Usage: unload-model <model_id>")
			exitFunc(1)
		}
		resp, err := doRequest("DELETE", "/models/"+fs.Arg(1), nil)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "reload":
		resp, err := doRequest("POST", "/reload", nil)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "list-remote":
		resp, err := doRequest("GET", "/remote_models", nil)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "upload-remote":
		if len(fs.Args()) < 3 {
			fmt.Fprintln(stdout, "Usage: upload-remote <local_path> <object_key>")
			exitFunc(1)
		}
		body := map[string]interface{}{"local_path": fs.Arg(1), "object_key": fs.Arg(2)}
		resp, err := doRequest("POST", "/upload_remote_model", body)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "delete-remote":
		if len(fs.Args()) < 2 {
			fmt.Fprintln(stdout, "Usage: delete-remote <object_key>")
			exitFunc(1)
		}
		body := map[string]interface{}{"object_key": fs.Arg(1)}
		resp, err := doRequest("POST", "/delete_remote_model", body)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "cleanup-cache":
		if len(fs.Args()) < 3 {
			fmt.Fprintln(stdout, "Usage: cleanup-cache <cache_dir> <keep1> [keep2 ...]")
			exitFunc(1)
		}
		body := map[string]interface{}{"cache_dir": fs.Arg(1), "keep": fs.Args()[2:]}
		resp, err := doRequest("POST", "/cleanup_cache", body)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
		fmt.Fprintln(stdout, string(resp))
	case "jwt":
		if len(fs.Args()) < 2 {
			fmt.Fprintln(stdout, "Usage: jwt <generate|validate> [args...]")
			exitFunc(1)
		}
		sub := fs.Arg(1)
		if sub == "generate" {
			if len(fs.Args()) < 8 {
				fmt.Fprintln(stdout, "Usage: jwt generate <secret> <algorithm> <issuer> <audience> <role> <scope> <expire_minutes>")
				exitFunc(1)
			}
			secret, alg, iss, aud, role, scope := fs.Arg(2), fs.Arg(3), fs.Arg(4), fs.Arg(5), fs.Arg(6), fs.Arg(7)
			expire := 60
			if len(fs.Args()) > 8 {
				expire = 0
				fmt.Sscanf(fs.Arg(8), "%d", &expire)
			}
			tok, err := generateJWT(secret, alg, iss, aud, role, scope, expire)
			if err != nil {
				fmt.Fprintln(stdout, "Error:", err)
				exitFunc(1)
			}
			fmt.Fprintln(stdout, tok)
		} else if sub == "validate" {
			if len(fs.Args()) < 5 {
				fmt.Fprintln(stdout, "Usage: jwt validate <token> <secret> <algorithm>")
				exitFunc(1)
			}
			tok, secret, alg := fs.Arg(2), fs.Arg(3), fs.Arg(4)
			err := validateJWT(tok, secret, alg)
			if err != nil {
				fmt.Fprintln(stdout, "Invalid token:", err)
				exitFunc(1)
			}
			fmt.Fprintln(stdout, "Valid token")
		} else {
			fmt.Fprintln(stdout, "Unknown jwt subcommand")
			exitFunc(1)
		}
	case "batch-infer":
		if len(fs.Args()) < 3 {
			fmt.Fprintln(stdout, "Usage: batch-infer <input_jsonl_file> <model_id> [version] [output_format=json|table|quiet]")
			exitFunc(1)
		}
		inputFile, modelID := fs.Arg(1), fs.Arg(2)
		version, outputFormat := "", "json"
		if len(fs.Args()) > 3 {
			version = fs.Arg(3)
		}
		if len(fs.Args()) > 4 {
			outputFormat = fs.Arg(4)
		}
		err := batchInfer(inputFile, modelID, version, outputFormat)
		if err != nil {
			fmt.Fprintln(stdout, "Error:", err)
			exitFunc(1)
		}
	default:
		stdout.Write([]byte("Unknown command\n"))
		return 1
	}
	return 0
}

func main() {
	exitFunc(runCLI(os.Args, os.Stdout))
}
