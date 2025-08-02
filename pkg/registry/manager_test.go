package registry

import (
	"testing"

	"github.com/stacklok/toolhive/pkg/config"
	"github.com/stacklok/toolhive/pkg/logger"
)

func TestNewRegistryManager(t *testing.T) {
	manager := NewRegistryManager()
	if manager == nil {
		t.Fatal("NewRegistryManager should not return nil")
	}

	// Should start with no registries
	infos := manager.ListRegistryInfo()
	if len(infos) != 0 {
		t.Errorf("Expected 0 registries, got %d", len(infos))
	}

	// Should return nil for default provider when no registries exist
	defaultProvider := manager.GetDefaultRegistry()
	if defaultProvider != nil {
		t.Error("Expected nil default provider when no registries exist")
	}
}

func TestNewRegistryManagerFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       *config.Config
		expectedType string
		expectedID   string
		expectedName string
	}{
		{
			name:         "nil config - embedded",
			config:       nil,
			expectedType: "embedded",
			expectedID:   "default",
			expectedName: "Default Registry",
		},
		{
			name:         "empty config - embedded",
			config:       &config.Config{},
			expectedType: "embedded",
			expectedID:   "default",
			expectedName: "Default Registry",
		},
		{
			name: "remote registry config",
			config: &config.Config{
				RegistryUrl:            "https://example.com/registry.json",
				AllowPrivateRegistryIp: true,
			},
			expectedType: "remote",
			expectedID:   "default",
			expectedName: "Default Registry",
		},
		{
			name: "local registry config",
			config: &config.Config{
				LocalRegistryPath: "/path/to/registry.json",
			},
			expectedType: "local",
			expectedID:   "default",
			expectedName: "Default Registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewRegistryManagerFromConfig(tt.config)
			if manager == nil {
				t.Fatal("NewRegistryManagerFromConfig should not return nil")
			}

			infos := manager.ListRegistryInfo()
			if len(infos) != 1 {
				t.Fatalf("Expected 1 registry, got %d", len(infos))
			}

			info := infos[0]
			if info.ID != tt.expectedID {
				t.Errorf("Expected ID %s, got %s", tt.expectedID, info.ID)
			}
			if info.Name != tt.expectedName {
				t.Errorf("Expected name %s, got %s", tt.expectedName, info.Name)
			}
			if info.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, info.Type)
			}
			if !info.Enabled {
				t.Error("Expected registry to be enabled")
			}

			// Should set this as default
			defaultProvider := manager.GetDefaultRegistry()
			if defaultProvider == nil {
				t.Error("Expected default provider to be set")
			}
		})
	}
}

func TestRegistryManagement(t *testing.T) {
	manager := NewRegistryManager()

	// Test adding registry
	regConfig := RegistryConfig{
		ID:       "test1",
		Name:     "Test Registry 1",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}

	err := manager.AddRegistry(regConfig)
	if err != nil {
		t.Fatalf("Failed to add registry: %v", err)
	}

	// Test listing registries
	infos := manager.ListRegistryInfo()
	if len(infos) != 1 {
		t.Fatalf("Expected 1 registry, got %d", len(infos))
	}
	if infos[0].ID != "test1" {
		t.Errorf("Expected ID test1, got %s", infos[0].ID)
	}

	// Test getting specific registry
	provider, err := manager.GetRegistry("test1")
	if err != nil {
		t.Fatalf("Failed to get registry: %v", err)
	}
	if provider == nil {
		t.Error("Expected provider, got nil")
	}

	// Test getting non-existent registry
	_, err = manager.GetRegistry("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent registry")
	}

	// Test adding duplicate registry
	err = manager.AddRegistry(regConfig)
	if err == nil {
		t.Error("Expected error when adding duplicate registry")
	}

	// Test removing registry
	err = manager.RemoveRegistry("test1")
	if err != nil {
		t.Fatalf("Failed to remove registry: %v", err)
	}

	infos = manager.ListRegistryInfo()
	if len(infos) != 0 {
		t.Errorf("Expected 0 registries after removal, got %d", len(infos))
	}
}

func TestRegistryEnableDisable(t *testing.T) {
	manager := NewRegistryManager()

	regConfig := RegistryConfig{
		ID:       "test1",
		Name:     "Test Registry 1",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}

	err := manager.AddRegistry(regConfig)
	if err != nil {
		t.Fatalf("Failed to add registry: %v", err)
	}

	// Test disabling registry
	err = manager.DisableRegistry("test1")
	if err != nil {
		t.Fatalf("Failed to disable registry: %v", err)
	}

	infos := manager.ListRegistryInfo()
	if infos[0].Enabled {
		t.Error("Expected registry to be disabled")
	}

	// Test enabling registry
	err = manager.EnableRegistry("test1")
	if err != nil {
		t.Fatalf("Failed to enable registry: %v", err)
	}

	infos = manager.ListRegistryInfo()
	if !infos[0].Enabled {
		t.Error("Expected registry to be enabled")
	}

	// Test enabling non-existent registry
	err = manager.EnableRegistry("nonexistent")
	if err == nil {
		t.Error("Expected error when enabling non-existent registry")
	}
}

