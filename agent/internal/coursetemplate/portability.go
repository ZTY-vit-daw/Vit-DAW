package coursetemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// jsonUnmarshalStrict decodes with DisallowUnknownFields so manifest schema
// drift fails closed at load time instead of silently dropping pins
// (AGENTS §11 compatibility policy).
func jsonUnmarshalStrict(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

// mediaRefPattern matches Tracktion media-source attributes on clips
// (`source="..."`). VST plugin `filename=` attributes are advisory plugin
// hints resolved by VST3 uniqueId and are deliberately not matched here.
var mediaRefPattern = regexp.MustCompile(`\ssource="[^"]*"`)

// assertNoMediaReferences enforces the portability contract: the template is
// MIDI-only and must carry no audio media path references (absolute or
// relative), so the package can never regress into machine-local paths.
func assertNoMediaReferences(templatePath string) error {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("portability read: %w", err)
	}
	if bytes.Contains(raw, []byte("<AUDIOCLIP")) {
		return fmt.Errorf("portability: template contains AUDIOCLIP elements (media path surface)")
	}
	if loc := mediaRefPattern.Find(raw); loc != nil {
		return fmt.Errorf("portability: template contains a media source reference near %q", string(loc))
	}
	return nil
}
