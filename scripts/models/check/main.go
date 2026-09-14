// Command check validates model onboarding offline. It never reads provider
// credentials, calls a provider, or changes model flags or registry files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/providermodels"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errors io.Writer) int {
	flags := flag.NewFlagSet("model-onboarding", flag.ContinueOnError)
	flags.SetOutput(errors)
	path := flags.String("contract", "", "candidate JSON contract to validate for admission")
	bindings := flags.Bool("bindings", false, "print exact current registry bindings as JSON (read-only)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *path != "" && *bindings {
		fmt.Fprintln(errors, "choose registry check, -bindings, or -contract <path>")
		return 2
	}
	r := providermodels.StaticRegistry()
	if *bindings {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(r.Bindings()); err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		return 0
	}
	if *path != "" {
		file, err := os.Open(*path)
		if err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		defer file.Close()
		c, err := modelcontract.Decode(file)
		if err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		fmt.Fprintf(out, "contract: %s\ndigest: %s\n", c.PublicID, c.Digest())
		for _, scenario := range c.OutstandingChecks() {
			fmt.Fprintf(out, "pending: %s\n", scenario)
		}
		if err := c.ValidateReady(); err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		if err := c.ValidateEvidenceFiles(os.DirFS(filepath.Dir(*path))); err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		// Candidate admission also checks provider binding and overlapping limits.
		if r.Contracts == nil {
			r.Contracts = map[string]modelcontract.Contract{}
		}
		r.Contracts[c.PublicID] = c
		if err := r.Validate(); err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		fmt.Fprintln(out, "ready: contract and registry binding validated")
		return 0
	}
	if err := r.Validate(); err != nil {
		fmt.Fprintln(errors, err)
		return 1
	}
	for _, b := range r.Bindings() {
		status := "legacy-unverified"
		if _, exists := r.Contracts[b.PublicID]; exists {
			status = "verified-contract"
		}
		fmt.Fprintf(out, "%s/%s: %s\n", b.Kind, b.PublicID, status)
	}
	return 0
}
