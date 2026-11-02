package update

import (
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	input := strings.Join([]string{
		"41b240b3f4c33161c4f3f09453e9424d58e33fe0f71c47f3a167e3e16d1f6ee2  dist/evolv_darwin_amd64",
		"089b6cc907a3a61af0cf74e97c5a41812590919d2e84e4a007f6aa20463983bd *evolv_linux_amd64",
		"not-a-hash  evolv_windows_amd64.exe",
		"",
	}, "\n")
	sums, err := parseChecksums(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := sums["evolv_darwin_amd64"]; got != "41b240b3f4c33161c4f3f09453e9424d58e33fe0f71c47f3a167e3e16d1f6ee2" {
		t.Fatalf("darwin sum = %q", got)
	}
	if got := sums["evolv_linux_amd64"]; got != "089b6cc907a3a61af0cf74e97c5a41812590919d2e84e4a007f6aa20463983bd" {
		t.Fatalf("linux sum = %q", got)
	}
	if _, ok := sums["evolv_windows_amd64.exe"]; ok {
		t.Fatal("malformed checksum accepted")
	}
}

func TestParseChecksumsRejectsEmpty(t *testing.T) {
	if _, err := parseChecksums(strings.NewReader("garbage\n")); err == nil {
		t.Fatal("empty checksum table accepted")
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		cur, next string
		want      bool
	}{
		{"0.30.0", "0.30.1", true},
		{"0.30.0", "v0.31.0", true},
		{"0.31.0", "0.30.9", false},
		{"1.0.0", "1.0.0", false},
	}
	for _, tc := range cases {
		if got := Newer(tc.cur, tc.next); got != tc.want {
			t.Errorf("Newer(%q,%q) = %v, want %v", tc.cur, tc.next, got, tc.want)
		}
	}
}

func TestAssetNameHasPlatform(t *testing.T) {
	name := AssetName()
	if !strings.HasPrefix(name, "evolv_") {
		t.Fatalf("AssetName = %q", name)
	}
}
