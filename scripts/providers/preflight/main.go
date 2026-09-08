// APIMart preflight is an operator-only metadata reader. It never submits jobs,
// updates runtime configuration, or treats public prices as verified key prices.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const maxResponseBytes = 2 * 1024 * 1024

var (
	modelPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)
	labelPattern  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.:/\[\]{}-]{0,127}$`)
	numberPattern = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,12})?$`)
)

type options struct {
	Models                        []string
	APIKey, BaseURL, ExpectedPath string
	Now                           time.Time
}

type report struct {
	Version      int           `json:"version"`
	CheckedAt    string        `json:"checked_at"`
	Status       string        `json:"status"`
	B0Complete   bool          `json:"b0_complete"`
	Models       []modelReport `json:"models"`
	Errors       []string      `json:"errors,omitempty"`
	ManualChecks []string      `json:"manual_checks"`
}

type modelReport struct {
	ID        string         `json:"id"`
	Category  string         `json:"category,omitempty"`
	Available bool           `json:"available"`
	Schema    map[string]any `json:"schema,omitempty"`
	Pricing   *priceReport   `json:"pricing,omitempty"`
	Errors    []string       `json:"errors,omitempty"`
	Pending   []string       `json:"pending,omitempty"`
}

type priceReport struct {
	ModelID      string                 `json:"model_id"`
	Unit         string                 `json:"unit"`
	Group        string                 `json:"group"`
	SourceURL    string                 `json:"source_url"`
	CheckedAt    string                 `json:"checked_at"`
	Rates        map[string]json.Number `json:"rates"`
	Verification string                 `json:"verification"`
}

func main() {
	args := flag.NewFlagSet("apimart-preflight", flag.ContinueOnError)
	args.SetOutput(io.Discard) // flag errors can include user-supplied values.
	ids := args.String("models", "", "comma-separated exact model IDs")
	expected := args.String("expected-pricing", "", "local price evidence JSON")
	err := args.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		fmt.Println("APIMart read-only preflight: --models ID[,ID] [--expected-pricing FILE.json]. Credentials: APIMART_API_KEY environment only.")
		return
	}
	if err != nil || len(args.Args()) != 0 {
		fmt.Println(`{"status":"blocked","errors":["invalid_arguments"],"b0_complete":false}`)
		os.Exit(1)
	}
	opt := options{Models: strings.Split(*ids, ","), APIKey: os.Getenv("APIMART_API_KEY"), BaseURL: os.Getenv("APIMART_BASE_URL"), ExpectedPath: *expected, Now: time.Now().UTC()}
	os.Exit(execute(opt, newHTTPClient(nil), os.Stdout))
}

func newHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func execute(opt options, client *http.Client, output io.Writer) int {
	r := report{Version: 1, CheckedAt: opt.Now.UTC().Format(time.RFC3339), Status: "checks_passed", Models: []modelReport{}, ManualChecks: []string{"working_key_group_tariff", "actual_provider_cost_without_generation_not_verified"}}
	finish := func(code int) int {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			data = []byte(`{"status":"blocked","errors":["report_encoding_failed"],"b0_complete":false}`)
			code = 1
		}
		// A malicious metadata server can echo the credential in a plausible field.
		if opt.APIKey != "" {
			escaped, _ := json.Marshal(opt.APIKey)
			if bytes.Contains(data, []byte(opt.APIKey)) || bytes.Contains(data, escaped[1:len(escaped)-1]) {
				data = []byte(`{"status":"blocked","errors":["unsafe_metadata"],"b0_complete":false}`)
				code = 1
			}
		}
		if _, err = fmt.Fprintln(output, string(data)); err != nil {
			return 1
		}
		return code
	}
	block := func(reason string) int { r.Status = "blocked"; r.Errors = []string{reason}; return finish(1) }
	base, err := validateOptions(&opt)
	if err != nil {
		return block(err.Error())
	}
	evidence, err := loadEvidence(opt.ExpectedPath, opt.Now)
	if err != nil {
		return block("invalid_price_evidence")
	}
	list, err := getJSON(client, base+"/models?expand=category", opt.APIKey)
	if err != nil {
		return block("models_" + err.Error())
	}
	rows, ok := list["data"].([]any)
	if !ok {
		return block("invalid_model_list")
	}
	selected := map[string]map[string]any{}
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			return block("invalid_model_list")
		}
		id, ok := m["id"].(string)
		if !ok {
			return block("invalid_model_list")
		}
		for _, wanted := range opt.Models {
			if id == wanted {
				if _, exists := selected[id]; exists {
					return block("duplicate_model_metadata")
				}
				selected[id] = m
			}
		}
	}
	code := 0
	for _, id := range opt.Models {
		m := modelReport{ID: id}
		item, exists := selected[id]
		if !exists {
			m.Errors = append(m.Errors, "model_not_available")
		} else {
			m.Available = true
			category, _ := item["category"].(string)
			switch category {
			case "image", "video", "chat", "audio":
				m.Category = category
			default:
				m.Errors = append(m.Errors, "unsupported_category")
			}
			if m.Category == "image" || m.Category == "video" {
				path := "/models/" + url.PathEscape(id) + "/schema"
				if strings.Contains(id, "/") {
					path = "/model-schema?" + url.Values{"model": {id}}.Encode()
				}
				data, schemaErr := getJSON(client, base+path, opt.APIKey)
				if schemaErr != nil {
					m.Errors = append(m.Errors, "schema_"+schemaErr.Error())
				} else {
					m.Schema, schemaErr = projectSchema(data, id, m.Category)
					if schemaErr != nil {
						m.Errors = append(m.Errors, "invalid_schema")
					} else {
						m.Schema["source_url"] = base + path
						if m.Schema["source"] == "base" {
							m.Pending = append(m.Pending, "model_specific_schema_required")
						}
						for _, field := range []string{"idempotency", "response_contract"} {
							if _, ok := m.Schema[field]; !ok {
								m.Pending = append(m.Pending, field+"_requires_documentation_review")
							}
						}
					}
				}
			} else if m.Category == "chat" || m.Category == "audio" {
				r.ManualChecks = append(r.ManualChecks, id+":chat_audio_parameter_contract_requires_documentation_review")
			}
			if len(m.Errors) == 0 {
				expected, hasEvidence := evidence[id]
				if m.Category == "chat" || (hasEvidence && expected.Unit == "usd_per_million_tokens") {
					pricingURL := "https://api.apimart.ai/api/pricing/model?" + url.Values{"model": {id}}.Encode()
					data, priceErr := getJSON(client, pricingURL, "")
					if priceErr != nil {
						m.Errors = append(m.Errors, "pricing_"+priceErr.Error())
					} else {
						m.Pricing, priceErr = projectTokenPricing(data, id, pricingURL, r.CheckedAt)
						if priceErr != nil {
							m.Errors = append(m.Errors, "invalid_token_pricing")
						} else if hasEvidence && !samePrice(*m.Pricing, expected) {
							m.Errors = append(m.Errors, "price_evidence_mismatch")
						}
					}
				} else if hasEvidence {
					expected.Verification = "local_reference_only_not_provider_verified"
					m.Pricing = &expected
				} else {
					m.Pending = append(m.Pending, "fixed_price_source_required")
				}
			}
		}
		if len(m.Errors) > 0 {
			code = 1
		} else if code == 0 && len(m.Pending) > 0 {
			code = 2
		}
		r.Models = append(r.Models, m)
	}
	if code == 1 {
		r.Status = "blocked"
	} else if code == 2 {
		r.Status = "incomplete"
	}
	return finish(code)
}

func validateOptions(opt *options) (string, error) {
	if len(opt.Models) == 0 || len(opt.Models) > 64 {
		return "", fmt.Errorf("explicit_model_ids_required")
	}
	seen := map[string]bool{}
	for i, id := range opt.Models {
		id = strings.TrimSpace(id)
		if !modelPattern.MatchString(id) || seen[id] {
			return "", fmt.Errorf("invalid_model_selection")
		}
		for _, part := range strings.Split(id, "/") {
			if part == "" || part == "." || part == ".." {
				return "", fmt.Errorf("invalid_model_selection")
			}
		}
		seen[id] = true
		opt.Models[i] = id
	}
	if strings.TrimSpace(opt.APIKey) == "" || strings.ContainsAny(opt.APIKey, "\r\n") {
		return "", fmt.Errorf("apimart_api_key_missing_or_invalid")
	}
	base := strings.TrimRight(strings.TrimSpace(opt.BaseURL), "/")
	if base == "" {
		base = "https://api.apimart.ai/v1"
	}
	// Do not send an operator's key to a configurable proxy or a redirect.
	if base != "https://api.apimart.ai/v1" {
		return "", fmt.Errorf("official_https_base_url_required")
	}
	return base, nil
}

