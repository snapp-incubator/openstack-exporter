package exporters

import (
	"crypto/tls"
	"net/http"
	"strings"
	"sync"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/prometheus/common/log"
)

// TeamSuffix is suffix of labels on tenenat e.g: test-team
const TeamSuffix string = "-team"

// tenant_id:teamName
var projectIDTeamMap = make(map[string]string)
var projectIDTeamMapMutex sync.RWMutex

// get team name from projectIDTeamMa
func getTeam(tenantId string) string {
	projectIDTeamMapMutex.RLock()
	teamName := projectIDTeamMap[tenantId]
	projectIDTeamMapMutex.RUnlock()
	return teamName
}

// UpdateProjectIDTeamMap job is get and set tenant_id:teamName to projectIDTeamMap
func UpdateProjectIDTeamMap() {
	extractTeamFromTags := func(tags []string) (tag string) {
		for _, t := range tags {
			if strings.HasSuffix(t, TeamSuffix) {
				return t
			}
		}
		return ""
	}

	allProjects, err := listAllProjects()
	if err == nil {
		ProjectsWithTeamTag := getProjectsWithTeamTag(allProjects)
		projectIDTeamMapMutex.Lock()
		go func() {
			for _, p := range ProjectsWithTeamTag {
				projectIDTeamMap[p.ID] = extractTeamFromTags(p.Tags)

			}
		}()
		projectIDTeamMapMutex.Unlock()
	}
}

// getProjectsWithTeamTag get all projects and return list of project that has -team label
func getProjectsWithTeamTag(projs []projects.Project) []projects.Project {
	projectsWtihTeamTag := make([]projects.Project, 0)
	isTagsContainTeam := func(tags []string) bool {
		for _, tag := range tags {
			if strings.HasSuffix(tag, TeamSuffix) {
				return true
			}
		}
		return false
	}

	for _, p := range projs {
		if isTagsContainTeam(p.Tags) {
			projectsWtihTeamTag = append(projectsWtihTeamTag, p)
		}
	}
	return projectsWtihTeamTag
}

// listAllProject return all projects that get from openstack.
func listAllProjects() ([]projects.Project, error) {

	var allProjects []projects.Project

	allPagesProject, err := projects.List(TeamServiceClient, projects.ListOpts{}).AllPages()
	if err != nil {
		log.Errorf("could not get projects: %s", err)
		return nil, err
	}

	allProjects, err = projects.ExtractProjects(allPagesProject)
	if err != nil {
		log.Errorf("projects Extrcat failed: %s", err)
		return nil, err
	}
	return allProjects, nil

}

var TeamServiceClient *gophercloud.ServiceClient

// NewTeamServiceClient creae and return a keystone client
func NewTeamServiceClient(cloud string, endpointType string) (*gophercloud.ServiceClient, error) {
	clientName := "identity"
	var err error
	var transport *http.Transport

	opts := clientconfig.ClientOpts{Cloud: cloud}

	config, err := clientconfig.GetCloudFromYAML(&opts)
	if err != nil {
		return nil, err
	}

	if !*config.Verify {
		log.Infoln("SSL verification disabled on transport")
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	client, err := NewServiceClient(clientName, &opts, transport, endpointType)
	if err != nil {
		return nil, err
	}
	return client, nil
}
