package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../../internal/adapter/provider/apimart/testdata/contracts/preflight", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func testOptions(t *testing.T) options {
	t.Helper()
	return options{Models: []string{"fixture-image", "fixture-chat"}, APIKey: "fixture" + "-credential-value", BaseURL: "https://api.apimart.ai/v1", ExpectedPath: "../../../internal/adapter/provider/apimart/testdata/contracts/preflight/expected-pricing.json", Now: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)}
}

func runFixture(t *testing.T, opt options, alter func(*http.Request, *http.Response) *http.Response) (report, int, string, int) {
	t.Helper()
	calls := 0
	client := newHTTPClient(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != "api.apimart.ai" || r.Body != nil {
			t.Fatal("unexpected request boundary")
		}
		var res *http.Response
		switch r.URL.Path {
		case "/v1/models":
			if r.URL.RawQuery != "expand=category" {
				t.Fatal("wrong metadata query")
			}
			res = response(200, fixture(t, "models.json"))
		case "/v1/models/fixture-image/schema":
			res = response(200, fixture(t, "image-schema.json"))
		case "/api/pricing/model":
			if r.URL.Query().Get("model") != "fixture-chat" {
				t.Fatal("wrong pricing model")
			}
			res = response(200, fixture(t, "token-pricing.json"))
		default:
			t.Fatal("unexpected API path")
		}
		if r.URL.Path == "/api/pricing/model" {
			if r.Header.Get("Authorization") != "" {
				t.Fatal("credential sent to public pricing endpoint")
			}
		} else if r.Header.Get("Authorization") != "Bearer "+opt.APIKey {
			t.Fatal("metadata authentication missing")
		}
		if alter != nil {
			res = alter(r, res)
		}
		return res, nil
	}))
	var out bytes.Buffer
	code := execute(opt, client, &out)
	var got report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal("output is not a JSON report")
	}
	if strings.Contains(out.String(), opt.APIKey) && opt.APIKey != "" {
		t.Fatal("credential disclosed in output")
	}
	if strings.Contains(out.String(), "PRIVATE_FIXTURE") {
		t.Fatal("raw private metadata disclosed")
	}
	return got, code, out.String(), calls
}

func TestPreflightOnlyReadsSelectedMetadataAndUsesEffectiveRates(t *testing.T) {
	got, code, raw, calls := runFixture(t, testOptions(t), nil)
	if code != 0 || calls != 3 || len(got.Models) != 2 {
		t.Fatalf("result code=%d calls=%d models=%d", code, calls, len(got.Models))
	}
	if !strings.Contains(raw, `1.3714288`) || strings.Contains(raw, `999`) {
		t.Fatal("wrong pricing source or repeated discount")
	}
	if !strings.Contains(raw, `"schema_version": "2026-07-30"`) || !strings.Contains(raw, `"4K"`) {
		t.Fatal("schema contract missing")
	}
	if got.B0Complete || len(got.ManualChecks) == 0 {
		t.Fatal("public metadata cannot verify actual key-group charges")
	}
}

func TestPreflightFailsClosedWithoutLeakingProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
	}{
		{"unauthorized", 401, `{"message":"PRIVATE_FIXTURE_ACCOUNT"}`},
		{"forbidden", 403, `{"message":"PRIVATE_FIXTURE_ACCOUNT"}`},
		{"rate_limit", 429, `{}`},
		{"server_error", 500, `{}`},
		{"malformed", 200, `not-json PRIVATE_FIXTURE_ACCOUNT`},
		{"false_success", 200, `{"success":false,"data":[]}`},
		{"wrong_data_shape", 200, `{"data":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, code, _, calls := runFixture(t, testOptions(t), func(r *http.Request, res *http.Response) *http.Response {
				if r.URL.Path == "/v1/models" {
					return response(tc.code, tc.body)
				}
				return res
			})
			if code != 1 || calls != 1 {
				t.Fatalf("code=%d calls=%d", code, calls)
			}
		})
	}
}

func TestPreflightRejectsMissingIDAndMissingOrMismatchedSchema(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		code int
		body string
	}{
		{"missing_id", "/v1/models", 200, `{"data":[]}`},
		{"duplicate_id", "/v1/models", 200, `{"data":[{"id":"fixture-image","category":"image"},{"id":"fixture-image","category":"image"}]}`},
		{"schema_unavailable", "/v1/models/fixture-image/schema", 404, `{}`},
		{"schema_invalid", "/v1/models/fixture-image/schema", 200, `{"success":true,"data":{}}`},
		{"wrong_model", "/v1/models/fixture-image/schema", 200, strings.ReplaceAll(fixture(t, "image-schema.json"), "fixture-image", "different-model")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opt := testOptions(t)
			opt.Models = []string{"fixture-image"}
			_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
				if r.URL.Path == tc.path {
					return response(tc.code, tc.body)
				}
				return res
			})
			if code != 1 {
				t.Fatalf("code=%d", code)
			}
		})
	}
}

func TestPreflightRejectsUnitRateAndGroupMismatch(t *testing.T) {
	for _, pair := range [][2]string{{"usd_per_million_tokens", "usd_per_token"}, {"1.3714288", "1.3714289"}, {`"group":"default"`, `"group":"different"`}} {
		opt := testOptions(t)
		opt.Models = []string{"fixture-chat"}
		_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
			if r.URL.Path == "/api/pricing/model" {
				return response(200, strings.ReplaceAll(fixture(t, "token-pricing.json"), pair[0], pair[1]))
			}
			return res
		})
		if code != 1 {
			t.Fatalf("mismatched price was accepted: %s", pair[0])
		}
	}
}

func TestPreflightRejectsUnsafeConfigBeforeSendingCredentials(t *testing.T) {
	for _, base := range []string{"http://api.apimart.ai/v1", "https://other.example/v1", "https://api.apimart.ai.attacker.test/v1", "https://api.apimart.ai/v1?key=fixture", "https://user:pass@api.apimart.ai/v1", "https://api.apimart.ai/v1/v1"} {
		opt := testOptions(t)
		opt.BaseURL = base
		_, code, _, calls := runFixture(t, opt, nil)
		if code != 1 || calls != 0 {
			t.Fatal("unsafe config sent a request")
		}
	}
	for _, ids := range [][]string{nil, {"../escape"}, {"fixture-image?leak=yes"}, {"fixture-image", "fixture-image"}} {
		opt := testOptions(t)
		opt.Models = ids
		_, code, _, calls := runFixture(t, opt, nil)
		if code != 1 || calls != 0 {
			t.Fatal("invalid model selection sent a request")
		}
	}
}

func TestPreflightRejectsRedirectsAndOversizedBodies(t *testing.T) {
	for _, kind := range []string{"redirect", "oversized"} {
		_, code, _, calls := runFixture(t, testOptions(t), func(r *http.Request, res *http.Response) *http.Response {
			if kind == "redirect" {
				res = response(302, "")
				res.Header.Set("Location", "https://other.example/private")
				return res
			}
			return response(200, strings.Repeat(" ", maxResponseBytes+1))
		})
		if code != 1 || calls != 1 {
			t.Fatalf("%s accepted or followed", kind)
		}
	}
}

func TestPreflightReportsUnverifiedFixedPricesWithoutInventingRates(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-image"}
	opt.ExpectedPath = ""
	got, code, raw, _ := runFixture(t, opt, nil)
	if code != 2 || got.B0Complete || strings.Contains(raw, `"rates"`) {
		t.Fatal("missing price evidence treated as verified")
	}
}

func TestPreflightRejectsUnboundSchema(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-image"}
	_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if strings.HasSuffix(r.URL.Path, "/schema") {
			body := strings.ReplaceAll(fixture(t, "image-schema.json"), `"model":"fixture-image",`, "")
			body = strings.ReplaceAll(body, `,"const":"fixture-image"`, "")
			return response(200, body)
		}
		return res
	})
	if code != 1 {
		t.Fatal("schema not bound to selected model accepted")
	}

}

func TestPreflightRejectsIncompleteTokenPrices(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-chat"}
	opt.ExpectedPath = ""
	_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if r.URL.Path == "/api/pricing/model" {
			return response(200, strings.ReplaceAll(fixture(t, "token-pricing.json"), `"effective_rates":{"input":1.3714288,"output":4.1142856}`, `"effective_rates":{"cached_input":0}`))
		}
		return res
	})
	if code != 1 {
		t.Fatal("missing input/output rates treated as verified")
	}
}

func TestPreflightKeepsZeroRatesAndMissingOptionalRatesDistinct(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-chat"}
	opt.ExpectedPath = ""
	got, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if r.URL.Path == "/api/pricing/model" {
			return response(200, strings.ReplaceAll(fixture(t, "token-pricing.json"), `"output":4.1142856`, `"output":0`))
		}
		return res
	})
	if code != 0 || got.Models[0].Pricing.Rates["output"] != "0" {
		t.Fatal("explicit zero rate lost")
	}
	if _, ok := got.Models[0].Pricing.Rates["output_thinking"]; ok {
		t.Fatal("invented missing rate")
	}
}

func TestPreflightDoesNotTrustMultitierPricesOrPartialSchema(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-chat"}
	_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if r.URL.Path == "/api/pricing/model" {
			return response(200, strings.ReplaceAll(fixture(t, "token-pricing.json"), `"tier_count":1`, `"tier_count":2`))
		}
		return res
	})
	if code != 1 {
		t.Fatal("multi-tier rates flattened without verification")
	}
	opt = testOptions(t)
	opt.Models = []string{"fixture-image"}
	got, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if strings.HasSuffix(r.URL.Path, "/schema") {
			return response(200, strings.ReplaceAll(fixture(t, "image-schema.json"), `"source":"task_model_registry"`, `"source":"base"`))
		}
		return res
	})
	if code != 2 || len(got.Models[0].Pending) == 0 {
		t.Fatal("base schema declared model-specific")
	}
}

func TestPreflightRedactsCredentialEchoAndTransportError(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-image"}
	_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if strings.HasSuffix(r.URL.Path, "/schema") {
			return response(200, strings.ReplaceAll(fixture(t, "image-schema.json"), `"source":"task_model_registry"`, `"source":"`+opt.APIKey+`"`))
		}
		return res
	})
	if code != 1 {
		t.Fatal("credential echo not blocked")
	}
	var output bytes.Buffer
	client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(opt.APIKey + " PRIVATE_FIXTURE_ACCOUNT")
	}))
	if execute(opt, client, &output) != 1 || strings.Contains(output.String(), opt.APIKey) || strings.Contains(output.String(), "PRIVATE_FIXTURE") {
		t.Fatal("transport error leaked")
	}
}

func TestPreflightUsesQuerySchemaForNamespacedModel(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"vendor/fixture-video"}
	opt.ExpectedPath = ""
	calls := 0
	client := newHTTPClient(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer "+opt.APIKey {
			t.Fatal("invalid metadata request")
		}
		if r.URL.Path == "/v1/models" {
			return response(200, fixture(t, "models.json")), nil
		}
		if r.URL.Path != "/v1/model-schema" || r.URL.Query().Get("model") != "vendor/fixture-video" {
			t.Fatal("namespaced model did not use query endpoint")
		}
		body := strings.ReplaceAll(fixture(t, "image-schema.json"), "fixture-image", "vendor/fixture-video")
		body = strings.ReplaceAll(body, "image_generation", "video_generation")
		body = strings.ReplaceAll(body, "/images/", "/videos/")
		return response(200, body), nil
	}))
	var output bytes.Buffer
	if execute(opt, client, &output) != 2 || calls != 2 {
		t.Fatal("namespaced model metadata failed")
	}
	var got report
	if json.Unmarshal(output.Bytes(), &got) != nil || got.Models[0].Schema == nil {
		t.Fatal("namespaced schema missing")
	}
}

func TestPreflightMalformedEvidenceFailsBeforeHTTP(t *testing.T) {
	for _, body := range []string{`{`, strings.ReplaceAll(fixture(t, "expected-pricing.json"), "usd_per_image", "usd_per_unknown"), strings.ReplaceAll(fixture(t, "expected-pricing.json"), "2026-09-08", "2099-01-01"), strings.ReplaceAll(fixture(t, "expected-pricing.json"), "https://apimart.ai/ru/pricing", "https://apimart.ai/ru/pricing?account=PRIVATE_FIXTURE_ACCOUNT")} {
		path := filepath.Join(t.TempDir(), "expected.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		opt := testOptions(t)
		opt.ExpectedPath = path
		_, code, _, calls := runFixture(t, opt, nil)
		if code != 1 || calls != 0 {
			t.Fatal("invalid price evidence used")
		}
	}
}

func TestPreflightDoesNotExportPromptConstantsOrEnums(t *testing.T) {
	opt := testOptions(t)
	opt.Models = []string{"fixture-image"}
	_, code, _, _ := runFixture(t, opt, func(r *http.Request, res *http.Response) *http.Response {
		if strings.HasSuffix(r.URL.Path, "/schema") {
			return response(200, strings.ReplaceAll(fixture(t, "image-schema.json"), `"prompt":{"type":"string",`, `"prompt":{"type":"string","const":"PRIVATE_FIXTURE_PROMPT","enum":["PRIVATE_FIXTURE_PROMPT"],`))
		}
		return res
	})
	if code != 0 {
		t.Fatal("safe schema projection failed")
	}
}

func TestPreflightMissingCredentialStopsBeforeAnyRequest(t *testing.T) {
	opt := testOptions(t)
	opt.APIKey = ""
	_, code, _, calls := runFixture(t, opt, nil)
	if code != 1 || calls != 0 {
		t.Fatal("missing key was not rejected before network")
	}
}