func getJSON(client *http.Client, endpoint, key string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid_request")
	}
	req.Header.Set("Accept", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transport_failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http_%d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("body_read_failed")
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("response_too_large")
	}
	data, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	if success, exists := data["success"]; exists && success != true {
		return nil, fmt.Errorf("unsuccessful_response")
	}
	if code, exists := data["code"]; exists && code != json.Number("200") {
		return nil, fmt.Errorf("unsuccessful_response")
	}
	return data, nil
}

func decodeObject(body []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil || data == nil {
		return nil, fmt.Errorf("invalid_json")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("invalid_json")
	}
	return data, nil
}

func unwrap(data map[string]any) map[string]any {
	if inner, ok := data["data"].(map[string]any); ok {
		return inner
	}
	return data
}

func projectSchema(raw map[string]any, id, category string) (map[string]any, error) {
	data := unwrap(raw)
	boundToModel := false
	for _, field := range []string{"id", "model"} {
		if value, ok := data[field]; ok {
			if value != id {
				return nil, fmt.Errorf("schema_model_mismatch")
			}
			boundToModel = true
		}
	}
	params := data
	if value, ok := data["parameters"].(map[string]any); ok {
		params = value
	}
	operation := category + "_generation"
	endpoint := "/v1/" + category + "s/generations"
	if params["operation"] != operation || params["method"] != "POST" || params["endpoint"] != endpoint {
		return nil, fmt.Errorf("schema_route_mismatch")
	}
	version, ok := params["schema_version"].(string)
	if !ok || !labelPattern.MatchString(version) {
		return nil, fmt.Errorf("schema_version_missing")
	}
	input, ok := params["input_schema"].(map[string]any)
	if !ok || input["type"] != "object" {
		return nil, fmt.Errorf("input_schema_missing")
	}
	properties, ok := input["properties"].(map[string]any)
	if !ok || len(properties) == 0 {
		return nil, fmt.Errorf("schema_properties_missing")
	}
	if modelField, ok := properties["model"].(map[string]any); ok {
		if value, exists := modelField["const"]; exists {
			if value != id {
				return nil, fmt.Errorf("schema_model_mismatch")
			}
			boundToModel = true
		}
		if values, exists := modelField["enum"].([]any); exists {
			found := false
			for _, value := range values {
				if value == id {
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("schema_model_mismatch")
			}
			boundToModel = true
		}
	}
	if !boundToModel {
		return nil, fmt.Errorf("schema_model_binding_missing")
	}
	cleanInput, err := cleanSchema(input, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"schema_version": version, "operation": operation, "method": "POST", "endpoint": endpoint, "input_schema": cleanInput}
	if source, ok := params["source"].(string); ok && labelPattern.MatchString(source) {
		out["source"] = source
	}
	for _, field := range []string{"idempotency", "response_contract"} {
		value := data[field]
		if value == nil {
			value = params[field]
		}
		if value != nil {
			if cleaned := cleanContract(value, 0); cleaned != nil {
				out[field] = cleaned
			}
		}
	}
	return out, nil
}

// Only machine constraints are exported. Descriptions, examples, arbitrary
// defaults and unknown metadata can contain user content or private URLs.
func cleanSchema(input map[string]any, depth int) (map[string]any, error) {
	if depth > 12 {
		return nil, fmt.Errorf("schema_too_deep")
	}
	out := map[string]any{}
	for key, value := range input {
		switch key {
		case "type", "format", "const":
			if text, ok := value.(string); ok {
				if !labelPattern.MatchString(text) {
					return nil, fmt.Errorf("unsafe_schema_value")
				}
				out[key] = text
			} else if key == "const" {
				if _, ok := value.(bool); ok {
					out[key] = value
				} else if n, ok := value.(json.Number); ok && validNumber(n) {
					out[key] = n
				} else {
					return nil, fmt.Errorf("invalid_schema_constant")
				}
			}
		case "required", "enum":
			items, ok := value.([]any)
			if !ok || len(items) > 256 {
				return nil, fmt.Errorf("invalid_schema_list")
			}
			for _, item := range items {
				switch v := item.(type) {
				case string:
					if !labelPattern.MatchString(v) {
						return nil, fmt.Errorf("unsafe_schema_value")
					}
				case json.Number:
					if key == "required" || !validNumber(v) {
						return nil, fmt.Errorf("invalid_schema_number")
					}
				case bool:
					if key == "required" {
						return nil, fmt.Errorf("invalid_schema_list")
					}
				default:
					return nil, fmt.Errorf("invalid_schema_list")
				}
			}
			out[key] = items
		case "minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems":
			n, ok := value.(json.Number)
			if !ok || !validNumber(n) {
				return nil, fmt.Errorf("invalid_schema_number")
			}
			out[key] = n
		case "additionalProperties":
			if b, ok := value.(bool); ok {
				out[key] = b
			}
		case "properties":
			fields, ok := value.(map[string]any)
			if !ok || len(fields) > 256 {
				return nil, fmt.Errorf("invalid_schema_properties")
			}
			cleaned := map[string]any{}
			for name, child := range fields {
				if !labelPattern.MatchString(name) {
					return nil, fmt.Errorf("unsafe_schema_field")
				}
				obj, ok := child.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid_schema_field")
				}
				nested, err := cleanSchema(obj, depth+1)
				if err != nil {
					return nil, err
				}
				switch name {
				case "prompt", "negative_prompt", "text", "lyrics", "description", "content", "instructions", "system":
					delete(nested, "const")
					delete(nested, "enum")
				}
				cleaned[name] = nested
			}
			out[key] = cleaned
		case "items":
			if child, ok := value.(map[string]any); ok {
				nested, err := cleanSchema(child, depth+1)
				if err != nil {
					return nil, err
				}
				out[key] = nested
			}
		case "anyOf", "oneOf", "allOf":
			items, ok := value.([]any)
			if !ok || len(items) > 64 {
				return nil, fmt.Errorf("invalid_schema_conditions")
			}
			cleaned := []any{}
			for _, item := range items {
				child, ok := item.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("invalid_schema_condition")
				}
				nested, err := cleanSchema(child, depth+1)
				if err != nil {
					return nil, err
				}
				cleaned = append(cleaned, nested)
			}
			out[key] = cleaned
		}
	}
	return out, nil
}

