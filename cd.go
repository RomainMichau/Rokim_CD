package main

import (
	"context"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
	"strings"
	"time"
)

type Flags struct {
	kubeconfig      string
	confFile        string
	githubToken     string
	intervalSeconds int64
}

func ParseFlags() Flags {
	kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig file")
	confFile := flag.String("conf", "", "path of the conf file")
	intervalSeconds := flag.Int64("interval-seconds", 60, "Interval between each check in seconds")
	githubToken := flag.String("token", "", "github token")
	flag.Parse()
	if *confFile == "" {
		if os.Getenv("ROKIM_CD_CONF_FILE") != "" {
			*confFile = os.Getenv("ROKIM_CD_CONF_FILE")
		} else {
			*confFile = "/etc/rokim_cd/config.yaml"
		}
	}
	if *githubToken == "" {
		if os.Getenv("GITHUB_TOKEN") != "" {
			*githubToken = os.Getenv("GITHUB_TOKEN")

		} else {
			log.Fatal("token flag is required")
		}
	}
	log.Infof("conf file: %s", *confFile)
	log.Infof("Run interval: %d seconds", *intervalSeconds)
	if *kubeconfig == "" {
		log.Infof("--kubeconfig flag undefine, will use in cluster kube config")
	}
	return Flags{
		kubeconfig:      *kubeconfig,
		confFile:        *confFile,
		githubToken:     *githubToken,
		intervalSeconds: *intervalSeconds,
	}
}

func isSuccessStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func getUpstreamImageDigest(appConf AppConf, token string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/packages/container/%s/versions", appConf.Username, appConf.ImageName)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating HTTP request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Go-HTTP-Client")

	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Error sending HTTP request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)

	if !isSuccessStatus(response.StatusCode) {
		body_str := string(body)
		return "", fmt.Errorf("Failed to get image manifest. Status code: %d Body: %s", response.StatusCode,
			body_str)
	}

	if err != nil {
		return "", fmt.Errorf("Error reading response body: %v", err)
	}
	var gitHubPackages []GitHubPackage
	err = yaml.Unmarshal(body, &gitHubPackages)
	for _, gitHubPackage := range gitHubPackages {
		for _, tag := range gitHubPackage.Metadata.Container.Tags {
			if tag == appConf.Tag {
				return gitHubPackage.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no image %s found on github", appConf.FullName())
}

func restartDeployment(deploymentName string, namespace string, clientset *kubernetes.Clientset) error {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error(fmt.Sprintf("Error getting deployment %s (namespace: %s)", deploymentName, namespace), err.Error())
		return err
	}

	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.ObjectMeta.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().Format(time.RFC3339)
	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		log.Error(fmt.Sprintf("Error restarting deployment %s (namespace: %s)", deploymentName, namespace), err.Error())
		return err
	}
	log.Infof("Deployment %s restarted", deploymentName)
	return nil
}

func deploymentIsUsingImageId(deploymentName string, namespace string, upstreamImageDigest string, clientset *kubernetes.Clientset) (bool, error) {
	// Get the deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error(fmt.Sprintf("Error getting deployment %s (namespace: %s)", deploymentName, namespace), err.Error())
		return false, err
	}

	labelSelector := fmt.Sprintf("%s=%s", "app.kubernetes.io/name", deployment.Spec.Selector.MatchLabels["app.kubernetes.io/name"])
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{

		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error(fmt.Sprintf("Error getting pods with label :%s (namespace: %s)", labelSelector, namespace), err.Error())
		return false, err
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Status.ContainerStatuses {
			split := strings.Split(container.ImageID, "@")
			if len(split) != 2 {
				log.Error(fmt.Sprintf("Error parsing image ID: %s", container.ImageID))
				return false, nil
			}
			digest := split[1]
			if digest == upstreamImageDigest {
				return true, nil
			}
		}
	}
	return false, nil
}

func readConfFile(confFilePath string) []AppConf {
	yamlFile, err := os.ReadFile(confFilePath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}
	var deployments []AppConf
	err = yaml.Unmarshal(yamlFile, &deployments)
	if err != nil {
		log.Fatalf("Error unmarshalling YAML: %v", err)
	}
	return deployments
}

func getKubernetesClient(confPath string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if confPath == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Error("Fail to load in clust config", err.Error())
			return nil, err
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", confPath)
		if err != nil {
			log.Error("Fail to load config", err.Error())
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("Error creating Kubernetes client:", err.Error())
		return nil, err
	}
	return clientset, nil
}

func main() {
	flags := ParseFlags()
	appConfs := readConfFile(flags.confFile)
	clientset, err := getKubernetesClient(flags.kubeconfig)
	if err != nil {
		log.Fatal("Error getting kubernetes client", err.Error())
	}
	for {
		redeploy := false
		for _, appConf := range appConfs {
			upstreamDigest, err := getUpstreamImageDigest(appConf, flags.githubToken)
			if err != nil {
				log.Errorf("Error fetching upstream ID for %s: %s", appConf.FullName(), err.Error())
				continue
			}
			isUsingLastImageVersion, err := deploymentIsUsingImageId(appConf.K8sDeployment, appConf.K8sNamespace, upstreamDigest, clientset)
			if err != nil {
				log.Error("Error checking if deployment is using image ", err.Error())
				continue
			}
			if isUsingLastImageVersion {
				log.Debugf("deployment %s is already using up to date image: %s", appConf.FullName(), upstreamDigest)
			} else {
				log.Infof("deployment %s is using old image: %s. Restarting...", appConf.FullName(), upstreamDigest)
				restartDeployment(appConf.K8sDeployment, appConf.K8sNamespace, clientset)
				redeploy = true
			}
		}
		if !redeploy {
			log.Info("No deployment to restart")
		}
		time.Sleep(time.Duration(flags.intervalSeconds) * time.Second)
	}
}
