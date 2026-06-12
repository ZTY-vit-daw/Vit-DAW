package artifacts

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	pdfreader "github.com/ledongthuc/pdf"
)

func ArtifactFromFile(path, id, conversationID, goalID, runID string) Artifact {
	clean := filepath.Clean(strings.TrimSpace(path))
	a := New(time.Now())
	a.ID = strings.TrimSpace(id)
	a.Kind = kindForPath(clean)
	a.Source = "upload"
	a.Path = clean
	a.Title = filepath.Base(clean)
	a.MIME = mimeForPath(clean)
	a.ConversationID = strings.TrimSpace(conversationID)
	a.GoalID = strings.TrimSpace(goalID)
	a.RunID = strings.TrimSpace(runID)
	if st, err := os.Stat(clean); err == nil && !st.IsDir() {
		a.SizeBytes = st.Size()
		a.Metadata["exists"] = true
	} else {
		a.Metadata["exists"] = false
		a.Status = "error"
		a.Summary = "File is not available."
	}
	return a
}

func Extract(a Artifact, maxTextRunes int) Artifact {
	if maxTextRunes <= 0 {
		maxTextRunes = DefaultTextLimit
	}
	a.Status = "ready"
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	switch strings.TrimSpace(a.Kind) {
	case "text":
		return extractText(a, maxTextRunes)
	case "image":
		return extractImage(a)
	case "audio":
		return extractAudio(a)
	case "video":
		return extractMetadataOnly(a, "Video file")
	case "document":
		return extractDocument(a, maxTextRunes)
	case "midi":
		return extractMIDI(a)
	case "web_page":
		if strings.TrimSpace(a.Text) != "" {
			a.Summary = firstLine(a.Text, 240)
		}
		return a
	default:
		return extractMetadataOnly(a, "File")
	}
}

func extractDocument(a Artifact, maxTextRunes int) Artifact {
	ext := strings.ToLower(filepath.Ext(a.Path))
	var (
		text string
		meta map[string]any
		err  error
	)
	switch ext {
	case ".docx":
		text, meta, err = extractDOCXText(a.Path, maxTextRunes)
	case ".pdf":
		text, meta, err = extractPDFText(a.Path, maxTextRunes)
	case ".doc":
		a.Metadata["document_format"] = "doc"
		a.Summary = "Legacy DOC document metadata only."
		return a
	default:
		return extractMetadataOnly(a, "Document")
	}
	if meta != nil {
		for k, v := range meta {
			a.Metadata[k] = v
		}
	}
	if err != nil {
		a.Summary = "Document metadata is available, but text could not be extracted."
		a.Metadata["extract_error"] = err.Error()
		return a
	}
	text = compactRunes(strings.TrimSpace(text), maxTextRunes)
	if strings.TrimSpace(text) == "" {
		a.Summary = "Document metadata is available, but no text was extracted."
		return a
	}
	a.Text = text
	a.Summary = firstLine(text, 240)
	a.Metadata["text_runes"] = len([]rune(text))
	return a
}

func extractDOCXText(path string, maxTextRunes int) (string, map[string]any, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", nil, err
	}
	defer zr.Close()
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", map[string]any{"document_format": "docx"}, errors.New("word/document.xml not found")
	}
	rc, err := doc.Open()
	if err != nil {
		return "", map[string]any{"document_format": "docx"}, err
	}
	defer rc.Close()
	var b strings.Builder
	dec := xml.NewDecoder(io.LimitReader(rc, 12*1024*1024))
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return b.String(), map[string]any{"document_format": "docx"}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteRune('\t')
			case "br":
				b.WriteRune('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteRune('\n')
			}
		case xml.CharData:
			if inText {
				b.Write([]byte(t))
			}
		}
		if maxTextRunes > 0 && len([]rune(b.String())) > maxTextRunes*2 {
			break
		}
	}
	text := normalizeExtractedText(b.String())
	return text, map[string]any{"document_format": "docx"}, nil
}

func extractPDFText(path string, maxTextRunes int) (string, map[string]any, error) {
	text, meta, err := extractPDFTextWithLibrary(path, maxTextRunes)
	if err == nil && usefulDocumentText(text) {
		return text, meta, nil
	}
	fallbackText, fallbackMeta, fallbackErr := extractPDFTextHeuristic(path, maxTextRunes)
	if fallbackErr == nil && usefulDocumentText(fallbackText) {
		fallbackMeta["pdf_text_mode"] = "heuristic_fallback"
		if err != nil {
			fallbackMeta["library_extract_error"] = err.Error()
		}
		return fallbackText, fallbackMeta, nil
	}
	if err != nil {
		return "", meta, err
	}
	if fallbackErr != nil {
		return "", fallbackMeta, fallbackErr
	}
	return "", meta, errors.New("no usable text layer found")
}

