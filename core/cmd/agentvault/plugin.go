package main

import (
	"fmt"
	"sort"

	"github.com/agentvault/core/internal/plugins"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage AgentVault plugins",
	Long:  `Install, enable, disable, and list MCP-based plugins for AgentVault.`,
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <path>",
	Short: "Install a plugin from a directory",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginInstall,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE:  runPluginList,
}

var pluginEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginEnable,
}

var pluginDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginDisable,
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginEnableCmd)
	pluginCmd.AddCommand(pluginDisableCmd)
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()
	if err := plugins.Install(vp, args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin installed from %s\n", args[0])
	return nil
}

func runPluginList(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()
	all, err := plugins.Discover(vp)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		fmt.Println("No plugins installed.")
		return nil
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Manifest.Name < all[j].Manifest.Name
	})

	for _, p := range all {
		status := "disabled"
		if p.Manifest.Enabled {
			status = "enabled"
		}
		fmt.Printf("  %-20s %-8s v%s — %s\n", p.Manifest.Name, status, p.Manifest.Version, p.Manifest.Description)
	}
	return nil
}

func runPluginEnable(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()
	if err := plugins.Enable(vp, args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin %q enabled.\n", args[0])
	return nil
}

func runPluginDisable(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()
	if err := plugins.Disable(vp, args[0]); err != nil {
		return err
	}
	fmt.Printf("Plugin %q disabled.\n", args[0])
	return nil
}
