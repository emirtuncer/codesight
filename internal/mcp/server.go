package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/emirtuncer/codesight/internal/search"
	gosync "github.com/emirtuncer/codesight/internal/sync"
	"github.com/emirtuncer/codesight/internal/tasks"
)

// toolContext holds the paths needed by all tool handlers.
type toolContext struct {
	projectDir    string
	codesightDir  string
	projectName   string
}

// NewServer creates an MCP server with the 5 codesight tools registered.
func NewServer(projectDir string) *server.MCPServer {
	codesightDir := filepath.Join(projectDir, ".codesight")
	projectName := filepath.Base(projectDir)

	tc := &toolContext{
		projectDir:   projectDir,
		codesightDir: codesightDir,
		projectName:  projectName,
	}

	s := server.NewMCPServer("codesight", "0.5.0", server.WithToolCapabilities(true))

	// 1. codesight_search
	s.AddTool(mcpgo.NewTool("codesight_search",
		mcpgo.WithDescription("Search symbols, features, and tasks in the codesight index. All parameters are optional and act as filters."),
		mcpgo.WithString("query", mcpgo.Description("Free-text search query")),
		mcpgo.WithString("kind", mcpgo.Description("Symbol kind filter (e.g. function, class, method)")),
		mcpgo.WithString("project", mcpgo.Description("Project name filter")),
		mcpgo.WithString("calls", mcpgo.Description("Find symbols that call this symbol name")),
		mcpgo.WithString("calledby", mcpgo.Description("Find symbols called by this symbol name")),
		mcpgo.WithString("urgency", mcpgo.Description("Urgency filter for tasks (low, medium, high)")),
		mcpgo.WithString("type", mcpgo.Description("Document type filter (symbol, task, feature)")),
	), tc.handleSearch)

	// 2. codesight_status
	s.AddTool(mcpgo.NewTool("codesight_status",
		mcpgo.WithDescription("Show a summary of the codesight index: symbol count, task count, and feature count."),
	), tc.handleStatus)

	// 3. codesight_task_create
	s.AddTool(mcpgo.NewTool("codesight_task_create",
		mcpgo.WithDescription("Create a new task in the codesight index."),
		mcpgo.WithString("title", mcpgo.Required(), mcpgo.Description("Task title")),
		mcpgo.WithString("urgency", mcpgo.Description("Task urgency: low, medium, or high")),
		mcpgo.WithString("description", mcpgo.Description("Optional task description")),
	), tc.handleTaskCreate)

	// 4. codesight_task_update
	s.AddTool(mcpgo.NewTool("codesight_task_update",
		mcpgo.WithDescription("Update an existing task's status, urgency, or assignee."),
		mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Task ID (e.g. T-001)")),
		mcpgo.WithString("status", mcpgo.Description("New status: open, in_progress, done, or cancelled")),
		mcpgo.WithString("urgency", mcpgo.Description("New urgency: low, medium, or high")),
		mcpgo.WithString("assign", mcpgo.Description("Assign to this person")),
	), tc.handleTaskUpdate)

	// 5. codesight_sync
	s.AddTool(mcpgo.NewTool("codesight_sync",
		mcpgo.WithDescription("Sync the codesight index with the current state of the project files."),
	), tc.handleSync)

	return s
}

// Serve creates an MCP server for projectDir and serves it over stdio.
func Serve(projectDir string) error {
	s := NewServer(projectDir)
	return server.ServeStdio(s)
}