func extractPDFTextWithLibrary(path string, maxTextRunes int) (string, map[string]any, error) {
	f, reader, err := pdfreader.Open(path)
	if err != nil {
		return "", map[string]any{"document_format": "pdf", "pdf_text_mode": "library"}, err
	}
	defer f.Close()
	plain, err := reader.GetPlainText()
	if err != nil {
		return "", map[string]any{"document_format": "pdf", "pdf_text_mode": "library", "page_count": reader.NumPage()}, err
	}
	limit := int64(2 * 1024 * 1024)
	if maxTextRunes > 0 {
		limit = int64(maxTextRunes * 8)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(plain, limit)); err != nil {
		return "", map[string]any{"document_format": "pdf", "pdf_text_mode": "library", "page_count": reader.NumPage()}, err
	}
	text := normalizeExtractedText(buf.String())
	return text, map[string]any{
		"document_format": "pdf",
		"pdf_text_mode":   "library",
		"page_count":      reader.NumPage(),
	}, nil
}

func extractPDFTextHeuristic(path string, maxTextRunes int) (string, map[string]any, error) {
	data, err := readBoundedFile(path, 12*1024*1024)
	if err != nil {
		return "", map[string]any{"document_format": "pdf"}, err
	}
	var parts []string
	parts = append(parts, pdfLiteralStrings(data)...)
	for _, stream := range pdfFlateStreams(data) {
		parts = append(parts, pdfLiteralStrings(stream)...)
		if maxTextRunes > 0 && len([]rune(strings.Join(parts, " "))) > maxTextRunes*2 {
			break
		}
	}
	text := normalizeExtractedText(strings.Join(parts, " "))
	meta := map[string]any{
		"document_format": "pdf",
		"pdf_text_mode":   "heuristic",
	}
	if text == "" {
		return "", meta, errors.New("no text layer found")
	}
	return text, meta, nil
}

func pdfFlateStreams(data []byte) [][]byte {
	var out [][]byte
	searchFrom := 0
	for {
		idx := bytes.Index(data[searchFrom:], []byte("stream"))
		if idx < 0 {
			break
		}
		streamStart := searchFrom + idx
		dictStart := streamStart - 512
		if dictStart < 0 {
			dictStart = 0
		}
		dict := data[dictStart:streamStart]
		endRel := bytes.Index(data[streamStart:], []byte("endstream"))
		if endRel < 0 {
			break
		}
		contentStart := streamStart + len("stream")
		if contentStart < len(data) && data[contentStart] == '\r' {
			contentStart++
		}
		if contentStart < len(data) && data[contentStart] == '\n' {
			contentStart++
		}
		contentEnd := streamStart + endRel
		content := bytes.TrimSpace(data[contentStart:contentEnd])
		if bytes.Contains(dict, []byte("/FlateDecode")) {
			if inflated, err := inflateZlib(content, 4*1024*1024); err == nil {
				out = append(out, inflated)
			}
		}
		searchFrom = streamStart + endRel + len("endstream")
	}
	return out
}

func inflateZlib(data []byte, limit int64) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, io.LimitReader(zr, limit))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func pdfLiteralStrings(data []byte) []string {
	var out []string
	for i := 0; i < len(data); i++ {
		if data[i] != '(' {
			continue
		}
		text, next, ok := readPDFLiteral(data, i+1)
		if ok {
			clean := normalizeExtractedText(text)
			if usefulExtractedText(clean) {
				out = append(out, clean)
			}
			i = next
		}
	}
	return out
}

func readPDFLiteral(data []byte, start int) (string, int, bool) {
	var b strings.Builder
	depth := 1
	for i := start; i < len(data); i++ {
		ch := data[i]
		if ch == '\\' {
			if i+1 >= len(data) {
				break
			}
			i++
			switch data[i] {
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			case 'b', 'f':
			default:
				b.WriteByte(data[i])
			}
			continue
		}
		if ch == '(' {
			depth++
		}
		if ch == ')' {
			depth--
			if depth == 0 {
				return b.String(), i, true
			}
		}
		b.WriteByte(ch)
	}
	return "", start, false
}

func normalizeExtractedText(text string) string {
	text = strings.ToValidUTF8(text, "\uFFFD")
	var b strings.Builder
	space := false
	newlines := 0
	for _, r := range text {
		switch {
		case r == '\r' || r == '\n':
			if newlines < 2 {
				b.WriteRune('\n')
			}
			space = false
			newlines++
		case unicode.IsSpace(r):
			if !space {
				b.WriteRune(' ')
			}
			space = true
			newlines = 0
		case unicode.IsPrint(r):
			b.WriteRune(r)
			space = false
			newlines = 0
		}
	}
	return strings.TrimSpace(b.String())
}

