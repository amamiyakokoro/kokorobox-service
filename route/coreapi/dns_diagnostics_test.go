package coreapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func dnsResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestDNSResponseClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"ipv4", 200, `{"Status":0,"Answer":[{"type":1,"data":"93.184.215.14"}]}`, "success"},
		{"CNAME alone", 200, `{"Status":0,"Answer":[{"type":5,"data":"example.com."}]}`, "failed"},
		{"empty answer", 200, `{"Status":0}`, "failed"},
		{"NXDOMAIN", 200, `{"Status":3}`, "failed"},
		{"SERVFAIL", 200, `{"Status":2}`, "failed"},
		{"not an IP", 200, `{"Status":0,"Answer":[{"type":1,"data":"private.invalid"}]}`, "failed"},
		{"disabled", 500, `{"message":"DNS section is disabled"}`, "unavailable"},
		{"resolve failed", 500, `{"message":"dns resolve failed: private credentials"}`, "failed"},
		{"unsupported", 404, `{"message":"not found"}`, "unavailable"},
		{"invalid", 200, `{`, "unavailable"},
		{"no status", 200, `{"Answer":[{"type":1,"data":"1.1.1.1"}]}`, "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyDNSResponse(dnsResponse(tc.status, tc.body), nil, "A"); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
	if got := classifyDNSResponse(nil, context.DeadlineExceeded, "A"); got != "failed" {
		t.Fatal(got)
	}
	if got := classifyDNSResponse(nil, errors.New("controller missing"), "A"); got != "unavailable" {
		t.Fatal(got)
	}
}

func TestCoreDNSIPv6OnlyAndDomainFailure(t *testing.T) {
	probe := func(failDomain bool) runtimeDNSState {
		return probeCoreDNS(context.Background(), func(ctx context.Context, path string) (*http.Response, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if !strings.HasPrefix(path, "/dns/query?name=") || (!strings.Contains(path, "www.gstatic.com") && !strings.Contains(path, "example.com")) {
				t.Errorf("unexpected query %s", path)
			}
			if failDomain && strings.Contains(path, "example.com") {
				return dnsResponse(200, `{"Status":2}`), nil
			}
			if strings.HasSuffix(path, "type=AAAA") {
				return dnsResponse(200, `{"Status":0,"Answer":[{"type":28,"data":"2606:4700::1111"}]}`), nil
			}
			return dnsResponse(200, `{"Status":0}`), nil
		})
	}
	if got := probe(false); got.Outcome != "success" || len(got.Queries) != 2 {
		t.Fatalf("%+v", got)
	}
	if got := probe(true); got.Outcome != "failed" || got.Queries[1].Outcome != "failed" {
		t.Fatalf("%+v", got)
	}
}

func TestCoreDNSUnavailableIsNotResolveFailure(t *testing.T) {
	got := probeCoreDNS(context.Background(), func(context.Context, string) (*http.Response, error) { return nil, errors.New("no controller") })
	if got.Outcome != "unavailable" {
		t.Fatalf("%+v", got)
	}
}
