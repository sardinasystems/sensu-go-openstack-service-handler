package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	os_services "github.com/gophercloud/gophercloud/v2/openstack/compute/v2/services"
	"github.com/gophercloud/gophercloud/v2/openstack/config"
	clouds "github.com/gophercloud/gophercloud/v2/openstack/config/clouds"
	oscli "github.com/gophercloud/utils/v2/client"
	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the handler plugin config.
type Config struct {
	sensu.PluginConfig
	Cloud      string
	CloudsFile string
	Service    string
	Binary     string
	Host       string
	ID         string
	Debug      bool
}

const (
	computeMicroversion = "2.79" // Train and newer

	serviceDisabled = os_services.ServiceDisabled
	serviceEnabled  = os_services.ServiceEnabled
)

var (
	// supportedServices = []string{"compute", "volume", "sharev2", "network", "orchestration", "container"}
	supportedServices = []string{"compute"}

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
		&sensu.PluginConfigOption[string]{
			Path:      "binary",
			Argument:  "binary",
			Shorthand: "b",
			Default:   "nova-compute", // TODO(vermakov): custom default for each service
			Usage:     "service binary to search",
			Value:     &plugin.Binary,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "host",
			Argument:  "host",
			Shorthand: "H",
			Default:   "",
			Value:     &plugin.Host,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "id",
			Argument: "id",
			Value:    &plugin.ID,
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
	return nil
}

func executeHandler(event *corev2.Event) error {
	ctx, cf := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cf()

	var httpCli *http.Client
	if plugin.Debug {
		httpCli = &http.Client{
			Transport: &oscli.RoundTripper{
				Rt:     &http.Transport{},
				Logger: &oscli.DefaultLogger{},
			},
		}
	} else {
		httpCli = &http.Client{Transport: &http.Transport{}}
	}

	pOpts := []clouds.ParseOption{clouds.WithCloudName(plugin.Cloud)}
	if plugin.CloudsFile != "" {
		pOpts = append(pOpts, clouds.WithLocations(plugin.CloudsFile))
	}

	ao, eo, tlsCfg, err := clouds.Parse(pOpts...)
	if err != nil {
		return err
	}

	// check never need to reauth
	ao.AllowReauth = false

	pc, err := config.NewProviderClient(ctx, ao, config.WithHTTPClient(*httpCli), config.WithTLSConfig(tlsCfg))
	if err != nil {
		return err
	}

	switch plugin.Service {
	case "compute":
		return handleCompute(ctx, pc, eo, event)

	// case "volume":
	// 	return handleVolume(cli)
	//
	// case "sharev2":
	// 	return handleShare(cli)
	//
	// case "network":
	// 	return handleNetwork(cli)
	//
	// case "orchestration":
	// 	return handleOrchestration(cli)
	//
	// case "container":
	// 	return handleContainer(cli)

	default:
		return fmt.Errorf("unsupported service: %s", plugin.Service)
	}
}

func handleCompute(ctx context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts, event *corev2.Event) error {
	_ = ctx

	cli, err := openstack.NewComputeV2(pc, eo)
	if err != nil {
		return err
	}
	cli.Microversion = computeMicroversion

	host := plugin.Host
	if host == "" {
		host = event.GetEntity().GetName()
	}

	computeID := plugin.ID
	if computeID == "" {
		log.Printf("Searching ID for host: %s, binary: %s", host, plugin.Binary)

		query := &os_services.ListOpts{
			Binary: plugin.Binary,
			Host:   host,
		}
		pages, err := os_services.List(cli, query).AllPages(ctx)
		if err != nil {
			return fmt.Errorf("Compute services list error: %w", err)
		}

		services, err := os_services.ExtractServices(pages)
		if err != nil {
			return err
		}

		if len(services) == 0 {
			return fmt.Errorf("Service not found")
		}

		computeID = services[0].ID
		log.Printf("Found service ID: %s", computeID)
	}

	updateOpts := os_services.UpdateOpts{}
	status := event.GetCheck().GetStatus()

	if status == sensu.CheckStateOK {
		updateOpts.Status = serviceEnabled
	} else {
		updateOpts.Status = serviceDisabled
		updateOpts.DisabledReason = fmt.Sprintf("Disabled by Health action, because %s", sensu.EventSummaryWithTrim(event, 100))
	}

	log.Printf("Apply action: %s", updateOpts.Status)
	_, err = os_services.Update(ctx, cli, computeID, updateOpts).Extract()
	if err != nil {
		return fmt.Errorf("Compute service id: %s update error: %w", computeID, err)
	}

	return nil
}