func usefulExtractedText(text string) bool {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) < 2 {
		return false
	}
	printable := 0
	letters := 0
	for _, r := range runes {
		if unicode.IsPrint(r) {
			printable++
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			letters++
		}
	}
	return printable >= len(runes)-1 && letters > 0
}

func usefulDocumentText(text string) bool {
	clean := strings.TrimSpace(text)
	if len([]rune(clean)) < 4 {
		return false
	}
	lower := strings.ToLower(clean)
	junkMarkers := []string{
		"begincmap",
		"endcmap",
		"/cidinit",
		"adobe",
		"cmapname",
		"cidrange",
		"cidchar",
	}
	markers := 0
	for _, marker := range junkMarkers {
		if strings.Contains(lower, marker) {
			markers++
		}
	}
	if markers >= 2 {
		return false
	}
	return usefulExtractedText(clean)
}

func extractText(a Artifact, maxTextRunes int) Artifact {
	data, err := readBoundedFile(a.Path, 512*1024)
	if err != nil {
		a.Status = "error"
		a.Summary = err.Error()
		return a
	}
	if !utf8.Valid(data) {
		data = []byte(strings.ToValidUTF8(string(data), "\uFFFD"))
	}
	text := compactRunes(string(data), maxTextRunes)
	a.Text = text
	a.Summary = firstLine(text, 240)
	a.Metadata["text_bytes_read"] = len(data)
	a.Metadata["text_truncated"] = len([]rune(string(data))) > len([]rune(text))
	return a
}

func extractImage(a Artifact) Artifact {
	f, err := os.Open(a.Path)
	if err != nil {
		a.Status = "error"
		a.Summary = err.Error()
		return a
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		a.Summary = "Image metadata is available, but dimensions could not be decoded."
		a.Metadata["decode_error"] = err.Error()
		return a
	}
	a.Summary = fmt.Sprintf("Image %dx%d (%s)", cfg.Width, cfg.Height, format)
	a.Metadata["width"] = cfg.Width
	a.Metadata["height"] = cfg.Height
	a.Metadata["format"] = format
	return a
}

func extractAudio(a Artifact) Artifact {
	if strings.EqualFold(filepath.Ext(a.Path), ".wav") {
		if meta, err := wavMetadata(a.Path); err == nil {
			for k, v := range meta {
				a.Metadata[k] = v
			}
			if dur, ok := meta["duration_seconds"]; ok {
				a.Summary = fmt.Sprintf("Audio file, duration %.2fs", dur)
				return a
			}
		}
	}
	return extractMetadataOnly(a, "Audio file")
}

func extractMIDI(a Artifact) Artifact {
	data, err := readBoundedFile(a.Path, 8*1024*1024)
	if err != nil {
		a.Status = "error"
		a.Summary = err.Error()
		return a
	}
	meta, err := midiMetadata(data)
	if err != nil {
		a.Summary = "MIDI file metadata is available, but events could not be summarized."
		a.Metadata["parse_error"] = err.Error()
		return a
	}
	for k, v := range meta {
		a.Metadata[k] = v
	}
	a.Summary = fmt.Sprintf("MIDI format %v, %v tracks, %v note events", meta["format"], meta["tracks"], meta["note_events"])
	return a
}

func extractMetadataOnly(a Artifact, label string) Artifact {
	if strings.TrimSpace(a.Summary) == "" {
		a.Summary = label + " metadata only."
	}
	return a
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("artifact has no file path")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if int64(len(data)) > limit {
		data = data[:limit]
	}
	return data, nil
}

func kindForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".markdown", ".json", ".csv", ".tsv", ".xml", ".log", ".yaml", ".yml", ".rtf":
		return "text"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	case ".wav", ".mp3", ".flac", ".ogg", ".oga", ".aif", ".aiff", ".m4a", ".wma":
		return "audio"
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".ogv":
		return "video"
	case ".mid", ".midi":
		return "midi"
	case ".pdf", ".doc", ".docx":
		return "document"
	default:
		return "unknown"
	}
}

func mimeForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mt := mimeByExt(ext); mt != "" {
		return mt
	}
	if f, err := os.Open(path); err == nil {
		defer f.Close()
		var head [512]byte
		n, _ := f.Read(head[:])
		return http.DetectContentType(head[:n])
	}
	return "application/octet-stream"
}

func mimeByExt(ext string) string {
	switch ext {
	case ".txt", ".md", ".markdown", ".log":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".xml":
		return "application/xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".mid", ".midi":
		return "audio/midi"
	case ".mp4":
		return "video/mp4"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return ""
	}
}

