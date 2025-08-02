package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	RegistryTypeLocal    = "local"
	RegistryTypeRemote   = "remote"
	RegistryTypeEmbedded = "embedded"
)

// RegistryManager manages multiple registry providers
type RegistryManager interface {
	// Aggregate operations (search across all registries)
	GetServer(name string) (*ImageMetadata, error)
	SearchServers(query string) ([]*ImageMetadata, error)
	ListServers() ([]*ImageMetadata, error)

	// Source-aware operations
	ListServersWithSource() ([]*ServerWithSource, error)

	// Registry access
	GetRegistry(id string) (Provider, error)
	ListRegistryInfo() []RegistryInfo

	// Registry management
	AddRegistry(config RegistryConfig) error
	RemoveRegistry(id string) error
	UpdateRegistry(id string, config RegistryConfig) error
	EnableRegistry(id string) error
	DisableRegistry(id string) error

	// Default registry management
	SetDefaultRegistry(id string) error
	GetDefaultRegistry() Provider
}

// registryManager implements the RegistryManager interface
type registryManager struct {
	providers map[string]Provider       // registry ID -> Provider
	configs   map[string]RegistryConfig // registry ID -> config
	defaultID string                    // current default registry
	mu        sync.RWMutex              // thread safety
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager() RegistryManager {
	return &registryManager{
		providers: make(map[string]Provider),
		configs:   make(map[string]RegistryConfig),
		mu:        sync.RWMutex{},
	}
}

// GetServer returns a server from any registry (priority order)
func (rm *registryManager) GetServer(name string) (*ImageMetadata, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Try default registry first
	if rm.defaultID != "" {
		if provider, exists := rm.providers[rm.defaultID]; exists {
			if config := rm.configs[rm.defaultID]; config.Enabled {
				if server, err := provider.GetServer(name); err == nil {
					return server, nil
				}
			}
		}
	}

	// Try other registries in priority order
	priorities := rm.getSortedRegistriesByPriority()
	for _, id := range priorities {
		if id == rm.defaultID {
			continue // already tried
		}
		if config := rm.configs[id]; config.Enabled {
			if provider, exists := rm.providers[id]; exists {
				if server, err := provider.GetServer(name); err == nil {
					return server, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("server not found: %s", name)
}

// SearchServers searches for servers across all enabled registries
func (rm *registryManager) SearchServers(query string) ([]*ImageMetadata, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var allResults []*ImageMetadata
	seenServers := make(map[string]bool)

	priorities := rm.getSortedRegistriesByPriority()
	for _, id := range priorities {
		if config := rm.configs[id]; config.Enabled {
			if provider, exists := rm.providers[id]; exists {
				if results, err := provider.SearchServers(query); err == nil {
					for _, server := range results {
						// Avoid duplicates (higher priority registry wins)
						if !seenServers[server.Name] {
							seenServers[server.Name] = true
							allResults = append(allResults, server)
						}
					}
				}
			}
		}
	}

	return allResults, nil
}

// ListServers returns all servers from enabled registries
func (rm *registryManager) ListServers() ([]*ImageMetadata, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var allServers []*ImageMetadata
	seenServers := make(map[string]bool)

	priorities := rm.getSortedRegistriesByPriority()
	for _, id := range priorities {
		if config := rm.configs[id]; config.Enabled {
			if provider, exists := rm.providers[id]; exists {
				if servers, err := provider.ListServers(); err == nil {
					for _, server := range servers {
						// Avoid duplicates (higher priority registry wins)
						if !seenServers[server.Name] {
							seenServers[server.Name] = true
							allServers = append(allServers, server)
						}
					}
				}
			}
		}
	}

	return allServers, nil
}

// ListServersWithSource returns all servers with their registry source information
func (rm *registryManager) ListServersWithSource() ([]*ServerWithSource, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var allServers []*ServerWithSource

	for id, config := range rm.configs {
		if !config.Enabled {
			continue
		}
		if provider, exists := rm.providers[id]; exists {
			if servers, err := provider.ListServers(); err == nil {
				for _, server := range servers {
					serverWithSource := &ServerWithSource{
						ImageMetadata: server,
						RegistryID:    id,
						RegistryName:  config.Name,
						Priority:      config.Priority,
					}
					allServers = append(allServers, serverWithSource)
				}
			}
		}
	}

	// Sort by priority then by name
	sort.Slice(allServers, func(i, j int) bool {
		if allServers[i].Priority != allServers[j].Priority {
			return allServers[i].Priority < allServers[j].Priority
		}
		return allServers[i].Name < allServers[j].Name
	})

	return allServers, nil
}

// GetRegistry returns a specific registry provider
func (rm *registryManager) GetRegistry(id string) (Provider, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if provider, exists := rm.providers[id]; exists {
		return provider, nil
	}
	return nil, fmt.Errorf("registry not found: %s", id)
}

// ListRegistryInfo returns information about all registries
func (rm *registryManager) ListRegistryInfo() []RegistryInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var infos []RegistryInfo
	for id, config := range rm.configs {
		info := RegistryInfo{
			RegistryConfig: config,
			LastChecked:    time.Now(),
		}

		// Check registry status and count servers
		if provider, exists := rm.providers[id]; exists && config.Enabled {
			if servers, err := provider.ListServers(); err == nil {
				info.Status = "online"
				info.ServerCount = len(servers)
			} else {
				info.Status = "error"
				info.ErrorMessage = err.Error()
			}
		} else if !config.Enabled {
			info.Status = "disabled"
		} else {
			info.Status = "offline"
		}

		infos = append(infos, info)
	}

	// Sort by priority then by name
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Priority != infos[j].Priority {
			return infos[i].Priority < infos[j].Priority
		}
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// AddRegistry adds a new registry
func (rm *registryManager) AddRegistry(config RegistryConfig) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Validate config
	if config.ID == "" {
		return fmt.Errorf("registry ID cannot be empty")
	}
	if config.Name == "" {
		return fmt.Errorf("registry name cannot be empty")
	}
	if _, exists := rm.configs[config.ID]; exists {
		return fmt.Errorf("registry with ID %s already exists", config.ID)
	}

	// Create provider based on type
	var provider Provider
	switch strings.ToLower(config.Type) {
	case RegistryTypeLocal:
		if config.Path == "" {
			provider = NewLocalRegistryProvider()
		} else {
			provider = NewLocalRegistryProvider(config.Path)
		}
	case RegistryTypeRemote:
		if config.URL == "" {
			return fmt.Errorf("remote registry requires URL")
		}
		provider = NewRemoteRegistryProvider(config.URL, config.AllowPrivateIp)
	case RegistryTypeEmbedded:
		provider = NewLocalRegistryProvider()
	default:
		return fmt.Errorf("unsupported registry type: %s", config.Type)
	}

	rm.configs[config.ID] = config
	rm.providers[config.ID] = provider

	// Set as default if it's the first registry
	if rm.defaultID == "" {
		rm.defaultID = config.ID
	}

	return nil
}

// RemoveRegistry removes a registry
func (rm *registryManager) RemoveRegistry(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.configs[id]; !exists {
		return fmt.Errorf("registry not found: %s", id)
	}

	delete(rm.configs, id)
	delete(rm.providers, id)

	// Update default if we removed the default registry
	if rm.defaultID == id {
		rm.defaultID = ""
		// Set new default to highest priority enabled registry
		priorities := rm.getSortedRegistriesByPriorityUnsafe()
		for _, newID := range priorities {
			if rm.configs[newID].Enabled {
				rm.defaultID = newID
				break
			}
		}
	}

	return nil
}

// UpdateRegistry updates an existing registry configuration
func (rm *registryManager) UpdateRegistry(id string, config RegistryConfig) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.configs[id]; !exists {
		return fmt.Errorf("registry not found: %s", id)
	}

	// Ensure ID matches
	config.ID = id

	// Update provider if URL/Path changed
	oldConfig := rm.configs[id]
	if oldConfig.URL != config.URL || oldConfig.Path != config.Path || oldConfig.Type != config.Type {
		var provider Provider
		switch strings.ToLower(config.Type) {
		case RegistryTypeLocal:
			if config.Path == "" {
				provider = NewLocalRegistryProvider()
			} else {
				provider = NewLocalRegistryProvider(config.Path)
			}
		case RegistryTypeRemote:
			if config.URL == "" {
				return fmt.Errorf("remote registry requires URL")
			}
			provider = NewRemoteRegistryProvider(config.URL, config.AllowPrivateIp)
		case RegistryTypeEmbedded:
			provider = NewLocalRegistryProvider()
		default:
			return fmt.Errorf("unsupported registry type: %s", config.Type)
		}
		rm.providers[id] = provider
	}

	rm.configs[id] = config
	return nil
}

// EnableRegistry enables a registry
func (rm *registryManager) EnableRegistry(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	config, exists := rm.configs[id]
	if !exists {
		return fmt.Errorf("registry not found: %s", id)
	}

	config.Enabled = true
	rm.configs[id] = config
	return nil
}

// DisableRegistry disables a registry
func (rm *registryManager) DisableRegistry(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	config, exists := rm.configs[id]
	if !exists {
		return fmt.Errorf("registry not found: %s", id)
	}

	config.Enabled = false
	rm.configs[id] = config

	// Update default if we disabled the default registry
	if rm.defaultID == id {
		rm.defaultID = ""
		// Set new default to highest priority enabled registry
		priorities := rm.getSortedRegistriesByPriorityUnsafe()
		for _, newID := range priorities {
			if rm.configs[newID].Enabled {
				rm.defaultID = newID
				break
			}
		}
	}

	return nil
}

// SetDefaultRegistry sets the default registry
func (rm *registryManager) SetDefaultRegistry(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	config, exists := rm.configs[id]
	if !exists {
		return fmt.Errorf("registry not found: %s", id)
	}
	if !config.Enabled {
		return fmt.Errorf("cannot set disabled registry as default: %s", id)
	}

	rm.defaultID = id
	return nil
}

// GetDefaultRegistry returns the default registry provider
func (rm *registryManager) GetDefaultRegistry() Provider {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.defaultID != "" {
		if provider, exists := rm.providers[rm.defaultID]; exists {
			if config := rm.configs[rm.defaultID]; config.Enabled {
				return provider
			}
		}
	}

	// Fallback to highest priority enabled registry
	priorities := rm.getSortedRegistriesByPriority()
	for _, id := range priorities {
		if config := rm.configs[id]; config.Enabled {
			if provider, exists := rm.providers[id]; exists {
				return provider
			}
		}
	}

	return nil
}

// getSortedRegistriesByPriority returns registry IDs sorted by priority (lower number = higher priority)
func (rm *registryManager) getSortedRegistriesByPriority() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.getSortedRegistriesByPriorityUnsafe()
}

// getSortedRegistriesByPriorityUnsafe returns registry IDs sorted by priority (internal, assumes lock held)
func (rm *registryManager) getSortedRegistriesByPriorityUnsafe() []string {
	var ids []string
	for id := range rm.configs {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		configI := rm.configs[ids[i]]
		configJ := rm.configs[ids[j]]
		if configI.Priority != configJ.Priority {
			return configI.Priority < configJ.Priority
		}
		return configI.Name < configJ.Name
	})

	return ids
}
