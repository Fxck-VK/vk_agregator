// Command preview generates the local-only UI catalog from the backend builder.
// It does not read credentials or make network requests.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if *check {
		current, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		if !bytes.Equal(bytes.ReplaceAll(current, []byte("\r\n"), []byte("\n")), data) {
			fail(fmt.Errorf("catalog preview is stale; run go run ./scripts/models/preview"))
		}
	} else if err := os.WriteFile(path, data, 0644); err != nil {
		fail(err)
	}
	fmt.Printf("catalog preview: %d models\n", len(catalog.Items))
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