// getStringArg safely retrieves a string argument from a tool request.
func getStringArg(req mcpgo.CallToolRequest, key string) string {
	v, ok := req.GetArguments()[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// handleSearch handles the codesight_search tool.
func (tc *toolContext) handleSearch(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	eng := search.New()
	if err := eng.Load(tc.codesightDir); err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("failed to load index: %v", err)), nil
	}

	q := search.Query{
		Text:     getStringArg(req, "query"),
		Kind:     getStringArg(req, "kind"),
		Project:  getStringArg(req, "project"),
		Calls:    getStringArg(req, "calls"),
		CalledBy: getStringArg(req, "calledby"),
		Urgency:  getStringArg(req, "urgency"),
		Type:     getStringArg(req, "type"),
	}

	results := eng.Search(q)

	type resultEntry struct {
		Name          string `json:"name"`
		Type          string `json:"type"`
		Kind          string `json:"kind,omitempty"`
		Project       string `json:"project,omitempty"`
		Status        string `json:"status,omitempty"`
		Urgency       string `json:"urgency,omitempty"`
		QualifiedName string `json:"qualified_name,omitempty"`
		FilePath      string `json:"file_path"`
		Score         float64 `json:"score"`
	}

	entries := make([]resultEntry, 0, len(results))
	for _, r := range results {
		entry := resultEntry{
			FilePath: r.FilePath,
			Score:    r.Score,
		}
		if r.Document != nil {
			entry.Name = r.Document.GetFrontmatterString("name")
			entry.Type = r.Document.Type
			entry.Kind = r.Document.GetFrontmatterString("kind")
			entry.Project = r.Document.GetFrontmatterString("project")
			entry.Status = r.Document.GetFrontmatterString("status")
			entry.Urgency = r.Document.GetFrontmatterString("urgency")
			entry.QualifiedName = r.Document.GetFrontmatterString("qualified_name")
		}
		entries = append(entries, entry)
	}

	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcpgo.NewToolResultText(string(b)), nil
}

// handleStatus handles the codesight_status tool.
func (tc *toolContext) handleStatus(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	eng := search.New()
	if err := eng.Load(tc.codesightDir); err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("failed to load index: %v", err)), nil
	}

	var symbols, taskCount, features int
	for _, doc := range eng.Documents() {
		switch doc.Type {
		case "symbol":
			symbols++
		case "task":
			taskCount++
		case "feature":
			features++
		}
	}

	summary := fmt.Sprintf("codesight index status for project %q:\n  symbols: %d\n  tasks:   %d\n  features: %d\n  total docs: %d",
		tc.projectName, symbols, taskCount, features, len(eng.Documents()))

	return mcpgo.NewToolResultText(summary), nil
}

// handleTaskCreate handles the codesight_task_create tool.
func (tc *toolContext) handleTaskCreate(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	title := getStringArg(req, "title")
	if title == "" {
		return mcpgo.NewToolResultError("title is required"), nil
	}

	opts := tasks.CreateOpts{
		Title:       title,
		Project:     tc.projectName,
		Urgency:     getStringArg(req, "urgency"),
		Description: getStringArg(req, "description"),
	}

	task, err := tasks.Create(tc.codesightDir, opts)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("failed to create task: %v", err)), nil
	}

	msg := fmt.Sprintf("Task created: %s — %s (urgency: %s)", task.ID, task.Title, task.Urgency)
	return mcpgo.NewToolResultText(msg), nil
}

// handleTaskUpdate handles the codesight_task_update tool.
func (tc *toolContext) handleTaskUpdate(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	id := getStringArg(req, "id")
	if id == "" {
		return mcpgo.NewToolResultError("id is required"), nil
	}

	updates := tasks.TaskUpdates{
		Status:     getStringArg(req, "status"),
		Urgency:    getStringArg(req, "urgency"),
		AssignedTo: getStringArg(req, "assign"),
	}

	if err := tasks.Update(tc.codesightDir, tc.projectName, id, updates); err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("failed to update task %s: %v", id, err)), nil
	}

	var parts []string
	if updates.Status != "" {
		parts = append(parts, "status="+updates.Status)
	}
	if updates.Urgency != "" {
		parts = append(parts, "urgency="+updates.Urgency)
	}
	if updates.AssignedTo != "" {
		parts = append(parts, "assigned_to="+updates.AssignedTo)
	}

	msg := fmt.Sprintf("Task %s updated: %s", id, strings.Join(parts, ", "))
	return mcpgo.NewToolResultText(msg), nil
}

// handleSync handles the codesight_sync tool.
func (tc *toolContext) handleSync(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	result, err := gosync.Run(tc.projectDir, tc.codesightDir, tc.projectName, false)
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("sync failed: %v", err)), nil
	}

	msg := fmt.Sprintf("Sync complete — added: %d, modified: %d, removed: %d, errors: %d",
		len(result.Added), len(result.Modified), len(result.Removed), len(result.Errors))
	return mcpgo.NewToolResultText(msg), nil
}
