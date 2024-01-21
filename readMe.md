# RokimCD

RockimCD is a lightweight CD tool for kubernetes.
RokimCD ensure that deployements running on kubernetes are always up to date
with the latest version on the github container registry

# How it works

The app read a configuration defining the list of deployments to watch.  
For each deployment, the app will check the latest version of the image on the github container registry.  
If the image is not up to date, the app will restart the deployment with the latest version.

# CLI Parameters

| Parameter            | Description                                                                                                               | Default                   |
|----------------------|---------------------------------------------------------------------------------------------------------------------------|---------------------------|
| `--kubeconfig`       | Path of the local kube config file.<br/> If empty the application will consider that it is running on the cluster itself. | ""                        |
| `--token`            | Github token. Required read-package permission                                                                            | Env var $GITHUB_TOKEN     |
| `--interval-seconds` | Interval between two checks in seconds                                                                                    | 60                        |
| `--conf`             | Path of the configuration. If empty will use $ROKIM_CD_CONF_FILE                                                          | /etc/rokim_cd/config.yaml |

# Configuration file

Example:

```yaml
---
# Image romainmichau/hugo_blog:latest
- image_name: hugo_blog
  tag: latest
  username: romainmichau
  k8s_namespace: rokim-apps
  k8s_deployment: hugo-blog-draft

# Same image but different deployment
- image_name: hugo_blog
  tag: latest
  username: romainmichau
  k8s_namespace: rokim-apps
  k8s_deployment: hugo-blog-prod

- image_name: connect4_ui
  tag: latest
  username: romainmichau
  k8s_namespace: rokim-apps
  k8s_deployment: connect4-ui
```

# Fields:

| Field            | Description                                        |
|------------------|----------------------------------------------------|
| `image_name`     | Name of the image on the github container registry |
| `tag`            | Tag of the image on the github container registry  |
| `username`       | Github username                                    |
| `k8s_namespace`  | Kubernetes namespace to watch                      |
| `k8s_deployment` | Kubernetes deployment to watch                     |