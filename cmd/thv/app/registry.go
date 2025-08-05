package app

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/stacklok/toolhive/pkg/registry"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage MCP server registry",
	Long:  `Manage the MCP server registry, including listing and getting information about available MCP servers.`,
}

var registryListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available MCP servers",
	Long:    `List all available MCP servers in the registry.`,
	RunE:    registryListCmdFunc,
}

var registryInfoCmd = &cobra.Command{
	Use:   "info [server]",
	Short: "Get information about an MCP server",
	Long:  `Get detailed information about a specific MCP server in the registry.`,
	Args:  cobra.ExactArgs(1),
	RunE:  registryInfoCmdFunc,
}

var registrySourcesCmd = &cobra.Command{
	Use:     "sources",
	Aliases: []string{"src"},
	Short:   "List configured registry sources",
	Long:    `List all configured registry sources and their status.`,
	RunE:    registrySourcesCmdFunc,
}

var registryAddCmd = &cobra.Command{
	Use:   "add <id> <url-or-path>",
	Short: "Add a new registry source",
	Long: `Add a new registry source with the specified ID and URL or file path.
The command automatically detects whether the input is a URL or file path.

Examples:
  thv registry add custom https://example.com/registry.json
  thv registry add local /path/to/local-registry.json
  thv registry add internal file:///path/to/internal-registry.json`,
	Args: cobra.ExactArgs(2),
	RunE: registryAddCmdFunc,
}

var registryRemoveCmd = &cobra.Command{
	Use:     "remove <id>",
	Aliases: []string{"rm"},
	Short:   "Remove a registry source",
	Long:    `Remove a registry source by ID.`,
	Args:    cobra.ExactArgs(1),
	RunE:    registryRemoveCmdFunc,
}

var registryEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a registry source",
	Long:  `Enable a registry source by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  registryEnableCmdFunc,
}

var registryDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a registry source",
	Long:  `Disable a registry source by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  registryDisableCmdFunc,
}

