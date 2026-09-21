package sys

import (
	"reflect"
	"testing"
)

func TestParseDNSOutput(t *testing.T) {
	for _, tc := range []struct {
		output string
		want   []string
		err    bool
	}{
		{"1.1.1.1\n2606:4700:4700::1111\n", []string{"1.1.1.1", "2606:4700:4700::1111"}, false},
		{"There aren't any DNS Servers set on Wi-Fi.\n", nil, false},
		{"unexpected output\n", nil, true},
	} {
		got, err := parseDNSOutput(tc.output)
		if (err != nil) != tc.err || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("parseDNSOutput(%q) = %v, %v; want %v, error=%v", tc.output, got, err, tc.want, tc.err)
		}
	}
}

func TestScutilValue(t *testing.T) {
	output := "<dictionary> {\n  PrimaryService : 1234-ABCD\n  UserDefinedName : Wi-Fi Network\n}\n"
	if got := scutilValue(output, "PrimaryService"); got != "1234-ABCD" {
		t.Fatal(got)
	}
	if got := scutilValue(output, "UserDefinedName"); got != "Wi-Fi Network" {
		t.Fatal(got)
	}
}