func compactRunes(text string, max int) string {
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "\n[truncated]"
}

func firstLine(text string, max int) string {
	text = strings.TrimSpace(text)
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	if text == "" {
		return "Text artifact."
	}
	return compactRunes(text, max)
}

func wavMetadata(path string) (map[string]any, error) {
	data, err := readBoundedFile(path, 256*1024)
	if err != nil {
		return nil, err
	}
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("not a RIFF/WAVE file")
	}
	meta := map[string]any{}
	pos := 12
	var sampleRate, byteRate uint32
	var channels, bitsPerSample uint16
	var dataBytes uint32
	for pos+8 <= len(data) {
		id := string(data[pos : pos+4])
		size := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		pos += 8
		if pos+int(size) > len(data) {
			break
		}
		chunk := data[pos : pos+int(size)]
		switch id {
		case "fmt ":
			if len(chunk) >= 16 {
				channels = binary.LittleEndian.Uint16(chunk[2:4])
				sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
				byteRate = binary.LittleEndian.Uint32(chunk[8:12])
				bitsPerSample = binary.LittleEndian.Uint16(chunk[14:16])
			}
		case "data":
			dataBytes = size
		}
		pos += int(size)
		if pos%2 == 1 {
			pos++
		}
	}
	if sampleRate > 0 {
		meta["sample_rate"] = sampleRate
	}
	if channels > 0 {
		meta["channels"] = channels
	}
	if bitsPerSample > 0 {
		meta["bits_per_sample"] = bitsPerSample
	}
	if byteRate > 0 && dataBytes > 0 {
		meta["duration_seconds"] = float64(dataBytes) / float64(byteRate)
	}
	return meta, nil
}

func midiMetadata(data []byte) (map[string]any, error) {
	if len(data) < 14 || string(data[:4]) != "MThd" {
		return nil, errors.New("missing MThd header")
	}
	headerLen := int(binary.BigEndian.Uint32(data[4:8]))
	if headerLen < 6 || len(data) < 8+headerLen {
		return nil, errors.New("invalid MIDI header")
	}
	format := binary.BigEndian.Uint16(data[8:10])
	tracks := binary.BigEndian.Uint16(data[10:12])
	division := binary.BigEndian.Uint16(data[12:14])
	pos := 8 + headerLen
	noteEvents := 0
	tempoEvents := 0
	for t := 0; t < int(tracks) && pos+8 <= len(data); t++ {
		if string(data[pos:pos+4]) != "MTrk" {
			break
		}
		size := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		pos += 8
		if pos+size > len(data) {
			break
		}
		notes, tempos := summarizeMIDITrack(data[pos : pos+size])
		noteEvents += notes
		tempoEvents += tempos
		pos += size
	}
	return map[string]any{
		"format":       int(format),
		"tracks":       int(tracks),
		"division":     int(division),
		"note_events":  noteEvents,
		"tempo_events": tempoEvents,
	}, nil
}

func summarizeMIDITrack(track []byte) (int, int) {
	pos := 0
	runningStatus := byte(0)
	notes := 0
	tempos := 0
	for pos < len(track) {
		if _, next, ok := readVarLen(track, pos); ok {
			pos = next
		} else {
			break
		}
		if pos >= len(track) {
			break
		}
		status := track[pos]
		if status < 0x80 {
			if runningStatus == 0 {
				break
			}
			status = runningStatus
		} else {
			pos++
			runningStatus = status
		}
		switch {
		case status == 0xFF:
			if pos >= len(track) {
				return notes, tempos
			}
			metaType := track[pos]
			pos++
			length, next, ok := readVarLen(track, pos)
			if !ok {
				return notes, tempos
			}
			pos = next + int(length)
			if metaType == 0x51 {
				tempos++
			}
		case status == 0xF0 || status == 0xF7:
			length, next, ok := readVarLen(track, pos)
			if !ok {
				return notes, tempos
			}
			pos = next + int(length)
		default:
			kind := status & 0xF0
			dataLen := 2
			if kind == 0xC0 || kind == 0xD0 {
				dataLen = 1
			}
			if pos+dataLen > len(track) {
				return notes, tempos
			}
			if kind == 0x90 && track[pos+1] > 0 {
				notes++
			}
			pos += dataLen
		}
	}
	return notes, tempos
}

func readVarLen(data []byte, pos int) (uint32, int, bool) {
	var value uint32
	for i := 0; i < 4; i++ {
		if pos >= len(data) {
			return 0, pos, false
		}
		b := data[pos]
		pos++
		value = (value << 7) | uint32(b&0x7F)
		if b&0x80 == 0 {
			return value, pos, true
		}
	}
	return value, pos, true
}