func TestDefaultRegistry(t *testing.T) {
	manager := NewRegistryManager()

	// Add first registry
	config1 := RegistryConfig{
		ID:       "test1",
		Name:     "Test Registry 1",
		Type:     "embedded",
		Priority: 2,
		Enabled:  true,
	}
	err := manager.AddRegistry(config1)
	if err != nil {
		t.Fatalf("Failed to add registry: %v", err)
	}

	// Should be set as default (first registry)
	defaultProvider := manager.GetDefaultRegistry()
	if defaultProvider == nil {
		t.Error("Expected default provider to be set")
	}

	// Add second registry with higher priority
	config2 := RegistryConfig{
		ID:       "test2",
		Name:     "Test Registry 2",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}
	err = manager.AddRegistry(config2)
	if err != nil {
		t.Fatalf("Failed to add registry: %v", err)
	}

	// Default should still be test1 (explicitly set)
	defaultProvider = manager.GetDefaultRegistry()
	if defaultProvider == nil {
		t.Error("Expected default provider to be set")
	}

	// Set test2 as default
	err = manager.SetDefaultRegistry("test2")
	if err != nil {
		t.Fatalf("Failed to set default registry: %v", err)
	}

	// Test setting non-existent registry as default
	err = manager.SetDefaultRegistry("nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent registry as default")
	}

	// Test setting disabled registry as default
	err = manager.DisableRegistry("test2")
	if err != nil {
		t.Fatalf("Failed to disable registry: %v", err)
	}

	err = manager.SetDefaultRegistry("test2")
	if err == nil {
		t.Error("Expected error when setting disabled registry as default")
	}
}

func TestAggregateOperations(t *testing.T) {
	manager := NewRegistryManager()

	// Create test registries with mock providers
	config1 := RegistryConfig{
		ID:       "test1",
		Name:     "Test Registry 1",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}
	config2 := RegistryConfig{
		ID:       "test2",
		Name:     "Test Registry 2",
		Type:     "embedded",
		Priority: 2,
		Enabled:  true,
	}

	// Add registries
	err := manager.AddRegistry(config1)
	if err != nil {
		t.Fatalf("Failed to add registry 1: %v", err)
	}
	err = manager.AddRegistry(config2)
	if err != nil {
		t.Fatalf("Failed to add registry 2: %v", err)
	}

	// Note: The actual aggregate operations will work once we have real providers
	// For now, we test that the methods exist and don't panic

	// Test GetServer
	_, err = manager.GetServer("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent server")
	}

	// Test SearchServers
	results, err := manager.SearchServers("test")
	if err != nil {
		t.Fatalf("SearchServers failed: %v", err)
	}
	if results == nil {
		t.Error("SearchServers should not return nil")
	}

	// Test ListServers
	servers, err := manager.ListServers()
	if err != nil {
		t.Fatalf("ListServers failed: %v", err)
	}
	if servers == nil {
		t.Error("ListServers should not return nil")
	}

	// Test ListServersWithSource
	serversWithSource, err := manager.ListServersWithSource()
	if err != nil {
		t.Fatalf("ListServersWithSource failed: %v", err)
	}
	if serversWithSource == nil {
		t.Error("ListServersWithSource should not return nil")
	}
}

func TestRegistryValidation(t *testing.T) {
	manager := NewRegistryManager()

	// Test adding registry with empty ID
	regConfig := RegistryConfig{
		ID:       "",
		Name:     "Test Registry",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}
	err := manager.AddRegistry(regConfig)
	if err == nil {
		t.Error("Expected error for empty registry ID")
	}

	// Test adding registry with empty name
	regConfig.ID = "test1"
	regConfig.Name = ""
	err = manager.AddRegistry(regConfig)
	if err == nil {
		t.Error("Expected error for empty registry name")
	}

	// Test adding remote registry without URL
	regConfig.Name = "Test Registry"
	regConfig.Type = "remote"
	regConfig.URL = ""
	err = manager.AddRegistry(regConfig)
	if err == nil {
		t.Error("Expected error for remote registry without URL")
	}

	// Test adding registry with unsupported type
	regConfig.Type = "unsupported"
	err = manager.AddRegistry(regConfig)
	if err == nil {
		t.Error("Expected error for unsupported registry type")
	}
}

