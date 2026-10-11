// VIS-FILM-P0-ROUTE-ENABLE one-shot read-image probe (run-then-delete, design-card probe precedent).
// Executes the production wiring for image understanding exactly as any in-process
// consumer (future B4 read-image iteration) would:
//
//	cfg, _, err := config.Load()      // real ~/.vit/config.json
//	client := &llm.Client{}           // zero-value construction, same as chat/server.go:484
//	client.CompleteImageUnderstanding(ctx, cfg, visionReq)
//
// Not a production surface; deleted after the run.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: visfilmprobe <image.png> <prompt-file> [out-json]")
		os.Exit(2)
	}
	imagePath := os.Args[1]
	promptFile := os.Args[2]
	outPath := "probe_result.json"
	if len(os.Args) > 3 {
		outPath = os.Args[3]
	}

	cfg, cfgPath, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[probe] config load failed: %v\n", err)
		os.Exit(3)
	}
	route := cfg.MultimodalRoutes["image_understanding"]
	fmt.Printf("[probe] config=%s baseUrl=%s model=%s\n", cfgPath, cfg.BaseURL, cfg.DefaultModel)
	fmt.Printf("[probe] route image_understanding: enabled=%v provider=%s baseURL=%q model=%q\n", route.Enabled, route.Provider, route.BaseURL, route.Model)

	promptBytes, err := os.ReadFile(promptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[probe] prompt read failed: %v\n", err)
		os.Exit(3)
	}
	imgBytes, err := os.ReadFile(imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[probe] image read failed: %v\n", err)
		os.Exit(3)
	}
	mime := "image/png"
	switch {
	case strings.HasSuffix(strings.ToLower(imagePath), ".jpg"), strings.HasSuffix(strings.ToLower(imagePath), ".jpeg"):
		mime = "image/jpeg"
	}
	sum := sha256.Sum256(imgBytes)

	visionReq := llm.VisionRequest{
		Prompt: string(promptBytes),
		Images: []llm.ImageInput{{
			MIME:       mime,
			DataBase64: base64.StdEncoding.EncodeToString(imgBytes),
		}},
		Metadata: llm.RequestMetadata{Source: "vis_film_p0_probe"},
	}

	client := &llm.Client{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	start := time.Now()
	resp, err := client.CompleteImageUnderstanding(ctx, cfg, visionReq)
	wallMs := time.Since(start).Milliseconds()

	result := map[string]any{
		"probe":            "vis_film_p0_route_enable",
		"image_path":       filepath.Base(imagePath),
		"image_mime":       mime,
		"image_bytes":      len(imgBytes),
		"image_sha256":     hex.EncodeToString(sum[:]),
		"config_path":      cfgPath,
		"route_enabled":    route.Enabled,
		"route_provider":   route.Provider,
		"endpoint_model":   cfg.DefaultModel,
		"endpoint_baseurl": cfg.BaseURL,
		"wall_ms":          wallMs,
		"raw_kind":         resp.RawKind,
		"usage":            resp.Usage,
		"timings":          resp.Timings,
		"text":             resp.Text,
		"err":              "",
	}

	if err != nil {
		result["err"] = err.Error()
		fmt.Printf("[probe] CALL ERROR after %dms: %v\n", wallMs, err)
	} else {
		fmt.Printf("[probe] OK wall=%dms rawKind=%s usage{in:%d out:%d} timings{build:%dms http:%dms total:%dms}\n",
			wallMs, resp.RawKind, resp.Usage.InputTokens, resp.Usage.OutputTokens,
			resp.Timings.RequestBuildMs, resp.Timings.HTTPMs, resp.Timings.TotalMs)
		fmt.Println("[probe] response text:")
		fmt.Println(resp.Text)
	}

	b, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "[probe] result marshal failed: %v\n", marshalErr)
		os.Exit(3)
	}
	if writeErr := os.WriteFile(outPath, b, 0o644); writeErr != nil {
		fmt.Fprintf(os.Stderr, "[probe] result write failed: %v\n", writeErr)
		os.Exit(3)
	}
	fmt.Printf("[probe] result written to %s\n", outPath)
	if err != nil {
		os.Exit(1)
	}
}
