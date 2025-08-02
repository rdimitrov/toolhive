package registry

import (
	"testing"

	"github.com/stacklok/toolhive/pkg/config"
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
