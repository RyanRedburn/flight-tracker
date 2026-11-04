package mcp

import (
	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func readOnlyTool(name, title, description string, schema *jsonschema.Schema) *mcpsdk.Tool {
	closedWorld := false

	return &mcpsdk.Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Annotations: &mcpsdk.ToolAnnotations{
			Title:         title,
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}
}