func TestUpdateRegistry(t *testing.T) {
	manager := NewRegistryManager()

	// Add initial registry
	regConfig := RegistryConfig{
		ID:       "test1",
		Name:     "Test Registry 1",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}
	err := manager.AddRegistry(regConfig)
	if err != nil {
		t.Fatalf("Failed to add registry: %v", err)
	}

	// Update registry
	updatedConfig := RegistryConfig{
		ID:       "test1", // ID should match
		Name:     "Updated Test Registry 1",
		Type:     "embedded",
		Priority: 5,
		Enabled:  false,
	}
	err = manager.UpdateRegistry("test1", updatedConfig)
	if err != nil {
		t.Fatalf("Failed to update registry: %v", err)
	}

	// Verify update
	infos := manager.ListRegistryInfo()
	if len(infos) != 1 {
		t.Fatalf("Expected 1 registry, got %d", len(infos))
	}
	if infos[0].Name != "Updated Test Registry 1" {
		t.Errorf("Expected updated name, got %s", infos[0].Name)
	}
	if infos[0].Priority != 5 {
		t.Errorf("Expected priority 5, got %d", infos[0].Priority)
	}
	if infos[0].Enabled {
		t.Error("Expected registry to be disabled")
	}

	// Test updating non-existent registry
	err = manager.UpdateRegistry("nonexistent", updatedConfig)
	if err == nil {
		t.Error("Expected error when updating non-existent registry")
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewRegistryManager()

	// Add a registry
	regConfig := RegistryConfig{
		ID:       "test1",
		Name:     "Test Registry 1",
		Type:     "embedded",
		Priority: 1,
		Enabled:  true,
	}
	err := manager.AddRegistry(regConfig)
	if err != nil {
		t.Fatalf("Failed to add registry: %v", err)
	}

	// Test concurrent reads (should not panic or deadlock)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			manager.ListRegistryInfo()
			manager.GetDefaultRegistry()
			manager.GetRegistry("test1")
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSaveToConfig(t *testing.T) {
	t.Parallel()
	
	// Initialize logger for config operations
	logger.Initialize()
	
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := tempDir + "/test_config.yaml"
	
	// Create manager with test registries
	manager := NewRegistryManager()
	
	// Add embedded registry
	embeddedConfig := RegistryConfig{
		ID:       "embedded-test",
		Name:     "Embedded Test Registry",
		Type:     RegistryTypeEmbedded,
		Priority: 1,
		Enabled:  true,
	}
	err := manager.AddRegistry(embeddedConfig)
	if err != nil {
		t.Fatalf("Failed to add embedded registry: %v", err)
	}
	
	// Add remote registry
	remoteConfig := RegistryConfig{
		ID:             "remote-test",
		Name:           "Remote Test Registry",
		Type:           RegistryTypeRemote,
		URL:            "https://example.com/registry.json",
		Priority:       2,
		AllowPrivateIp: false,
		Enabled:        true,
	}
	err = manager.AddRegistry(remoteConfig)
	if err != nil {
		t.Fatalf("Failed to add remote registry: %v", err)
	}
	
	// Test SaveToConfigPath
	err = manager.SaveToConfigPath(configPath)
	if err != nil {
		t.Fatalf("SaveToConfigPath failed: %v", err)
	}
	
	// Load config and verify registries were saved
	cfg, err := config.LoadOrCreateConfigWithPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}
	
	if len(cfg.Registries) != 2 {
		t.Errorf("Expected 2 registries in config, got %d", len(cfg.Registries))
	}
	
	// Verify embedded registry was saved
	var embeddedFound, remoteFound bool
	for _, reg := range cfg.Registries {
		if reg.ID == "embedded-test" {
			embeddedFound = true
			if reg.Name != "Embedded Test Registry" {
				t.Errorf("Expected embedded registry name 'Embedded Test Registry', got '%s'", reg.Name)
			}
			if reg.Type != "embedded" {
				t.Errorf("Expected embedded registry type 'embedded', got '%s'", reg.Type)
			}
		}
		if reg.ID == "remote-test" {
			remoteFound = true
			if reg.URL != "https://example.com/registry.json" {
				t.Errorf("Expected remote registry URL 'https://example.com/registry.json', got '%s'", reg.URL)
			}
			if reg.AllowPrivateIp != false {
				t.Errorf("Expected remote registry AllowPrivateIp false, got %v", reg.AllowPrivateIp)
			}
		}
	}
	
	if !embeddedFound {
		t.Error("Embedded registry not found in saved config")
	}
	if !remoteFound {
		t.Error("Remote registry not found in saved config")
	}
}

func TestSaveToConfigWithDefaultRegistry(t *testing.T) {
	t.Parallel()
	
	// Initialize logger for config operations
	logger.Initialize()
	
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := tempDir + "/test_config_with_default.yaml"
	
	// Create manager with test registries
	manager := NewRegistryManager()
	
	// Add registries
	config1 := RegistryConfig{
		ID:       "registry1",
		Name:     "Registry 1",
		Type:     RegistryTypeEmbedded,
		Priority: 2,
		Enabled:  true,
	}
	config2 := RegistryConfig{
		ID:       "registry2", 
		Name:     "Registry 2",
		Type:     RegistryTypeEmbedded,
		Priority: 1,
		Enabled:  true,
	}
	
	err := manager.AddRegistry(config1)
	if err != nil {
		t.Fatalf("Failed to add registry1: %v", err)
	}
	err = manager.AddRegistry(config2)
	if err != nil {
		t.Fatalf("Failed to add registry2: %v", err)
	}
	
	// Set specific default registry
	err = manager.SetDefaultRegistry("registry1")
	if err != nil {
		t.Fatalf("Failed to set default registry: %v", err)
	}
	
	// Save to config
	err = manager.SaveToConfigPath(configPath)
	if err != nil {
		t.Fatalf("SaveToConfigPath failed: %v", err)
	}
	
	// Load and verify default registry was saved
	cfg, err := config.LoadOrCreateConfigWithPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}
	
	if cfg.DefaultRegistryID != "registry1" {
		t.Errorf("Expected default registry ID 'registry1', got '%s'", cfg.DefaultRegistryID)
	}
}

