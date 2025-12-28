# Tiltfile

# Helper to deploy helm charts
def deploy_helm(name, repo_name, repo_url, chart, namespace, values_files=[], version=None, set_args=[], set_string_args=[], resource_deps=[]):
  flags = []
  if version:
    flags.append("--version " + version)
  for v in values_files:
    flags.append("-f " + v)
  for s in set_args:
    flags.append("--set " + s)
  for s in set_string_args:
    flags.append("--set-string " + s)
  
  flag_str = " ".join(flags)
  
  # Command to add repo and upgrade/install
  cmd = """
  helm repo add {repo_name} {repo_url}
  helm repo update {repo_name}
  helm upgrade --install {name} {repo_name}/{chart} --namespace {namespace} --create-namespace {flags}
  """.format(
    repo_name=repo_name,
    repo_url=repo_url,
    name=name,
    chart=chart,
    namespace=namespace,
    flags=flag_str
  )
  
  local_resource(
    name,
    cmd=cmd,
    deps=values_files,
    resource_deps=resource_deps
  )

# =============================================================================
# 1. Infrastructure (Helm + Manifests)
# =============================================================================

# NGINX Ingress Controller
deploy_helm('nginx-ingress',
  repo_name='ingress-nginx',
  repo_url='https://kubernetes.github.io/ingress-nginx',
  chart='ingress-nginx',
  namespace='ingress-nginx',
  version='4.10.0',
  set_args=[
    'controller.hostPort.enabled=true',
    'controller.service.type=NodePort',
    'controller.watchIngressWithoutClass=true',
    'controller.admissionWebhooks.enabled=false',
    'controller.replicaCount=1',
  ],
  set_string_args=[
    'controller.nodeSelector.ingress-ready=true',
  ]
)

# Postgres Bids
deploy_helm('postgres-bids',
  repo_name='bitnami',
  repo_url='https://charts.bitnami.com/bitnami',
  chart='postgresql',
  namespace='default',
  values_files=['deploy/k8s/infra/values-postgres-bids.yaml']
)

# Postgres Stats
deploy_helm('postgres-stats',
  repo_name='bitnami',
  repo_url='https://charts.bitnami.com/bitnami',
  chart='postgresql',
  namespace='default',
  values_files=['deploy/k8s/infra/values-postgres-stats.yaml']
)

# RabbitMQ (using direct manifest - Bitnami images have limited free availability since Aug 2025)
# See: https://github.com/bitnami/containers/issues/83267
k8s_yaml('deploy/k8s/infra/rabbitmq.yaml')
k8s_resource('rabbitmq', labels=['infra'])

# Redis
deploy_helm('redis',
  repo_name='bitnami',
  repo_url='https://charts.bitnami.com/bitnami',
  chart='redis',
  namespace='default',
  values_files=['deploy/k8s/infra/values-redis.yaml']
)

# =============================================================================
# 2. Application Services (Helm Charts)
# =============================================================================

# Bid Service
docker_build('bid-service',
  context='.',
  dockerfile='services/bid-service/Dockerfile',
  ignore=['frontend', 'docs']
)

bid_service_yaml = helm(
    './deploy/charts/bid-service',
    name='bid-service',
    values=['./deploy/charts/bid-service/values.yaml']
)
k8s_yaml(bid_service_yaml)

k8s_resource('bid-service-migrate', 
  labels=['migrations'], 
  resource_deps=['postgres-bids']
)
k8s_resource('bid-service-api', 
  labels=['app'], 
  resource_deps=['postgres-bids', 'rabbitmq', 'redis', 'bid-service-migrate']
)
k8s_resource('bid-service-worker', 
  labels=['app'], 
  resource_deps=['postgres-bids', 'rabbitmq', 'bid-service-migrate']
)


# User Stats Service
docker_build('user-stats-service',
  context='.',
  dockerfile='services/user-stats-service/Dockerfile',
  ignore=['frontend', 'docs']
)

user_stats_service_yaml = helm(
    './deploy/charts/user-stats-service',
    name='user-stats-service',
    values=['./deploy/charts/user-stats-service/values.yaml']
)
k8s_yaml(user_stats_service_yaml)

k8s_resource('user-stats-service-migrate', 
  labels=['migrations'], 
  resource_deps=['postgres-stats']
)
k8s_resource('user-stats-service-api', 
  labels=['app'], 
  resource_deps=['postgres-stats', 'rabbitmq', 'user-stats-service-migrate']
)
k8s_resource('user-stats-service-worker', 
  labels=['app'], 
  resource_deps=['postgres-stats', 'rabbitmq', 'user-stats-service-migrate']
)
