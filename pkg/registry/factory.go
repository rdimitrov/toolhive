package registry

import (
	"sync"

	"github.com/stacklok/toolhive/pkg/config"
)

var (
	defaultManager     RegistryManager
	defaultManagerOnce sync.Once
	defaultManagerErr  error
)

// NewRegistryManagerFromConfig creates a new registry manager from the configuration
// This loads multi-registry configurations when available, or falls back to legacy single registry
func NewRegistryManagerFromConfig(cfg *config.Config) RegistryManager {
	manager := NewRegistryManager()

	if cfg == nil {
		// No config, use embedded registry as default
		registryConfig := RegistryConfig{
			ID:       "default",
			Name:     "Default Registry",
			Type:     RegistryTypeEmbedded,
			Priority: 1,
			Enabled:  true,
		}
		_ = manager.AddRegistry(registryConfig)
		return manager
	}

	// If multi-registry configuration is available, use it
	if len(cfg.Registries) > 0 {
		for _, configRegistry := range cfg.Registries {
			registryConfig := RegistryConfig{
				ID:             configRegistry.ID,
				Name:           configRegistry.Name,
				Type:           configRegistry.Type,
				URL:            configRegistry.URL,
				Path:           configRegistry.Path,
				Priority:       configRegistry.Priority,
				AllowPrivateIp: configRegistry.AllowPrivateIp,
				Enabled:        configRegistry.Enabled,
			}
			_ = manager.AddRegistry(registryConfig)
		}
		// Set default registry if specified
		if cfg.DefaultRegistryID != "" {
			_ = manager.SetDefaultRegistry(cfg.DefaultRegistryID)
		}
		return manager
	}

	// Fall back to legacy single registry configuration
	registryConfig := RegistryConfig{
		ID:       "default",
		Name:     "Default Registry",
		Priority: 1,
		Enabled:  true,
	}

	// Determine registry type and configuration from legacy fields
	if len(cfg.RegistryUrl) > 0 {
		registryConfig.Type = RegistryTypeRemote
		registryConfig.URL = cfg.RegistryUrl
		registryConfig.AllowPrivateIp = cfg.AllowPrivateRegistryIp
	} else if len(cfg.LocalRegistryPath) > 0 {
		registryConfig.Type = RegistryTypeLocal
		registryConfig.Path = cfg.LocalRegistryPath
	} else {
		registryConfig.Type = RegistryTypeEmbedded
	}

	// Add the registry to the manager
	if err := manager.AddRegistry(registryConfig); err != nil {
		// If we can't add the registry, return an empty manager
		// This shouldn't happen in practice, but provides a safe fallback
		return NewRegistryManager()
	}

	return manager
}

// GetDefaultManager returns the default multi-registry manager instance
func GetDefaultManager() (RegistryManager, error) {
	defaultManagerOnce.Do(func() {
		cfg, err := config.LoadOrCreateConfig()
		if err != nil {
			defaultManagerErr = err
			return
		}
		defaultManager = NewRegistryManagerFromConfig(cfg)
	})

	return defaultManager, defaultManagerErr
}
