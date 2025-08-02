package registry

import (
	"sync"

	"github.com/stacklok/toolhive/pkg/config"
)

var (
	defaultProvider     Provider
	defaultProviderOnce sync.Once
	defaultProviderErr  error
)

// NewRegistryProvider creates a new registry provider based on the configuration
func NewRegistryProvider(cfg *config.Config) Provider {
	if cfg != nil && len(cfg.RegistryUrl) > 0 {
		return NewRemoteRegistryProvider(cfg.RegistryUrl, cfg.AllowPrivateRegistryIp)
	}
	if cfg != nil && len(cfg.LocalRegistryPath) > 0 {
		return NewLocalRegistryProvider(cfg.LocalRegistryPath)
	}
	return NewEmbeddedRegistryProvider()
}

// GetDefaultProvider returns the default registry provider instance
// This maintains backward compatibility with the existing singleton pattern
func GetDefaultProvider() (Provider, error) {
	defaultProviderOnce.Do(func() {
		cfg, err := config.LoadOrCreateConfig()
		if err != nil {
			defaultProviderErr = err
			return
		}
		defaultProvider = NewRegistryProvider(cfg)
	})

	return defaultProvider, defaultProviderErr
}

// NewRegistryManagerFromConfig creates a new registry manager from the configuration
// This converts the existing single registry configuration to the multi-registry format
func NewRegistryManagerFromConfig(cfg *config.Config) RegistryManager {
	manager := NewRegistryManager()

	// Convert existing configuration to registry config
	registryConfig := RegistryConfig{
		ID:       "default",
		Name:     "Default Registry",
		Priority: 1,
		Enabled:  true,
	}

	// Determine registry type and configuration
	if cfg != nil && len(cfg.RegistryUrl) > 0 {
		registryConfig.Type = RegistryTypeRemote
		registryConfig.URL = cfg.RegistryUrl
		registryConfig.AllowPrivateIp = cfg.AllowPrivateRegistryIp
	} else if cfg != nil && len(cfg.LocalRegistryPath) > 0 {
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