func TestGetRegistry(t *testing.T) {
	t.Parallel()
	
	manager := NewRegistryManager()
	
	// Test getting non-existent registry
	_, err := manager.GetRegistry("non-existent")
	if err == nil {
		t.Error("Expected error when getting non-existent registry")
	}
	
	// Add a test registry
	testConfig := RegistryConfig{
		ID:       "test-registry",
		Name:     "Test Registry",
		Type:     RegistryTypeEmbedded,
		Priority: 1,
		Enabled:  true,
	}
	err = manager.AddRegistry(testConfig)
	if err != nil {
		t.Fatalf("Failed to add test registry: %v", err)
	}
	
	// Test getting existing registry
	provider, err := manager.GetRegistry("test-registry")
	if err != nil {
		t.Fatalf("Failed to get existing registry: %v", err)
	}
	if provider == nil {
		t.Error("Expected non-nil provider for existing registry")
	}
	
	// Test that we can use the provider
	_, err = provider.ListServers()
	if err != nil {
		t.Errorf("Failed to list servers from retrieved provider: %v", err)
	}
}

func TestRegistryManagerErrorHandling(t *testing.T) {
	t.Parallel()
	
	manager := NewRegistryManager()
	
	// Test adding registry with empty ID
	invalidConfig := RegistryConfig{
		ID:   "",
		Name: "Invalid Registry",
		Type: RegistryTypeEmbedded,
	}
	err := manager.AddRegistry(invalidConfig)
	if err == nil {
		t.Error("Expected error when adding registry with empty ID")
	}
	
	// Test adding registry with empty name
	invalidConfig.ID = "valid-id"
	invalidConfig.Name = ""
	err = manager.AddRegistry(invalidConfig)
	if err == nil {
		t.Error("Expected error when adding registry with empty name")
	}
	
	// Test adding registry with empty type
	invalidConfig.Name = "Valid Name"
	invalidConfig.Type = ""
	err = manager.AddRegistry(invalidConfig)
	if err == nil {
		t.Error("Expected error when adding registry with empty type")
	}
	
	// Test adding remote registry without URL
	invalidConfig.Type = RegistryTypeRemote
	invalidConfig.URL = ""
	err = manager.AddRegistry(invalidConfig)
	if err == nil {
		t.Error("Expected error when adding remote registry without URL")
	}
	
	// Test adding local registry without path
	invalidConfig.Type = RegistryTypeLocal
	invalidConfig.Path = ""
	err = manager.AddRegistry(invalidConfig)
	if err == nil {
		t.Error("Expected error when adding local registry without path")
	}
	
	// Test operations on non-existent registry
	err = manager.RemoveRegistry("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent registry")
	}
	
	err = manager.EnableRegistry("non-existent")
	if err == nil {
		t.Error("Expected error when enabling non-existent registry")
	}
	
	err = manager.DisableRegistry("non-existent")
	if err == nil {
		t.Error("Expected error when disabling non-existent registry")
	}
	
	err = manager.SetDefaultRegistry("non-existent")
	if err == nil {
		t.Error("Expected error when setting non-existent default registry")
	}
	
	updateConfig := RegistryConfig{
		ID:   "non-existent",
		Name: "Updated Registry",
		Type: RegistryTypeEmbedded,
	}
	err = manager.UpdateRegistry("non-existent", updateConfig)
	if err == nil {
		t.Error("Expected error when updating non-existent registry")
	}
}
