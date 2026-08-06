package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"vit-daw-agent/internal/com"
)

func main() {
	artifactPath := flag.String("artifact", "", "path to a dad.compressor_dual_tap_evidence.v1 artifact")
	beforeArtifactPath := flag.String("before-artifact", "", "before artifact for change_delta")
	afterArtifactPath := flag.String("after-artifact", "", "after artifact for change_delta")
	compact := flag.Bool("compact", false, "emit only the sanitized agent/LLM context projection")
	flag.Parse()
	if *artifactPath == "" && (*beforeArtifactPath == "" || *afterArtifactPath == "") {
		fail("provide -artifact or both -before-artifact and -after-artifact")
	}
	projection := com.Projection{}
	if *artifactPath != "" {
		projection = pairedProjection(*artifactPath)
	} else {
		before := pairedProjection(*beforeArtifactPath)
		after := pairedProjection(*afterArtifactPath)
		projection = com.Build(com.Input{Mode: com.ModeChangeDelta,
			Change:    &com.ChangeDeltaInput{Before: &before, After: &after},
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	}
	var output any = projection
	if *compact {
		output = com.ContextProjection(projection)
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fail(fmt.Sprintf("encode projection: %v", err))
	}
	fmt.Println(string(encoded))
	if projection.Status == com.StatusMissing || projection.Status == com.StatusStale || projection.Status == com.StatusSuspect {
		os.Exit(2)
	}
}

func pairedProjection(path string) com.Projection {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail(fmt.Sprintf("read artifact: %v", err))
	}
	artifact, err := com.DecodePairedEvidenceArtifact(raw)
	if err != nil {
		fail(err.Error())
	}
	return com.Build(com.Input{
		Mode: com.ModePairedIO, Paired: &artifact,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
