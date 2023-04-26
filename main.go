package main

import (
	"fmt"
	"log"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Cloud      string
	CloudsFile string
	Service    string
	Debug      bool
}

// const supportedServices = []string{"compute", "volume", "sharev2", "network", "orchestration", "container"}
const supportedServices = []string{"compute"}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-go-openstack-service-handler",
			Short:    "Plugin to change enabled state of the OpenStack service.",
			Keyspace: "sensu.io/plugins/sensu-go-openstack-service-handler/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "cloud",
			Env:       "OS_CLOUD",
			Argument:  "cloud",
			Shorthand: "c",
			Default:   "monitoring",
			Usage:     "Cloud used to access openstack API",
			Value:     &plugin.Cloud,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "os_config_file",
			Env:      "OS_CLIENT_CONFIG_FILE",
			Argument: "os-config-file",
			Default:  "",
			Usage:    "Clouds.yaml file path",
			Value:    &plugin.CloudsFile,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "service",
			Argument:  "service",
			Shorthand: "s",
			Default:   "compute",
			Allow:     supportedServices,
			Usage:     "Service to check",
			Value:     &plugin.Service,
		},
		&sensu.PluginConfigOption[bool]{
			Argument:  "debug",
			Shorthand: "d",
			Usage:     "Debug API calls",
			Value:     &plugin.Debug,
		},
	}
)

func main() {
	handler := sensu.NewGoHandler(&plugin.PluginConfig, options, checkArgs, executeHandler)
	handler.Execute()
}

func checkArgs(event *corev2.Event) error {
	if len(plugin.Example) == 0 {
		return fmt.Errorf("--example or HANDLER_EXAMPLE environment variable is required")
	}
	return nil
}

func executeHandler(event *corev2.Event) error {
	log.Println("executing handler with --example", plugin.Example)
	return nil
}