func cleanContract(value any, depth int) any {
	if depth > 6 {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			switch key {
			case "supported", "header", "header_name", "ttl_seconds", "scope", "submit", "poll", "http_status", "status_codes", "task_id_path", "endpoint", "method", "in_progress_status", "conflict_status", "result_indeterminate":
				if cleaned := cleanContract(child, depth+1); cleaned != nil {
					out[key] = cleaned
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	case []any:
		if len(v) > 32 {
			return nil
		}
		out := []any{}
		for _, child := range v {
			if cleaned := cleanContract(child, depth+1); cleaned != nil {
				out = append(out, cleaned)
			}
		}
		if len(out) > 0 {
			return out
		}
	case string:
		if labelPattern.MatchString(v) && !strings.Contains(v, "://") {
			return v
		}
	case bool:
		return v
	case json.Number:
		if validNumber(v) {
			return v
		}
	}
	return nil
}

func validNumber(n json.Number) bool { return numberPattern.MatchString(string(n)) }

func projectTokenPricing(raw map[string]any, id, source, date string) (*priceReport, error) {
	data := unwrap(raw)
	if model, ok := data["model"]; ok && model != id {
		return nil, fmt.Errorf("pricing_model_mismatch")
	}
	p, ok := data["pricing"].(map[string]any)
	if !ok || p["unit"] != "usd_per_million_tokens" {
		return nil, fmt.Errorf("pricing_unit_mismatch")
	}
	if tierCount, exists := p["tier_count"]; exists && tierCount != json.Number("1") {
		return nil, fmt.Errorf("tiered_pricing_requires_review")
	}
	group, ok := p["group"].(string)
	if !ok || !labelPattern.MatchString(group) {
		return nil, fmt.Errorf("pricing_group_missing")
	}
	rates, err := readRates(p["effective_rates"], true)
	if err != nil {
		return nil, err
	}
	switch p["pricing_mode"] {
	case "standard":
		if _, ok := rates["input"]; !ok {
			return nil, fmt.Errorf("input_rate_missing")
		}
		if _, ok := rates["output"]; !ok {
			return nil, fmt.Errorf("output_rate_missing")
		}
	case "image_modalities":
		_, textInput := rates["text_input"]
		_, imageInput := rates["image_input"]
		if !textInput && !imageInput {
			return nil, fmt.Errorf("input_rate_missing")
		}
		if _, ok := rates["image_output"]; !ok {
			return nil, fmt.Errorf("output_rate_missing")
		}
	default:
		return nil, fmt.Errorf("pricing_mode_unsupported")
	}
	return &priceReport{ModelID: id, Unit: "usd_per_million_tokens", Group: group, SourceURL: source, CheckedAt: date, Rates: rates, Verification: "public_effective_rates_not_key_group_verified"}, nil
}

func readRates(value any, token bool) (map[string]json.Number, error) {
	obj, ok := value.(map[string]any)
	if !ok || len(obj) == 0 || len(obj) > 64 {
		return nil, fmt.Errorf("rates_missing")
	}
	rates := map[string]json.Number{}
	for key, value := range obj {
		if !labelPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid_rate_name")
		}
		if token {
			switch key {
			case "input", "cached_input", "explicit_cached_input", "cache_write", "cache_write_5m", "cache_write_1h", "output", "output_thinking", "text_input", "cached_text_input", "image_input", "cached_image_input", "text_output", "image_output":
			default:
				return nil, fmt.Errorf("unknown_token_rate")
			}
		}
		n, ok := value.(json.Number)
		if !ok || !validNumber(n) {
			return nil, fmt.Errorf("invalid_rate")
		}
		rates[key] = n
	}
	return rates, nil
}

func loadEvidence(path string, now time.Time) (map[string]priceReport, error) {
	out := map[string]priceReport{}
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path) // #nosec G304 -- Explicit local operator CLI evidence file; not an HTTP input. Size/schema are validated and raw contents are never emitted.
	if err != nil {
		return nil, err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, fmt.Errorf("invalid_evidence")
	}
	data, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	models, ok := data["models"].([]any)
	if !ok || len(models) == 0 || len(models) > 64 {
		return nil, fmt.Errorf("invalid_evidence")
	}
	for _, item := range models {
		p, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid_evidence")
		}
		id, _ := p["model_id"].(string)
		unit, _ := p["unit"].(string)
		group, _ := p["group"].(string)
		source, _ := p["source_url"].(string)
		date, _ := p["checked_at"].(string)
		if !modelPattern.MatchString(id) || !labelPattern.MatchString(group) {
			return nil, fmt.Errorf("invalid_evidence")
		}
		if _, exists := out[id]; exists {
			return nil, fmt.Errorf("duplicate_evidence")
		}
		switch unit {
		case "usd_per_million_tokens", "usd_per_image", "usd_per_second", "usd_per_call":
		default:
			return nil, fmt.Errorf("invalid_evidence_unit")
		}
		checked, err := time.Parse("2006-01-02", date)
		if err != nil || checked.After(now) {
			return nil, fmt.Errorf("invalid_evidence_date")
		}
		u, err := url.Parse(source)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" {
			return nil, fmt.Errorf("invalid_evidence_source")
		}
		allowed := u.Host == "apimart.ai" && (u.Path == "/ru/pricing" || u.Path == "/pricing") && u.RawQuery == ""
		allowed = allowed || (u.Host == "api.apimart.ai" && u.Path == "/api/pricing/model" && u.Query().Get("model") == id && len(u.Query()) == 1)
		if !allowed {
			return nil, fmt.Errorf("invalid_evidence_source")
		}
		rates, err := readRates(p["rates"], unit == "usd_per_million_tokens")
		if err != nil {
			return nil, err
		}
		out[id] = priceReport{ModelID: id, Unit: unit, Group: group, SourceURL: source, CheckedAt: date, Rates: rates}
	}
	return out, nil
}

func samePrice(actual, expected priceReport) bool {
	if actual.Unit != expected.Unit || actual.Group != expected.Group || len(actual.Rates) != len(expected.Rates) {
		return false
	}
	for key := range expected.Rates {
		got, exists := actual.Rates[key]
		if !exists {
			return false
		}
		a, ok := new(big.Rat).SetString(string(got))
		if !ok {
			return false
		}
		b, ok := new(big.Rat).SetString(string(expected.Rates[key]))
		if !ok || a.Cmp(b) != 0 {
			return false
		}
	}
	return true
}