var registrySetDefaultCmd = &cobra.Command{
	Use:   "set-default <id>",
	Short: "Set the default registry source",
	Long:  `Set the default registry source by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  registrySetDefaultCmdFunc,
}

var registryUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a registry source",
	Long:  `Update configuration for an existing registry source.`,
	Args:  cobra.ExactArgs(1),
	RunE:  registryUpdateCmdFunc,
}

var (
	registryFormat        string
	registryName          string
	registryPriority      int
	allowPrivateIp        bool
	registryURL           string
	registryPath          string
	sourceRegistry        string
	showSource            bool
	allSources            bool
)

func init() {
	// Add registry command to root command
	rootCmd.AddCommand(registryCmd)

	// Add subcommands to registry command - Server operations
	registryCmd.AddCommand(registryListCmd)
	registryCmd.AddCommand(registryInfoCmd)

	// Add subcommands to registry command - Registry management
	registryCmd.AddCommand(registrySourcesCmd)
	registryCmd.AddCommand(registryAddCmd)
	registryCmd.AddCommand(registryRemoveCmd)
	registryCmd.AddCommand(registryEnableCmd)
	registryCmd.AddCommand(registryDisableCmd)
	registryCmd.AddCommand(registrySetDefaultCmd)
	registryCmd.AddCommand(registryUpdateCmd)

	// Add flags for server operations
	registryListCmd.Flags().StringVar(&registryFormat, "format", FormatText, "Output format (json or text)")
	registryListCmd.Flags().StringVar(&sourceRegistry, "source", "", "List servers from specific registry source")
	registryListCmd.Flags().BoolVar(&showSource, "show-source", false, "Show source registry for each server")
	registryListCmd.Flags().BoolVar(&allSources, "all-sources", false, "List from all registries including duplicates")

	registryInfoCmd.Flags().StringVar(&registryFormat, "format", FormatText, "Output format (json or text)")
	registryInfoCmd.Flags().StringVar(&sourceRegistry, "source", "", "Get server info from specific registry source")

	// Add flags for registry management
	registrySourcesCmd.Flags().StringVar(&registryFormat, "format", FormatText, "Output format (json or text)")

	registryAddCmd.Flags().StringVar(&registryName, "name", "", "Human-readable name for the registry")
	registryAddCmd.Flags().IntVar(&registryPriority, "priority", 1, "Registry priority (lower number = higher priority)")
	registryAddCmd.Flags().BoolVar(&allowPrivateIp, "allow-private-ip", false, "Allow private IP addresses in URL")

	registryUpdateCmd.Flags().StringVar(&registryName, "name", "", "Update registry name")
	registryUpdateCmd.Flags().IntVar(&registryPriority, "priority", 0, "Update registry priority")
	registryUpdateCmd.Flags().StringVar(&registryURL, "url", "", "Update registry URL")
	registryUpdateCmd.Flags().StringVar(&registryPath, "path", "", "Update registry file path")
	registryUpdateCmd.Flags().BoolVar(&allowPrivateIp, "allow-private-ip", false, "Allow private IP addresses in URL")
}

func registryListCmdFunc(_ *cobra.Command, _ []string) error {
	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	var servers []*registry.ImageMetadata
	var serversWithSource []*registry.ServerWithSource

	// Handle different listing modes
	if sourceRegistry != "" {
		// List from specific registry
		registryProvider, err := manager.GetRegistry(sourceRegistry)
		if err != nil {
			return fmt.Errorf("registry '%s' not found: %v", sourceRegistry, err)
		}
		servers, err = registryProvider.ListServers()
		if err != nil {
			return fmt.Errorf("failed to list servers from registry '%s': %v", sourceRegistry, err)
		}
	} else if allSources {
		// List from all registries with source information
		serversWithSource, err = manager.ListServersWithSource()
		if err != nil {
			return fmt.Errorf("failed to list servers with sources: %v", err)
		}
	} else {
		// Default behavior - list from all enabled registries (priority-based)
		servers, err = manager.ListServers()
		if err != nil {
			return fmt.Errorf("failed to list servers: %v", err)
		}
	}

	// Sort servers by name
	if servers != nil {
		sort.Slice(servers, func(i, j int) bool {
			return servers[i].Name < servers[j].Name
		})
	} else if serversWithSource != nil {
		sort.Slice(serversWithSource, func(i, j int) bool {
			return serversWithSource[i].Name < serversWithSource[j].Name
		})
	}

	// Output based on format
	switch registryFormat {
	case FormatJSON:
		if serversWithSource != nil {
			return printJSONServersWithSource(serversWithSource)
		}
		return printJSONServers(servers)
	default:
		if serversWithSource != nil || showSource {
			if serversWithSource == nil {
				// Convert servers to serversWithSource for display
				serversWithSource = make([]*registry.ServerWithSource, len(servers))
				for i, server := range servers {
					serversWithSource[i] = &registry.ServerWithSource{
						ImageMetadata: server,
						RegistryID:    "unknown",
						RegistryName:  "Unknown",
						Priority:      0,
					}
				}
			}
			printTextServersWithSource(serversWithSource)
		} else {
			printTextServers(servers)
		}
		return nil
	}
}

func registryInfoCmdFunc(_ *cobra.Command, args []string) error {
	serverName := args[0]
	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	var server *registry.ImageMetadata

	if sourceRegistry != "" {
		// Get server from specific registry
		registryProvider, err := manager.GetRegistry(sourceRegistry)
		if err != nil {
			return fmt.Errorf("registry '%s' not found: %v", sourceRegistry, err)
		}
		server, err = registryProvider.GetServer(serverName)
		if err != nil {
			return fmt.Errorf("failed to get server '%s' from registry '%s': %v", serverName, sourceRegistry, err)
		}
	} else {
		// Default behavior - search across all enabled registries
		server, err = manager.GetServer(serverName)
		if err != nil {
			return fmt.Errorf("failed to get server information: %v", err)
		}
	}

	// Output based on format
	switch registryFormat {
	case FormatJSON:
		return printJSONServer(server)
	default:
		printTextServerInfo(serverName, server)
		return nil
	}
}

// printJSONServers prints servers in JSON format
func printJSONServers(servers []*registry.ImageMetadata) error {
	// Marshal to JSON
	jsonData, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Print JSON
	fmt.Println(string(jsonData))
	return nil
}

// printJSONServer prints a single server in JSON format
func printJSONServer(server *registry.ImageMetadata) error {
	// Marshal to JSON
	jsonData, err := json.MarshalIndent(server, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Print JSON
	fmt.Println(string(jsonData))
	return nil
}

// printTextServers prints servers in text format
func printTextServers(servers []*registry.ImageMetadata) {
	// Create a tabwriter for pretty output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tTIER\tSTARS\tPULLS")

	// Print server information
	for _, server := range servers {
		stars := 0
		pulls := 0
		if server.Metadata != nil {
			stars = server.Metadata.Stars
			pulls = server.Metadata.Pulls
		}

		desc := server.Description
		if server.Status == "Deprecated" {
			desc = "**DEPRECATED** " + desc
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
			server.Name,
			truncateString(desc, 60),
			server.Tier,
			stars,
			pulls,
		)
	}

	// Flush the tabwriter
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to flush tabwriter: %v\n", err)
	}
}

// printTextServerInfo prints detailed information about a server in text format
// nolint:gocyclo
func printTextServerInfo(name string, server *registry.ImageMetadata) {
	fmt.Printf("Name: %s\n", server.Name)
	fmt.Printf("Image: %s\n", server.Image)
	fmt.Printf("Description: %s\n", server.Description)
	fmt.Printf("Tier: %s\n", server.Tier)
	fmt.Printf("Status: %s\n", server.Status)
	fmt.Printf("Transport: %s\n", server.Transport)
	if (server.Transport == "sse" || server.Transport == "streamable-http") && server.TargetPort > 0 {
		fmt.Printf("Target Port: %d\n", server.TargetPort)
	}
	fmt.Printf("Repository URL: %s\n", server.RepositoryURL)
	fmt.Printf("Has Provenance: %s\n", map[bool]string{true: "Yes", false: "No"}[server.Provenance != nil])

	if server.Metadata != nil {
		fmt.Printf("Popularity: %d stars, %d pulls\n", server.Metadata.Stars, server.Metadata.Pulls)
		fmt.Printf("Last Updated: %s\n", server.Metadata.LastUpdated)
	} else {
		fmt.Printf("Popularity: 0 stars, 0 pulls\n")
		fmt.Printf("Last Updated: N/A\n")
	}

	// Print tools
	if len(server.Tools) > 0 {
		fmt.Println("Tools:")
		for _, tool := range server.Tools {
			fmt.Printf("  - %s\n", tool)
		}
	}

	// Print environment variables
	if len(server.EnvVars) > 0 {
		fmt.Println("\nEnvironment Variables:")
		for _, envVar := range server.EnvVars {
			required := ""
			if envVar.Required {
				required = " (required)"
			}
			defaultValue := ""
			if envVar.Default != "" {
				defaultValue = fmt.Sprintf(" [default: %s]", envVar.Default)
			}
			fmt.Printf("  - %s%s%s: %s\n", envVar.Name, required, defaultValue, envVar.Description)
		}
	}

	// Print tags
	if len(server.Tags) > 0 {
		fmt.Println("Tags:")
		fmt.Printf("  %s\n", strings.Join(server.Tags, ", "))
	}

	// Print permissions
	if server.Permissions != nil {
		fmt.Println("Permissions:")

		// Print read permissions
		if len(server.Permissions.Read) > 0 {
			fmt.Println("  Read:")
			for _, path := range server.Permissions.Read {
				fmt.Printf("    - %s\n", path)
			}
		}

		// Print write permissions
		if len(server.Permissions.Write) > 0 {
			fmt.Println("  Write:")
			for _, path := range server.Permissions.Write {
				fmt.Printf("    - %s\n", path)
			}
		}

		// Print network permissions
		if server.Permissions.Network != nil && server.Permissions.Network.Outbound != nil {
			fmt.Println("  Network:")
			outbound := server.Permissions.Network.Outbound

			if outbound.InsecureAllowAll {
				fmt.Println("    Insecure Allow All: true")
			}

			if len(outbound.AllowHost) > 0 {
				fmt.Printf("    Allow Host: %s\n", strings.Join(outbound.AllowHost, ", "))
			}

			if len(outbound.AllowPort) > 0 {
				ports := make([]string, len(outbound.AllowPort))
				for i, port := range outbound.AllowPort {
					ports[i] = fmt.Sprintf("%d", port)
				}
				fmt.Printf("    Allow Port: %s\n", strings.Join(ports, ", "))
			}
		}
	}

	// Print example command
	fmt.Println("Example Command:")
	fmt.Printf("  thv run %s\n", name)
}

// truncateString truncates a string to the specified length and adds "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// registrySourcesCmdFunc lists all configured registry sources
func registrySourcesCmdFunc(_ *cobra.Command, _ []string) error {
	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	registryInfos := manager.ListRegistryInfo()

	switch registryFormat {
	case FormatJSON:
		return printJSONRegistryInfos(registryInfos)
	default:
		printTextRegistryInfos(registryInfos)
		return nil
	}
}

// registryAddCmdFunc adds a new registry source
func registryAddCmdFunc(_ *cobra.Command, args []string) error {
	id := args[0]
	urlOrPath := args[1]

	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	// Detect registry type and validate input
	registryType, cleanPath := detectRegistryType(urlOrPath)
	
	registryConfig := registry.RegistryConfig{
		ID:       id,
		Name:     registryName,
		Priority: registryPriority,
		Enabled:  true,
	}

	// Set name to ID if not provided
	if registryConfig.Name == "" {
		registryConfig.Name = id
	}

	switch registryType {
	case "url":
		registryConfig.Type = registry.RegistryTypeRemote
		registryConfig.URL = cleanPath
		registryConfig.AllowPrivateIp = allowPrivateIp
	case "file":
		registryConfig.Type = registry.RegistryTypeLocal
		registryConfig.Path = cleanPath
	default:
		return fmt.Errorf("unsupported registry type")
	}

	// Add registry to manager
	if err := manager.AddRegistry(registryConfig); err != nil {
		return fmt.Errorf("failed to add registry: %v", err)
	}

	// Save configuration
	if err := manager.SaveToConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("Successfully added registry '%s' (%s)\n", id, registryConfig.Name)
	return nil
}

// registryRemoveCmdFunc removes a registry source
func registryRemoveCmdFunc(_ *cobra.Command, args []string) error {
	id := args[0]

	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	// Remove registry
	if err := manager.RemoveRegistry(id); err != nil {
		return fmt.Errorf("failed to remove registry: %v", err)
	}

	// Save configuration
	if err := manager.SaveToConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("Successfully removed registry '%s'\n", id)
	return nil
}

// registryEnableCmdFunc enables a registry source
func registryEnableCmdFunc(_ *cobra.Command, args []string) error {
	id := args[0]

	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	if err := manager.EnableRegistry(id); err != nil {
		return fmt.Errorf("failed to enable registry: %v", err)
	}

	if err := manager.SaveToConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("Successfully enabled registry '%s'\n", id)
	return nil
}

// registryDisableCmdFunc disables a registry source  
func registryDisableCmdFunc(_ *cobra.Command, args []string) error {
	id := args[0]

	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	if err := manager.DisableRegistry(id); err != nil {
		return fmt.Errorf("failed to disable registry: %v", err)
	}

	if err := manager.SaveToConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("Successfully disabled registry '%s'\n", id)
	return nil
}

// registrySetDefaultCmdFunc sets the default registry source
func registrySetDefaultCmdFunc(_ *cobra.Command, args []string) error {
	id := args[0]

	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	if err := manager.SetDefaultRegistry(id); err != nil {
		return fmt.Errorf("failed to set default registry: %v", err)
	}

	if err := manager.SaveToConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("Successfully set '%s' as default registry\n", id)
	return nil
}

// registryUpdateCmdFunc updates a registry source
func registryUpdateCmdFunc(_ *cobra.Command, args []string) error {
	id := args[0]

	manager, err := registry.GetDefaultManager()
	if err != nil {
		return fmt.Errorf("failed to get registry manager: %v", err)
	}

	// Verify registry exists
	_, err = manager.GetRegistry(id)
	if err != nil {
		return fmt.Errorf("registry '%s' not found: %v", id, err)
	}

	// Get registry info to build update config
	registryInfos := manager.ListRegistryInfo()
	var currentConfig registry.RegistryConfig
	found := false
	for _, info := range registryInfos {
		if info.ID == id {
			currentConfig = info.RegistryConfig
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("registry '%s' not found in configuration", id)
	}

	// Update fields that were provided
	updateConfig := currentConfig
	updated := false

	if registryName != "" {
		updateConfig.Name = registryName
		updated = true
	}

	if registryPriority > 0 {
		updateConfig.Priority = registryPriority
		updated = true
	}

	if registryURL != "" {
		updateConfig.Type = registry.RegistryTypeRemote
		updateConfig.URL = registryURL
		updateConfig.Path = ""
		updateConfig.AllowPrivateIp = allowPrivateIp
		updated = true
	}

	if registryPath != "" {
		updateConfig.Type = registry.RegistryTypeLocal
		updateConfig.Path = registryPath
		updateConfig.URL = ""
		updated = true
	}

	if !updated {
		return fmt.Errorf("no update parameters provided")
	}

	// Update registry
	if err := manager.UpdateRegistry(id, updateConfig); err != nil {
		return fmt.Errorf("failed to update registry: %v", err)
	}

	// Save configuration
	if err := manager.SaveToConfig(); err != nil {
		return fmt.Errorf("failed to save configuration: %v", err)
	}

	fmt.Printf("Successfully updated registry '%s'\n", id)
	return nil
}

// Helper functions for output formatting
func printJSONRegistryInfos(registryInfos []registry.RegistryInfo) error {
	jsonData, err := json.MarshalIndent(registryInfos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

func printTextRegistryInfos(registryInfos []registry.RegistryInfo) {
	if len(registryInfos) == 0 {
		fmt.Println("No registries configured")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tSTATUS\tSERVERS\tPRIORITY")

	for _, info := range registryInfos {
		status := "●"
		if !info.Enabled {
			status = "○"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\n",
			info.ID,
			info.Name,
			info.Type,
			status,
			info.ServerCount,
			info.Priority,
		)
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to flush tabwriter: %v\n", err)
	}
}

// printJSONServersWithSource prints servers with source information in JSON format
func printJSONServersWithSource(servers []*registry.ServerWithSource) error {
	jsonData, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}
	fmt.Println(string(jsonData))
	return nil
}

// printTextServersWithSource prints servers with source information in text format
func printTextServersWithSource(servers []*registry.ServerWithSource) {
	if len(servers) == 0 {
		fmt.Println("No servers found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tTIER\tSTARS\tPULLS\tSOURCE")

	for _, server := range servers {
		stars := 0
		pulls := 0
		if server.Metadata != nil {
			stars = server.Metadata.Stars
			pulls = server.Metadata.Pulls
		}

		desc := server.Description
		if server.Status == "Deprecated" {
			desc = "**DEPRECATED** " + desc
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
			server.Name,
			truncateString(desc, 60),
			server.Tier,
			stars,
			pulls,
			server.RegistryName,
		)
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to flush tabwriter: %v\n", err)
	}
}

