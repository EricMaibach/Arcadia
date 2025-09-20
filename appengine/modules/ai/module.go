package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"arcadia/modules/ai/core"
	"arcadia/modules/ai/interfaces"
	"arcadia/modules/ai/models"
	"arcadia/modules/ai/providers"
	"arcadia/modules/ai/tools"
)

// Module implements the AIModule interface
type Module struct {
	// Configuration
	config *Config

	// Dependencies
	deps *interfaces.Dependencies

	// Core components
	contextManager interfaces.ContextManager
	toolManager    interfaces.ToolManager

	// Provider management
	providers       map[string]interfaces.AIProvider
	activeProvider  interfaces.AIProvider
	providerMutex   sync.RWMutex

	// State management
	state      *models.ModuleState
	stateMutex sync.RWMutex

	// Lifecycle
	started   bool
	startTime time.Time
	stopChan  chan bool

	// Event handling
	eventListeners []EventListener
	eventMutex     sync.RWMutex

	// Health monitoring
	healthStatus *models.HealthStatus
}

// NewModule creates a new AI module instance
func NewModule(ctx context.Context, config *Config, deps *interfaces.Dependencies) (*Module, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Validate dependencies
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	// Create module
	module := &Module{
		config:         config,
		deps:           deps,
		providers:      make(map[string]interfaces.AIProvider),
		state:          &models.ModuleState{
			Status:        models.ModuleStatusStarting,
			StartTime:     time.Now(),
			LastUpdate:    time.Now(),
			ConfigVersion: "1.0",
			Features:      []string{},
			HealthScore:   1.0,
			Metadata:      make(map[string]interface{}),
		},
		eventListeners: make([]EventListener, 0),
		stopChan:       make(chan bool),
		healthStatus: &models.HealthStatus{
			Status:    models.HealthStatusHealthy,
			Component: "ai_module",
			Message:   "Module initializing",
			Timestamp: time.Now(),
		},
	}

	// Initialize core components
	if err := module.initializeComponents(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	// Initialize providers
	if err := module.initializeProviders(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	// Update state
	module.updateState(models.ModuleStatusRunning)

	if module.deps.Logger != nil {
		module.deps.Logger.Info("AI module created successfully", "provider_count", len(module.providers))
	}

	return module, nil
}

// Version information
func (m *Module) Version() string {
	return Version()
}

func (m *Module) GetInfo() *ModuleInfo {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()

	info := GetModuleInfo()
	info.Status = m.state.Status
	info.StartedAt = m.state.StartTime.Format(time.RFC3339)
	info.Uptime = time.Since(m.state.StartTime).String()
	info.ActiveProvider = m.state.ActiveProvider

	// Add providers list
	m.providerMutex.RLock()
	providerNames := make([]string, 0, len(m.providers))
	for name := range m.providers {
		providerNames = append(providerNames, name)
	}
	m.providerMutex.RUnlock()

	info.Providers = providerNames
	info.Config = m.getConfigInfo()

	return info
}

// Core AI operations
func (m *Module) SendMessage(ctx context.Context, message string) (string, error) {
	return m.SendMessageWithContext(ctx, message, "default")
}

func (m *Module) SendMessageWithContext(ctx context.Context, message string, contextID string) (string, error) {
	// Check if module is running
	if !m.isRunning() {
		return "", models.NewAIError(models.ErrorTypeInternal, "module is not running", "ai_module", 503)
	}

	// Get active provider
	provider := m.getActiveProvider()
	if provider == nil {
		return "", models.NewAIError(models.ErrorTypeProvider, "no active provider configured", "ai_module", 503)
	}

	// Record metrics
	startTime := time.Now()
	if m.deps.Metrics != nil {
		m.deps.Metrics.IncrementCounter("messages_sent", map[string]string{
			"provider":   provider.Name(),
			"context_id": contextID,
		})
	}

	// Send message through provider
	response, err := provider.SendMessageWithContext(ctx, message, contextID)

	// If response is empty and no error, check if we need to execute tools
	if err == nil && response == "" {
		if toolResponse, toolErr := m.handleToolExecution(ctx, contextID, 0); toolErr == nil && toolResponse != "" {
			response = toolResponse
		}
	}

	duration := time.Since(startTime).Milliseconds()
	success := err == nil

	// Record metrics
	if m.deps.Metrics != nil {
		tags := map[string]string{
			"provider":   provider.Name(),
			"context_id": contextID,
			"success":    fmt.Sprintf("%t", success),
		}
		m.deps.Metrics.RecordDuration("message_duration", float64(duration), tags)

		if !success {
			m.deps.Metrics.IncrementCounter("message_errors", tags)
		}
	}

	// Publish events
	if m.deps.EventBus != nil {
		if success {
			event := models.NewEvent(models.EventTypeMessageSent, models.EventSourceModule)
			event.WithData(map[string]interface{}{
				"context_id": contextID,
				"provider":   provider.Name(),
				"duration":   duration,
			})
			m.publishEvent(ctx, event)
		} else {
			event := models.NewEvent(models.EventTypeErrorOccurred, models.EventSourceModule)
			event.WithData(map[string]interface{}{
				"context_id": contextID,
				"provider":   provider.Name(),
				"error":      err.Error(),
				"duration":   duration,
			})
			m.publishEvent(ctx, event)
		}
	}

	// Log the operation
	if m.deps.Logger != nil {
		if success {
			m.deps.Logger.Info("Message sent successfully",
				"context_id", contextID,
				"provider", provider.Name(),
				"duration_ms", duration,
				"message_length", len(message),
				"response_length", len(response))
		} else {
			m.deps.Logger.Error("Message sending failed",
				"context_id", contextID,
				"provider", provider.Name(),
				"duration_ms", duration,
				"error", err)
		}
	}

	return response, err
}

// Context management
func (m *Module) ClearContext(ctx context.Context, contextID string) error {
	if m.contextManager == nil {
		return models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	err := m.contextManager.ClearContext(ctx, contextID)

	// Publish event
	if m.deps.EventBus != nil {
		event := models.NewEvent(models.EventTypeContextDeleted, models.EventSourceModule)
		event.WithData(map[string]interface{}{
			"context_id": contextID,
			"success":    err == nil,
		})
		if err != nil {
			event.AddMetadata("error", err.Error())
		}
		m.publishEvent(ctx, event)
	}

	return err
}

func (m *Module) GetContextStats(ctx context.Context, contextID string) (*models.ContextStats, error) {
	if m.contextManager == nil {
		return nil, models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	return m.contextManager.GetContextStats(ctx, contextID)
}

func (m *Module) ListContexts(ctx context.Context) ([]string, error) {
	if m.contextManager == nil {
		return nil, models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	return m.contextManager.ListContexts(ctx)
}

// Tool management
func (m *Module) RefreshTools(ctx context.Context) error {
	if m.toolManager == nil {
		return models.NewAIError(models.ErrorTypeInternal, "tool manager not available", "ai_module", 503)
	}

	err := m.toolManager.RefreshTools(ctx)

	// Update provider tools
	if err == nil {
		if tools, toolErr := m.toolManager.ListTools(ctx); toolErr == nil {
			for _, provider := range m.providers {
				provider.SetTools(ctx, tools)
			}
		}
	}

	return err
}

func (m *Module) GetTools(ctx context.Context) ([]models.Tool, error) {
	if m.toolManager == nil {
		return nil, models.NewAIError(models.ErrorTypeInternal, "tool manager not available", "ai_module", 503)
	}

	return m.toolManager.ListTools(ctx)
}

func (m *Module) ExecuteTool(ctx context.Context, toolName string, input any) (string, error) {
	if m.toolManager == nil {
		return "", models.NewAIError(models.ErrorTypeInternal, "tool manager not available", "ai_module", 503)
	}

	return m.toolManager.ExecuteTool(ctx, toolName, input)
}

// Provider management
func (m *Module) GetProviderInfo(ctx context.Context) (*models.ProviderInfo, error) {
	provider := m.getActiveProvider()
	if provider == nil {
		return nil, models.NewAIError(models.ErrorTypeProvider, "no active provider", "ai_module", 503)
	}

	return provider.GetProviderInfo(ctx)
}

func (m *Module) SwitchProvider(ctx context.Context, providerName string) error {
	m.providerMutex.Lock()
	defer m.providerMutex.Unlock()

	provider, exists := m.providers[providerName]
	if !exists {
		return models.NewAIError(models.ErrorTypeProvider, fmt.Sprintf("provider not found: %s", providerName), "ai_module", 404)
	}

	oldProvider := ""
	if m.activeProvider != nil {
		oldProvider = m.activeProvider.Name()
	}

	m.activeProvider = provider
	m.state.ActiveProvider = providerName

	// Publish event
	if m.deps.EventBus != nil {
		event := models.NewEvent(models.EventTypeProviderSwitched, models.EventSourceModule)
		event.WithData(map[string]interface{}{
			"old_provider": oldProvider,
			"new_provider": providerName,
		})
		m.publishEvent(ctx, event)
	}

	if m.deps.Logger != nil {
		m.deps.Logger.Info("Provider switched", "old_provider", oldProvider, "new_provider", providerName)
	}

	return nil
}

func (m *Module) ListProviders(ctx context.Context) ([]string, error) {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()

	providers := make([]string, 0, len(m.providers))
	for name := range m.providers {
		providers = append(providers, name)
	}

	return providers, nil
}

// Conversation management (placeholder implementations)
func (m *Module) CreateConversation(ctx context.Context, contextID string) error {
	if m.contextManager == nil {
		return models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	return m.contextManager.CreateContext(ctx, contextID)
}

func (m *Module) GetConversation(ctx context.Context, contextID string) (*models.ConversationContext, error) {
	if m.contextManager == nil {
		return nil, models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	return m.contextManager.GetContext(ctx, contextID)
}

func (m *Module) UpdateConversation(ctx context.Context, contextID string, message models.Message) error {
	if m.contextManager == nil {
		return models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	return m.contextManager.AddMessage(ctx, contextID, message)
}

func (m *Module) DeleteConversation(ctx context.Context, contextID string) error {
	if m.contextManager == nil {
		return models.NewAIError(models.ErrorTypeInternal, "context manager not available", "ai_module", 503)
	}

	return m.contextManager.DeleteContext(ctx, contextID)
}

// Health and diagnostics
func (m *Module) HealthCheck(ctx context.Context) (*models.HealthStatus, error) {
	status := &models.HealthStatus{
		Status:     models.HealthStatusHealthy,
		Component:  "ai_module",
		Message:    "Module is operational",
		Timestamp:  time.Now(),
		Duration:   0,
		Checks:     make(map[string]interface{}),
		Dependencies: make(map[string]*models.HealthStatus),
		Metadata:   make(map[string]interface{}),
		Uptime:     time.Since(m.startTime).Seconds(),
	}

	// Check module state
	if !m.isRunning() {
		status.Status = models.HealthStatusUnhealthy
		status.Message = "Module is not running"
		return status, nil
	}

	// Check active provider
	if provider := m.getActiveProvider(); provider != nil {
		providerHealth, err := provider.HealthCheck(ctx)
		if err != nil {
			status.Dependencies[provider.Name()] = &models.HealthStatus{
				Status:  models.HealthStatusUnhealthy,
				Message: fmt.Sprintf("Provider health check failed: %v", err),
			}
		} else {
			status.Dependencies[provider.Name()] = providerHealth
		}

		if providerHealth != nil && !providerHealth.IsHealthy() {
			status.Status = models.HealthStatusWarning
			status.Message = "Provider health issues detected"
		}
	} else {
		status.Status = models.HealthStatusUnhealthy
		status.Message = "No active provider configured"
	}

	// Check core components
	if m.contextManager != nil {
		status.Checks["context_manager"] = "available"
	} else {
		status.Checks["context_manager"] = "unavailable"
		status.Status = models.HealthStatusWarning
	}

	if m.toolManager != nil {
		status.Checks["tool_manager"] = "available"
	} else {
		status.Checks["tool_manager"] = "unavailable"
		status.Status = models.HealthStatusWarning
	}

	// Add metadata
	status.Metadata["provider_count"] = len(m.providers)
	status.Metadata["active_provider"] = m.state.ActiveProvider
	status.Metadata["features"] = m.state.Features

	m.healthStatus = status
	return status, nil
}

func (m *Module) GetMetrics(ctx context.Context) (*models.ModuleMetrics, error) {
	metrics := &models.ModuleMetrics{
		ModuleName:        ModuleName,
		Version:           Version(),
		Timestamp:         time.Now(),
		Uptime:            time.Since(m.startTime).Seconds(),
		ActiveContexts:    0,
		ProviderMetrics:   make(map[string]*models.ProviderMetrics),
		ToolMetrics:       make(map[string]*models.ToolMetrics),
		CustomMetrics:     make(map[string]interface{}),
	}

	// Get context count
	if m.contextManager != nil {
		if count, err := m.contextManager.GetActiveContextCount(ctx); err == nil {
			metrics.ActiveContexts = count
		}
	}

	// Get provider metrics
	for name, provider := range m.providers {
		if providerMetrics, err := provider.GetMetrics(ctx); err == nil {
			metrics.ProviderMetrics[name] = providerMetrics
		}
	}

	// Get tool metrics
	if m.toolManager != nil {
		if tools, err := m.toolManager.ListTools(ctx); err == nil {
			for _, tool := range tools {
				if toolMetrics, err := m.toolManager.GetToolMetrics(ctx, tool.Name); err == nil {
					metrics.ToolMetrics[tool.Name] = toolMetrics
				}
			}
		}
	}

	// Add custom metrics
	metrics.CustomMetrics["module_status"] = m.state.Status
	metrics.CustomMetrics["health_score"] = m.state.HealthScore

	return metrics, nil
}

func (m *Module) ValidateConfiguration(ctx context.Context) error {
	return m.config.Validate()
}

// Lifecycle management
func (m *Module) Start(ctx context.Context) error {
	if m.started {
		return nil // Already started
	}

	// Start core components
	if m.contextManager != nil {
		// Context manager doesn't need explicit start
	}

	if m.toolManager != nil {
		if err := m.toolManager.RefreshTools(ctx); err != nil {
			if m.deps.Logger != nil {
				m.deps.Logger.Warn("Failed to refresh tools during start", "error", err)
			}
		}
	}

	// Start providers
	for name, provider := range m.providers {
		if err := provider.Start(ctx); err != nil {
			if m.deps.Logger != nil {
				m.deps.Logger.Error("Failed to start provider", "provider", name, "error", err)
			}
		}
	}

	m.started = true
	m.startTime = time.Now()
	m.updateState(models.ModuleStatusRunning)

	// Publish event
	if m.deps.EventBus != nil {
		event := models.NewEvent(models.EventTypeModuleStarted, models.EventSourceModule)
		m.publishEvent(ctx, event)
	}

	if m.deps.Logger != nil {
		m.deps.Logger.Info("AI module started successfully")
	}

	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if !m.started {
		return nil // Already stopped
	}

	m.updateState(models.ModuleStatusStopping)

	// Stop providers
	for name, provider := range m.providers {
		if err := provider.Stop(ctx); err != nil {
			if m.deps.Logger != nil {
				m.deps.Logger.Error("Failed to stop provider", "provider", name, "error", err)
			}
		}
	}

	// Stop context manager cleanup
	if cm, ok := m.contextManager.(*core.ContextManager); ok {
		cm.Stop()
	}

	m.started = false
	m.updateState(models.ModuleStatusStopped)

	// Publish event
	if m.deps.EventBus != nil {
		event := models.NewEvent(models.EventTypeModuleStopped, models.EventSourceModule)
		m.publishEvent(ctx, event)
	}

	if m.deps.Logger != nil {
		m.deps.Logger.Info("AI module stopped")
	}

	return nil
}

func (m *Module) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	return m.Start(ctx)
}

// Configuration management
func (m *Module) UpdateConfiguration(ctx context.Context, updates map[string]interface{}) error {
	// This would implement configuration updates
	// For now, return not implemented
	return fmt.Errorf("configuration updates not implemented")
}

func (m *Module) GetConfiguration(ctx context.Context) (map[string]interface{}, error) {
	return m.getConfigInfo(), nil
}

// Internal helper methods

func (m *Module) handleToolExecution(ctx context.Context, contextID string, depth int) (string, error) {
	// Prevent infinite recursion
	const maxDepth = 3
	if depth >= maxDepth {
		return "", fmt.Errorf("maximum tool execution depth reached")
	}

	// Get the conversation context to check for tool calls
	if m.contextManager == nil {
		return "", fmt.Errorf("context manager not available")
	}

	conversationContext, err := m.contextManager.GetContext(ctx, contextID)
	if err != nil {
		return "", fmt.Errorf("failed to get context: %w", err)
	}

	// Find the most recent assistant message with tool calls
	var assistantMessage *models.Message
	for i := len(conversationContext.Messages) - 1; i >= 0; i-- {
		msg := &conversationContext.Messages[i]
		if msg.Role == models.RoleAssistant && len(msg.ToolCalls) > 0 {
			assistantMessage = msg
			break
		}
	}

	if assistantMessage == nil {
		return "", nil // No tool calls to execute
	}

	// Check if all tool calls have results already
	allExecuted := true
	for _, toolCall := range assistantMessage.ToolCalls {
		if toolCall.Result == "" && toolCall.Error == "" {
			allExecuted = false
			break
		}
	}

	if allExecuted {
		return "", nil // All tools already executed
	}

	// Execute tools and collect results
	if m.toolManager == nil {
		return "", fmt.Errorf("tool manager not available")
	}

	for i, toolCall := range assistantMessage.ToolCalls {
		if toolCall.Result == "" && toolCall.Error == "" {
			if m.deps.Logger != nil {
				m.deps.Logger.Info("Executing tool", "tool", toolCall.Name, "context_id", contextID)
			}

			startTime := time.Now()
			result, err := m.toolManager.ExecuteTool(ctx, toolCall.Name, toolCall.Input)
			duration := time.Since(startTime).Milliseconds()

			// Update the tool call with results
			if err != nil {
				assistantMessage.ToolCalls[i].Error = err.Error()
			} else {
				assistantMessage.ToolCalls[i].Result = result
			}
			assistantMessage.ToolCalls[i].Duration = float64(duration)

			// Create a tool message for the conversation context
			toolMessage := models.Message{
				Role:       models.RoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
				Timestamp:  time.Now(),
			}

			if err != nil {
				toolMessage.Content = fmt.Sprintf("Error: %s", err.Error())
			}

			// Add tool result to context
			if err := m.contextManager.AddMessage(ctx, contextID, toolMessage); err != nil {
				if m.deps.Logger != nil {
					m.deps.Logger.Error("Failed to add tool result to context", "error", err)
				}
			}
		}
	}

	// Get active provider and make recursive call to get final response
	provider := m.getActiveProvider()
	if provider == nil {
		return "", fmt.Errorf("no active provider configured")
	}

	// Make recursive call to provider to process tool results
	response, err := provider.SendMessageWithContext(ctx, "", contextID)
	if err != nil {
		return "", fmt.Errorf("failed to get response after tool execution: %w", err)
	}

	// Check if we need another round of tool execution
	if response == "" {
		return m.handleToolExecution(ctx, contextID, depth+1)
	}

	return response, nil
}

func (m *Module) initializeComponents(ctx context.Context) error {
	// Initialize context manager
	if m.config.EnablePersistence || m.config.ContextTTL > 0 {
		contextManager := core.NewContextManager(m.config.ToModelsConfig(), m.deps)
		m.contextManager = contextManager
		if m.deps.Logger != nil {
			m.deps.Logger.Info("Context manager initialized", "persistence", m.config.EnablePersistence, "ttl", m.config.ContextTTL)
		}
	}

	// Initialize tool manager
	if m.config.EnableMCP {
		toolManager := tools.NewManager(m.config.ToModelsConfig(), m.deps)
		m.toolManager = toolManager
		if m.deps.Logger != nil {
			m.deps.Logger.Info("Tool manager initialized", "mcp_enabled", m.config.EnableMCP)
		}
	}

	return nil
}

func (m *Module) initializeProviders(ctx context.Context) error {
	// Initialize OpenAI provider if configured
	if m.config.Provider == "openai" || (m.config.Providers != nil && m.config.Providers["openai"] != nil) {
		var providerConfig map[string]interface{}
		if m.config.Providers != nil && m.config.Providers["openai"] != nil {
			if pc, ok := m.config.Providers["openai"].(map[string]interface{}); ok {
				providerConfig = pc
			}
		}

		if providerConfig == nil {
			providerConfig = make(map[string]interface{})
		}

		provider, err := providers.NewOpenAIProvider(ctx, providerConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize OpenAI provider: %w", err)
		}

		// Set dependencies
		provider.SetLogger(m.deps.Logger)
		provider.SetMetrics(m.deps.Metrics)
		provider.SetContextManager(m.contextManager)

		// Set tools if available
		if m.toolManager != nil {
			if tools, err := m.toolManager.ListTools(ctx); err == nil {
				provider.SetTools(ctx, tools)
			}
		}

		m.providers["openai"] = provider

		// Set as active provider if it's the configured provider
		if m.config.Provider == "openai" {
			m.activeProvider = provider
			m.state.ActiveProvider = "openai"
		}

		if m.deps.Logger != nil {
			m.deps.Logger.Info("OpenAI provider initialized")
		}
	}

	// Ensure we have an active provider
	if m.activeProvider == nil && len(m.providers) > 0 {
		// Use the first available provider
		for name, provider := range m.providers {
			m.activeProvider = provider
			m.state.ActiveProvider = name
			break
		}
	}

	return nil
}

func (m *Module) updateState(status string) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()

	m.state.Status = status
	m.state.LastUpdate = time.Now()

	if m.contextManager != nil {
		if count, err := m.contextManager.GetActiveContextCount(context.Background()); err == nil {
			m.state.ContextCount = count
		}
	}

	if m.toolManager != nil {
		if tools, err := m.toolManager.ListTools(context.Background()); err == nil {
			m.state.ToolCount = len(tools)
		}
	}

	m.state.ProviderCount = len(m.providers)

	// Update features list
	features := []string{}
	if m.config.EnableMCP {
		features = append(features, "mcp")
	}
	if m.config.EnablePersistence {
		features = append(features, "persistence")
	}
	if m.config.EnableMetrics {
		features = append(features, "metrics")
	}
	if m.config.EnableCaching {
		features = append(features, "caching")
	}
	if m.config.EnableEventBus {
		features = append(features, "events")
	}
	m.state.Features = features
}

func (m *Module) getActiveProvider() interfaces.AIProvider {
	m.providerMutex.RLock()
	defer m.providerMutex.RUnlock()
	return m.activeProvider
}

func (m *Module) isRunning() bool {
	m.stateMutex.RLock()
	defer m.stateMutex.RUnlock()
	return m.started && m.state.Status == models.ModuleStatusRunning
}

func (m *Module) publishEvent(ctx context.Context, event *models.Event) {
	if m.deps.EventBus != nil {
		if err := m.deps.EventBus.PublishAsync(ctx, event); err != nil {
			if m.deps.Logger != nil {
				m.deps.Logger.Error("Failed to publish event", "event_type", event.Type, "error", err)
			}
		}
	}

	// Notify event listeners
	m.eventMutex.RLock()
	listeners := make([]EventListener, len(m.eventListeners))
	copy(listeners, m.eventListeners)
	m.eventMutex.RUnlock()

	for _, listener := range listeners {
		// Handle different event types
		switch event.Type {
		case models.EventTypeMessageSent:
			// Extract message from event data if available
			// This is a simplified implementation
		case models.EventTypeErrorOccurred:
			if data, ok := event.Data.(map[string]interface{}); ok {
				if errStr, ok := data["error"].(string); ok {
					err := fmt.Errorf("%s", errStr)
					listener.OnError(ctx, err, data)
				}
			}
		}
	}
}

func (m *Module) getConfigInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider":              m.config.Provider,
		"max_tokens":            m.config.DefaultMaxTokens,
		"timeout":               m.config.DefaultTimeout,
		"max_context_messages":  m.config.MaxContextMessages,
		"context_compaction":    m.config.ContextCompaction,
		"context_ttl":           m.config.ContextTTL,
		"enable_mcp":            m.config.EnableMCP,
		"enable_persistence":    m.config.EnablePersistence,
		"enable_metrics":        m.config.EnableMetrics,
		"enable_caching":        m.config.EnableCaching,
		"enable_event_bus":      m.config.EnableEventBus,
		"max_concurrent_requests": m.config.MaxConcurrentRequests,
	}
}

// Factory function to create a new AI module
func NewAIModule(ctx context.Context, config map[string]interface{}) (AIModule, error) {
	// Parse configuration
	aiConfig := DefaultConfig()

	// Apply configuration overrides
	if provider, ok := config["provider"].(string); ok {
		aiConfig.Provider = provider
	}

	if maxTokens, ok := config["max_tokens"]; ok {
		switch v := maxTokens.(type) {
		case int:
			aiConfig.DefaultMaxTokens = v
		case float64:
			aiConfig.DefaultMaxTokens = int(v)
		}
	}

	if timeout, ok := config["timeout"]; ok {
		switch v := timeout.(type) {
		case int:
			aiConfig.DefaultTimeout = time.Duration(v) * time.Second
		case float64:
			aiConfig.DefaultTimeout = time.Duration(v) * time.Second
		case string:
			if d, err := time.ParseDuration(v); err == nil {
				aiConfig.DefaultTimeout = d
			}
		}
	}

	if enableMCP, ok := config["enable_mcp"].(bool); ok {
		aiConfig.EnableMCP = enableMCP
	}

	if enablePersistence, ok := config["enable_persistence"].(bool); ok {
		aiConfig.EnablePersistence = enablePersistence
	}

	if enableMetrics, ok := config["enable_metrics"].(bool); ok {
		aiConfig.EnableMetrics = enableMetrics
	}

	if providers, ok := config["providers"].(map[string]interface{}); ok {
		aiConfig.Providers = providers
	}

	// Create minimal dependencies (would normally be injected)
	deps := &interfaces.Dependencies{
		Logger: nil, // Would be injected
		Metrics: nil, // Would be injected
		Database: nil, // Would be injected
		// ... other dependencies
	}

	return NewModule(ctx, aiConfig, deps)
}