package main

import (
	"encoding/json"
	"fmt"
	"os"
	"vk-ai-aggregator/internal/service/productcatalog"
)

func main() {
	c, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		fail(err)
	}
	r, err := productcatalog.AuditWorkspaceEconomy(c)
	if err != nil {
		fail(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(r); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
