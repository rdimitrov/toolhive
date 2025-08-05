package v1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stacklok/toolhive/pkg/logger"
	"github.com/stacklok/toolhive/pkg/registry"
)

// RegistryRoutes defines the routes for the registry API.
type RegistryRoutes struct {
	manager registry.RegistryManager
}

// RegistryRouter creates a new router for the registry API.
func RegistryRouter(manager registry.RegistryManager) http.Handler {
	routes := RegistryRoutes{manager: manager}

	r := chi.NewRouter()
	r.Get("/", routes.listRegistries)
	r.Post("/", routes.addRegistry)
	r.Get("/{name}", routes.getRegistry)
	r.Delete("/{name}", routes.removeRegistry)

	// Add nested routes for servers within a registry
	r.Route("/{name}/servers", func(r chi.Router) {
		r.Get("/", routes.listServers)
		r.Get("/{serverName}", routes.getServer)
	})
	return r
}

//	 listRegistries
//
//		@Summary		List registries
//		@Description	Get a list of the current registries
//		@Tags			registry
//		@Produce		json
//		@Success		200	{object}	registryListResponse
//		@Router			/api/v1beta/registry [get]
func (routes *RegistryRoutes) listRegistries(w http.ResponseWriter, _ *http.Request) {
	registryInfos := routes.manager.ListRegistryInfo()

	registries := make([]registryInfo, 0, len(registryInfos))
	for _, info := range registryInfos {
		registries = append(registries, registryInfo{
			Name:        info.ID,
			Version:     "1.0", // This could be extracted from registry data if needed
			LastUpdated: info.LastChecked.Format("2006-01-02T15:04:05Z"),
			ServerCount: info.ServerCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	response := registryListResponse{Registries: registries}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

//	 addRegistry
//
//		@Summary		Add a registry
//		@Description	Add a new registry
//		@Tags			registry
//		@Accept			json
//		@Produce		json
//		@Param			body	body		addRegistryRequest	true	"Registry configuration"
//		@Success		201		{object}	registryInfo
//		@Failure		400		{string}	string	"Bad Request"
//		@Failure		409		{string}	string	"Conflict - Registry already exists"
//		@Router			/api/v1beta/registry [post]
func (routes *RegistryRoutes) addRegistry(w http.ResponseWriter, r *http.Request) {
	var req addRegistryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ID == "" {
		http.Error(w, "Registry ID is required", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Registry name is required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, "Registry type is required", http.StatusBadRequest)
		return
	}

	// Create registry configuration
	config := registry.RegistryConfig{
		ID:             req.ID,
		Name:           req.Name,
		Type:           req.Type,
		URL:            req.URL,
		Path:           req.Path,
		Priority:       req.Priority,
		AllowPrivateIp: req.AllowPrivateIp,
		Enabled:        true, // New registries are enabled by default
	}

	// Add the registry
	if err := routes.manager.AddRegistry(config); err != nil {
		if err.Error() == fmt.Sprintf("registry with ID %s already exists", req.ID) {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	// Store the configuration changes to disk
	if err := routes.manager.SaveToConfig(); err != nil {
		// Log the error but don't fail the request since the registry was added successfully
		logger.Errorf("Failed to save registry configuration to disk: %v", err)
	}

	// Return the created registry info
	registryInfos := routes.manager.ListRegistryInfo()
	for _, info := range registryInfos {
		if info.ID == req.ID {
			response := registryInfo{
				Name:        info.ID,
				Version:     "1.0",
				LastUpdated: info.LastChecked.Format("2006-01-02T15:04:05Z"),
				ServerCount: info.ServerCount,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	http.Error(w, "Failed to retrieve created registry", http.StatusInternalServerError)
}

//	 getRegistry
//
//		@Summary		Get a registry
//		@Description	Get details of a specific registry
//		@Tags			registry
//		@Produce		json
//		@Param			name	path		string	true	"Registry name/ID"
//		@Success		200	{object}	getRegistryResponse
//		@Failure		404	{string}	string	"Not Found"
//		@Router			/api/v1beta/registry/{name} [get]
func (routes *RegistryRoutes) getRegistry(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Get the specific registry provider
	provider, err := routes.manager.GetRegistry(name)
	if err != nil {
		http.Error(w, "Registry not found", http.StatusNotFound)
		return
	}

	// Get the registry data
	reg, err := provider.GetRegistry()
	if err != nil {
		http.Error(w, "Failed to get registry data", http.StatusInternalServerError)
		return
	}

	// Get registry info for metadata
	registryInfos := routes.manager.ListRegistryInfo()
	var registryInfo *registry.RegistryInfo
	for _, info := range registryInfos {
		if info.ID == name {
			registryInfo = &info
			break
		}
	}

	if registryInfo == nil {
		http.Error(w, "Registry metadata not found", http.StatusInternalServerError)
		return
	}

	response := getRegistryResponse{
		Name:        registryInfo.ID,
		Version:     reg.Version,
		LastUpdated: reg.LastUpdated,
		ServerCount: len(reg.Servers),
		Registry:    reg,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Errorf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

//	 removeRegistry
//
//		@Summary		Remove a registry
//		@Description	Remove a specific registry
//		@Tags			registry
//		@Produce		json
//		@Param			name	path		string	true	"Registry name/ID"
//		@Success		204	{string}	string	"No Content"
//		@Failure		400	{string}	string	"Bad Request"
//		@Failure		404	{string}	string	"Not Found"
//		@Router			/api/v1beta/registry/{name} [delete]
func (routes *RegistryRoutes) removeRegistry(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Attempt to remove the registry
	if err := routes.manager.RemoveRegistry(name); err != nil {
		if err.Error() == fmt.Sprintf("registry not found: %s", name) {
			http.Error(w, "Registry not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	// Store the configuration changes to disk
	if err := routes.manager.SaveToConfig(); err != nil {
		// Log the error but don't fail the request since the registry was removed successfully
		logger.Errorf("Failed to save registry configuration to disk: %v", err)
	}

	// Successfully removed
	w.WriteHeader(http.StatusNoContent)
}

//	 listServers
//
//		@Summary		List servers in a registry
//		@Description	Get a list of servers in a specific registry
//		@Tags			registry
//		@Produce		json
//		@Param			name	path		string	true	"Registry name/ID"
//		@Success		200	{object}	listServersResponse
//		@Failure		404	{string}	string	"Not Found"
//		@Router			/api/v1beta/registry/{name}/servers [get]
func (routes *RegistryRoutes) listServers(w http.ResponseWriter, r *http.Request) {
	registryName := chi.URLParam(r, "name")

	// Get the specific registry provider
	provider, err := routes.manager.GetRegistry(registryName)
	if err != nil {
		http.Error(w, "Registry not found", http.StatusNotFound)
		return
	}

	servers, err := provider.ListServers()
	if err != nil {
		logger.Errorf("Failed to list servers: %v", err)
		http.Error(w, "Failed to list servers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := listServersResponse{Servers: servers}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Errorf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

//	 getServer
//
//		@Summary		Get a server from a registry
//		@Description	Get details of a specific server in a registry
//		@Tags			registry
//		@Produce		json
//		@Param			name		path		string	true	"Registry name/ID"
//		@Param			serverName	path		string	true	"Server name"
//		@Success		200	{object}	getServerResponse
//		@Failure		404	{string}	string	"Not Found"
//		@Router			/api/v1beta/registry/{name}/servers/{serverName} [get]
func (routes *RegistryRoutes) getServer(w http.ResponseWriter, r *http.Request) {
	registryName := chi.URLParam(r, "name")
	serverName := chi.URLParam(r, "serverName")

	// Get the specific registry provider
	provider, err := routes.manager.GetRegistry(registryName)
	if err != nil {
		http.Error(w, "Registry not found", http.StatusNotFound)
		return
	}

	server, err := provider.GetServer(serverName)
	if err != nil {
		logger.Errorf("Failed to get server '%s': %v", serverName, err)
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := getServerResponse{Server: server}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Errorf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Response type definitions.

// registryInfo represents basic information about a registry
//
//	@Description	Basic information about a registry
type registryInfo struct {
	// Name of the registry
	Name string `json:"name"`
	// Version of the registry schema
	Version string `json:"version"`
	// Last updated timestamp
	LastUpdated string `json:"last_updated"`
	// Number of servers in the registry
	ServerCount int `json:"server_count"`
}

// registryListResponse represents the response for listing registries
//
//	@Description	Response containing a list of registries
type registryListResponse struct {
	// List of registries
	Registries []registryInfo `json:"registries"`
}

// getRegistryResponse represents the response for getting a registry
//
//	@Description	Response containing registry details
type getRegistryResponse struct {
	// Name of the registry
	Name string `json:"name"`
	// Version of the registry schema
	Version string `json:"version"`
	// Last updated timestamp
	LastUpdated string `json:"last_updated"`
	// Number of servers in the registry
	ServerCount int `json:"server_count"`
	// Full registry data
	Registry *registry.Registry `json:"registry"`
}

// listServersResponse represents the response for listing servers in a registry
//
//	@Description	Response containing a list of servers
type listServersResponse struct {
	// List of servers in the registry
	Servers []*registry.ImageMetadata `json:"servers"`
}

// getServerResponse represents the response for getting a server from a registry
//
//	@Description	Response containing server details
type getServerResponse struct {
	// Server details
	Server *registry.ImageMetadata `json:"server"`
}

// addRegistryRequest represents the request for adding a new registry
//
//	@Description	Request to add a new registry
type addRegistryRequest struct {
	// Unique identifier for the registry
	ID string `json:"id"`
	// Display name for the registry
	Name string `json:"name"`
	// Type of registry (remote, local, embedded)
	Type string `json:"type"`
	// URL for remote registries
	URL string `json:"url,omitempty"`
	// File path for local registries
	Path string `json:"path,omitempty"`
	// Priority for registry resolution (lower number = higher priority)
	Priority int `json:"priority"`
	// Whether to allow private IP addresses for remote registries
	AllowPrivateIp bool `json:"allow_private_ip"`
}
