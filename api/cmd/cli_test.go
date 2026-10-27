package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionPrintsLinkedVersion(t *testing.T) {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	err := Run("1.2.3", []string{"version"})
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil || strings.TrimSpace(string(out)) != "catalystcal 1.2.3" {
		t.Fatalf("version output %q, err %v", out, err)
	}
	if err := Run("x", []string{"bogus"}); err == nil {
		t.Fatal("unknown verb accepted")
	}
}
