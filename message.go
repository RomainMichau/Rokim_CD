package main

import (
	"fmt"
	"time"
)

type AppConf struct {
	ImageName     string `yaml:"image_name"`
	Tag           string `yaml:"tag"`
	Username      string `yaml:"username"`
	K8sNamespace  string `yaml:"k8s_namespace"`
	K8sDeployment string `yaml:"k8s_deployment"`
}

type GitHubPackage struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	PackageHTMLURL string    `json:"package_html_url"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	HTMLURL        string    `json:"html_url"`
	Metadata       struct {
		PackageType string `json:"package_type"`
		Container   struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

func (g *AppConf) FullName() string {
	return fmt.Sprintf("%s/%s:%s", g.Username, g.ImageName, g.Tag)
}
