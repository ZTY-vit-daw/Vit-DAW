package shelltools

import "testing"

func TestParseCommandRejectsMetacharacters(t *testing.T) {
	if _, err := parseCommand("go test ./... | tee out.txt"); err == nil {
		t.Fatal("expected metacharacter rejection")
	}
}

func TestAllowlist(t *testing.T) {
	if !isAllowed([]string{"go", "test", "./..."}) {
		t.Fatal("go test should be allowlisted")
	}
	if isAllowed([]string{"powershell", "-Command", "Remove-Item", "x"}) {
		t.Fatal("powershell should not be allowlisted")
	}
}
