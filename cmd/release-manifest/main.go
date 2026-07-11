package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"vk-ai-aggregator/internal/platform/releasemanifest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "assemble":
		return runAssemble(args[1:], stdout, stderr)
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "verify-attestation":
		return runVerifyAttestation(args[1:], stdout, stderr)
	case "verify-trivy":
		return runVerifyTrivy(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "release-manifest: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runVerifyTrivy(args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("verify-trivy", flag.ContinueOnError)
	set.SetOutput(stderr)
	reportPath := set.String("report", "", "Trivy JSON report path")
	imageRef := set.String("image-ref", "", "immutable image reference with sha256 digest")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if set.NArg() != 0 || anyEmpty(*reportPath, *imageRef) {
		fmt.Fprintln(stderr, "release-manifest: verify-trivy requires all named flags and no positional arguments")
		return 2
	}
	raw, err := os.ReadFile(*reportPath)
	if err != nil {
		fmt.Fprintf(stderr, "release-manifest: read Trivy report: %v\n", err)
		return 1
	}
	if err := releasemanifest.VerifyTrivyPolicy(raw, *imageRef); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "Trivy release policy verified")
	return 0
}

func runVerifyAttestation(args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("verify-attestation", flag.ContinueOnError)
	set.SetOutput(stderr)
	verificationOutput := set.String("verification-output", "", "Cosign verify-attestation JSON output path")
	predicatePath := set.String("predicate", "", "expected predicate JSON path")
	imageRef := set.String("image-ref", "", "immutable image reference with sha256 digest")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if set.NArg() != 0 || anyEmpty(*verificationOutput, *predicatePath, *imageRef) {
		fmt.Fprintln(stderr, "release-manifest: verify-attestation requires all named flags and no positional arguments")
		return 2
	}
	verificationRaw, err := os.ReadFile(*verificationOutput)
	if err != nil {
		fmt.Fprintf(stderr, "release-manifest: read verification output: %v\n", err)
		return 1
	}
	predicateRaw, err := os.ReadFile(*predicatePath)
	if err != nil {
		fmt.Fprintf(stderr, "release-manifest: read predicate: %v\n", err)
		return 1
	}
	if err := releasemanifest.VerifyAttestation(verificationRaw, predicateRaw, *imageRef); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "release attestation verified")
	return 0
}

func runAssemble(args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("assemble", flag.ContinueOnError)
	set.SetOutput(stderr)
	inputDir := set.String("input-dir", "", "directory containing service metadata and artifacts")
	output := set.String("output", "", "release manifest output path")
	repository := set.String("repository", "", "lowercase owner/repository")
	commit := set.String("commit", "", "full lowercase commit SHA")
	branch := set.String("branch", "", "source branch")
	workflowIdentity := set.String("workflow-identity", "", "expected GitHub workflow certificate identity")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if set.NArg() != 0 || anyEmpty(*inputDir, *output, *repository, *commit, *branch, *workflowIdentity) {
		fmt.Fprintln(stderr, "release-manifest: assemble requires all named flags and no positional arguments")
		return 2
	}
	manifest, err := releasemanifest.AssembleDirectory(*inputDir, releasemanifest.ManifestHeader{
		Repository:       *repository,
		CommitSHA:        *commit,
		SourceBranch:     *branch,
		WorkflowIdentity: *workflowIdentity,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := releasemanifest.WriteManifestFile(*output, manifest); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "release manifest assembled")
	return 0
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("verify", flag.ContinueOnError)
	set.SetOutput(stderr)
	manifestPath := set.String("manifest", "", "release manifest path")
	bundleDir := set.String("bundle-dir", "", "release bundle directory")
	expectedRepository := set.String("expected-repository", "", "expected lowercase owner/repository")
	expectedCommit := set.String("expected-commit", "", "expected full lowercase commit SHA")
	expectedWorkflowIdentity := set.String("expected-workflow-identity", "", "expected GitHub workflow certificate identity")
	outputEnv := set.String("output-env", "", "verified digest environment output path")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if set.NArg() != 0 || anyEmpty(*manifestPath, *bundleDir, *expectedRepository, *expectedCommit, *expectedWorkflowIdentity, *outputEnv) {
		fmt.Fprintln(stderr, "release-manifest: verify requires all named flags and no positional arguments")
		return 2
	}
	release, err := releasemanifest.VerifyFile(*manifestPath, *bundleDir, releasemanifest.Expectations{
		Repository:       *expectedRepository,
		CommitSHA:        *expectedCommit,
		WorkflowIdentity: *expectedWorkflowIdentity,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := releasemanifest.WriteEnvFile(*outputEnv, release); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "release manifest verified")
	return 0
}

func anyEmpty(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: release-manifest <assemble|verify|verify-attestation|verify-trivy> [flags]")
}
