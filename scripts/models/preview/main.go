// Command preview generates the local-only UI catalog from the backend builder.
// It does not read credentials or make network requests.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"vk-ai-aggregator/internal/service/productcatalog"
)

func main() {
	check := flag.Bool("check", false, "check the generated fixture without writing")
	flag.Parse()
	path := "web/platform/src/features/session/model-catalog.preview.json"
	if flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	catalog, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		fail(err)
	}
	references, err := referencePrices(catalog)
	if err != nil {
		fail(err)
	}
	for output, value := range map[string]any{path: catalog, filepath.Join(filepath.Dir(path), "model-reference-prices.preview.json"): references} {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			fail(err)
		}
		data = append(data, '\n')
		if *check {
			current, err := os.ReadFile(output)
			if err != nil {
				fail(err)
			}
			if !bytes.Equal(bytes.ReplaceAll(current, []byte("\r\n"), []byte("\n")), data) {
				fail(fmt.Errorf("preview %s is stale; run go run ./scripts/models/preview", output))
			}
		} else if err := os.WriteFile(output, data, 0644); err != nil {
			fail(err)
		}
	}
	fmt.Printf("catalog preview: %d models\n", len(catalog.Items))
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
