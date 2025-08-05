package registry

import (
	"fmt"
	"os"
	"strings"
)

// LocalRegistryProvider provides registry data from local files
type LocalRegistryProvider struct {
	filePath string
}

// NewLocalRegistryProvider creates a new local registry provider
// filePath is required and must point to a valid registry JSON file
func NewLocalRegistryProvider(filePath string) *LocalRegistryProvider {
	return &LocalRegistryProvider{filePath: filePath}
}

// GetRegistry returns the registry data from local file
func (p *LocalRegistryProvider) GetRegistry() (*Registry, error) {
	if p.filePath == "" {
		return nil, fmt.Errorf("local registry provider requires a file path")
	}

	data, err := os.ReadFile(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local registry file %s: %w", p.filePath, err)
	}

	registry, err := parseRegistryData(data)
	if err != nil {
		return nil, err
	}

	// Set name field on each server based on map key
	for name, server := range registry.Servers {
		server.Name = name
	}

	return registry, nil
}

// GetServer returns a specific server by name
func (p *LocalRegistryProvider) GetServer(name string) (*ImageMetadata, error) {
	reg, err := p.GetRegistry()
	if err != nil {
		return nil, err
	}

	server, ok := reg.Servers[name]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	return server, nil
}

// SearchServers searches for servers matching the query
func (p *LocalRegistryProvider) SearchServers(query string) ([]*ImageMetadata, error) {
	reg, err := p.GetRegistry()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []*ImageMetadata

	for name, server := range reg.Servers {
		// Search in name
		if strings.Contains(strings.ToLower(name), query) {
			results = append(results, server)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(server.Description), query) {
			results = append(results, server)
			continue
		}

		// Search in tags
		for _, tag := range server.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, server)
				break
			}
		}
	}

	return results, nil
}

// ListServers returns all available servers
func (p *LocalRegistryProvider) ListServers() ([]*ImageMetadata, error) {
	reg, err := p.GetRegistry()
	if err != nil {
		return nil, err
	}

	servers := make([]*ImageMetadata, 0, len(reg.Servers))
	for _, server := range reg.Servers {
		servers = append(servers, server)
	}

	return servers, nil
}
