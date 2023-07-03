package exporters

import (
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud/openstack/baremetal/apiversions"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/extendedserverattributes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/prometheus/client_golang/prometheus"
)

var defaultNovaWithTeamMetrics = []Metric{
	{Name: "flavors", Fn: ListFlavors},
	{Name: "flavor", Labels: []string{"id", "name", "vcpus", "ram", "disk", "is_public"}},
	{Name: "availability_zones", Fn: ListAZs},
	{Name: "security_groups", Fn: ListComputeSecGroups},
	{Name: "total_vms", Fn: NovaTeamListAllServers},
	{Name: "agent_state", Labels: []string{"id", "hostname", "service", "adminState", "zone", "disabledReason"}, Fn: ListNovaAgentState},
	{Name: "running_vms", Labels: []string{"hostname", "availability_zone", "aggregates"}, Fn: ListHypervisors},
	{Name: "current_workload", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "vcpus_available", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "vcpus_used", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "memory_available_bytes", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "memory_used_bytes", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "local_storage_available_bytes", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "local_storage_used_bytes", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "free_disk_bytes", Labels: []string{"hostname", "availability_zone", "aggregates"}},
	{Name: "server_status", Labels: []string{"id", "status", "name", "tenant_id", "user_id", "address_ipv4",
		"address_ipv6", "host_id", "hypervisor_hostname", "uuid", "availability_zone", "flavor_id", "team"}},
	{Name: "limits_vcpus_max", Labels: []string{"tenant", "tenant_id"}, Fn: ListComputeLimits, Slow: true},
	{Name: "limits_vcpus_used", Labels: []string{"tenant", "tenant_id"}, Slow: true},
	{Name: "limits_memory_max", Labels: []string{"tenant", "tenant_id"}, Slow: true},
	{Name: "limits_memory_used", Labels: []string{"tenant", "tenant_id"}, Slow: true},
	{Name: "limits_instances_used", Labels: []string{"tenant", "tenant_id"}, Slow: true},
	{Name: "limits_instances_max", Labels: []string{"tenant", "tenant_id"}, Slow: true},
	{Name: "server_local_gb", Labels: []string{"name", "id", "tenant_id"}, Fn: ListUsage, Slow: true},
}

func NewNovaTeamExporter(config *ExporterConfig) (*NovaExporter, error) {
	exporter := NovaExporter{
		BaseOpenStackExporter{
			Name:           "nova",
			ExporterConfig: *config,
		},
	}
	for _, metric := range defaultNovaWithTeamMetrics {
		if exporter.isDeprecatedMetric(&metric) {
			continue
		}
		if !exporter.isSlowMetric(&metric) {
			exporter.AddMetric(metric.Name, metric.Fn, metric.Labels, metric.DeprecatedVersion, nil)
		}
	}

	envMicroversion, present := os.LookupEnv("OS_COMPUTE_API_VERSION")
	if present {
		exporter.Client.Microversion = envMicroversion
	} else {

		microversion, err := apiversions.Get(config.Client, "v2.1").Extract()
		if err == nil {
			exporter.Client.Microversion = microversion.Version
		}
	}

	return &exporter, nil
}

// NovaTeamListAllServers is copy of ListAllServers in nova.go + team label
func NovaTeamListAllServers(exporter *BaseOpenStackExporter, ch chan<- prometheus.Metric) error {
	type ServerWithExt struct {
		servers.Server
		availabilityzones.ServerAvailabilityZoneExt
		extendedserverattributes.ServerAttributesExt
	}

	var allServers []ServerWithExt

	allPagesServers, err := servers.List(exporter.Client, servers.ListOpts{AllTenants: true}).AllPages()
	if err != nil {
		return err
	}

	err = servers.ExtractServersInto(allPagesServers, &allServers)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(exporter.Metrics["total_vms"].Metric,
		prometheus.GaugeValue, float64(len(allServers)))

	// Server status metrics
	for _, server := range allServers {
		ch <- prometheus.MustNewConstMetric(exporter.Metrics["server_status"].Metric,
			prometheus.GaugeValue, float64(mapServerStatus(server.Status)), server.ID, server.Status, server.Name, server.TenantID,
			server.UserID, server.AccessIPv4, server.AccessIPv6, server.HostID, server.HypervisorHostname, server.ID, server.AvailabilityZone, fmt.Sprintf("%v", server.Flavor["id"]), getTeam(server.TenantID))
	}

	return nil
}
