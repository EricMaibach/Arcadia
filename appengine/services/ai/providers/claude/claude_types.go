package claude

// Claude API structures
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Can be string or array of content blocks
}

type ClaudeTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type ClaudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []ClaudeMessage `json:"messages"`
	Tools     []ClaudeTool    `json:"tools,omitempty"`
	System    string          `json:"system,omitempty"`
}

type ClaudeContent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type ClaudeResponse struct {
	Content    []ClaudeContent `json:"content"`
	StopReason string          `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Claude-specific configuration
type ClaudeConfig struct {
	APIKey             string `json:"api_key"`
	BaseURL            string `json:"base_url"`
	Model              string `json:"model"`
	MaxTokens          int    `json:"max_tokens"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	EnableMCP          bool   `json:"enable_mcp"`
	MCPServerCmd       string `json:"mcp_server_cmd"`
	MaxContextMessages int    `json:"max_context_messages"`
	ContextCompaction  bool   `json:"context_compaction"`
	ContextTTLMinutes  int    `json:"context_ttl_minutes"`
}
