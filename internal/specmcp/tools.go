package specmcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerTools registers the ask-questions and finish-spec tools with the MCP server.
func (s *Server) registerTools() error {
	// ask-questions: array of question objects
	s.mcpServer.AddTool(
		mcp.NewTool("ask-questions",
			mcp.WithDescription("Ask the user one or more questions and receive their answers"),
			mcp.WithArray("questions", mcp.Required(),
				mcp.Items(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"question": map[string]any{
							"type":        "string",
							"description": "Full question text",
						},
						"header": map[string]any{
							"type":        "string",
							"description": "Short label (max 30 chars)",
						},
						"options": map[string]any{
							"type":        "array",
							"description": "Available answer options",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"label": map[string]any{
										"type":        "string",
										"description": "Display text (1-5 words)",
									},
									"description": map[string]any{
										"type":        "string",
										"description": "Longer description of the option",
									},
								},
								"required": []string{"label", "description"},
							},
						},
						"multiple": map[string]any{
							"type":        "boolean",
							"description": "Allow multi-select (default: false)",
						},
					},
					"required": []string{"question", "header", "options"},
				})),
		),
		s.handleAskQuestions,
	)

	// TODO: Register finish-spec tool in next task

	return nil
}

// handleAskQuestions handles the ask-questions tool call.
// Implementation will be added in a future task.
func (s *Server) handleAskQuestions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// TODO: Implement in TAS-16
	return mcp.NewToolResultError("ask-questions handler not yet implemented"), nil
}
