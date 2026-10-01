package coreapi

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
)

// Fixed public domains exercise DNS independently of application logs.
// Callers cannot turn diagnostics into arbitrary DNS queries.
var diagnosticDNSDomains = []string{"www.gstatic.com", "example.com"}

type runtimeDNSQuery struct {
	Domain  string `json:"domain"`
	Outcome string `json:"outcome"`
}
type runtimeDNSState struct {
	Outcome string            `json:"outcome"`
	Queries []runtimeDNSQuery `json:"queries"`
}

func probeCoreDNS(ctx context.Context, request func(context.Context, string) (*http.Response, error)) runtimeDNSState {
	ctx, cancel := context.WithTimeout(ctx, controllerTimeout)
	defer cancel()
	result := runtimeDNSState{Outcome: "success", Queries: make([]runtimeDNSQuery, len(diagnosticDNSDomains))}
	var wg sync.WaitGroup
	for i, domain := range diagnosticDNSDomains {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var outcomes [2]string
			var types sync.WaitGroup
			for j, kind := range []string{"A", "AAAA"} {
				types.Add(1)
				go func() {
					defer types.Done()
					response, err := request(ctx, "/dns/query?name="+domain+"&type="+kind)
					outcomes[j] = classifyDNSResponse(response, err, kind)
				}()
			}
			types.Wait()
			outcome := "unavailable"
			for _, value := range outcomes {
				if value == "failed" {
					outcome = "failed"
				}
			}
			if outcomes[0] == "success" || outcomes[1] == "success" {
				outcome = "success"
			}
			result.Queries[i] = runtimeDNSQuery{Domain: domain, Outcome: outcome}
		}()
	}
	wg.Wait()
	for _, query := range result.Queries {
		if query.Outcome == "failed" {
			result.Outcome = "failed"
			break
		}
		if query.Outcome == "unavailable" {
			result.Outcome = "unavailable"
		}
	}
	return result
}

func classifyDNSResponse(response *http.Response, err error, kind string) string {
	if err != nil {
		if isTimeout(err) {
			return "failed"
		}
		return "unavailable"
	}
	if response == nil || response.Body == nil {
		return "unavailable"
	}
	defer response.Body.Close()
	var data struct {
		Status  *int   `json:"Status"`
		Message string `json:"message"`
		Answer  []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&data) != nil {
		return "unavailable"
	}
	if response.StatusCode == http.StatusInternalServerError && data.Message != "" && data.Message != "DNS section is disabled" {
		return "failed"
	}
	if response.StatusCode != http.StatusOK || data.Status == nil {
		return "unavailable"
	}
	if *data.Status != 0 {
		return "failed"
	}
	for _, answer := range data.Answer {
		ip := net.ParseIP(answer.Data)
		if ip != nil && ((kind == "A" && answer.Type == 1 && ip.To4() != nil) || (kind == "AAAA" && answer.Type == 28 && ip.To4() == nil)) {
			return "success"
		}
	}
	return "failed"
}
