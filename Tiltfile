# Tiltfile

# Helper to deploy helm charts
def deploy_helm(name, repo_name, repo_url, chart, namespace, values_files=[], version=None, set_args=[], set_string_args=[], resource_deps=[], labels=[]):
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
    resource_deps=resource_deps,
    labels=labels
  )

# =============================================================================
# 0. Secrets & Keys
# =============================================================================

local_resource(
  'generate_keys',
  cmd='scripts/generate-dev-keys.sh',
  deps=['scripts/generate-dev-keys.sh'],
  labels=['dev-only']
)

# Create secret if it doesn't exist.
# We use a shell command to check existence to avoid erroring if it already exists.
local_resource(
  'create_auth_keys_secret',
  cmd='kubectl get secret auth-keys >/dev/null 2>&1 || kubectl create secret generic auth-keys --from-file=private.pem=.data/keys/private.pem --from-file=public.pem=.data/keys/public.pem',
  resource_deps=['generate_keys'],
  labels=['dev-only']
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
  ],
  labels=['infra']
)

# Postgres Auth
deploy_helm('postgres-auth',
  repo_name='bitnami',
  repo_url='https://charts.bitnami.com/bitnami',
  chart='postgresql',
  namespace='default',
  values_files=['deploy/k8s/infra/values-postgres-auth.yaml'],
  labels=['db']
)

# Postgres Bids
deploy_helm('postgres-bids',
  repo_name='bitnami',
  repo_url='https://charts.bitnami.com/bitnami',
  chart='postgresql',
  namespace='default',
  values_files=['deploy/k8s/infra/values-postgres-bids.yaml'],
  labels=['db']
)

# Postgres Stats
deploy_helm('postgres-stats',
  repo_name='bitnami',
  repo_url='https://charts.bitnami.com/bitnami',
  chart='postgresql',
  namespace='default',
  values_files=['deploy/k8s/infra/values-postgres-stats.yaml'],
  labels=['db']
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
  values_files=['deploy/k8s/infra/values-redis.yaml'],
  labels=['infra']
)

# =============================================================================
# 2. Application Services (Helm Charts)
# =============================================================================

# Auth Service
docker_build('auth-service',
  context='.',
  dockerfile='services/auth-service/Dockerfile',
  ignore=['frontend', 'docs']
)

auth_service_yaml = helm(
    './deploy/charts/auth-service',
    name='auth-service',
    values=['./deploy/charts/auth-service/values.yaml']
)
k8s_yaml(auth_service_yaml)

k8s_resource('auth-service-migrate', 
  labels=['jobs'], 
  resource_deps=['postgres-auth']
)
k8s_resource('auth-service-api', 
  labels=['app'], 
  resource_deps=['postgres-auth', 'rabbitmq', 'auth-service-migrate', 'create_auth_keys_secret']
)

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
  labels=['jobs'], 
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
  labels=['jobs'], 
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


# Frontend BFF (Next.js)
docker_build('frontend',
  context='frontend',
  dockerfile='frontend/Dockerfile'
)

frontend_v2_yaml = helm(
    './deploy/charts/frontend',
    name='frontend',
    values=['./deploy/charts/frontend/values.yaml']
)
k8s_yaml(frontend_v2_yaml)

k8s_resource('frontend',
  labels=['frontend'],
  resource_deps=['nginx-ingress', 'create_auth_keys_secret']
)

# =============================================================================
# DEV ONLY: Database Down Migrations
# =============================================================================
# Change these versions when you need to migrate down to a specific version.
# Then trigger the corresponding resource in Tilt UI.
DB_DOWN_VERSION_AUTH = 0
DB_DOWN_VERSION_BIDS = 0
DB_DOWN_VERSION_STATS = 0

# Reset ALL databases to version 0
local_resource(
  'db-reset-all',
  cmd='''
    set -e
    echo "=== Resetting auth-service DB ===" && \
    kubectl exec sts/postgres-auth-postgresql -- sh -c "export PGPASSWORD=password && psql -U user -d auth_db -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'" && \
    echo "=== Resetting bid-service DB ===" && \
    kubectl exec sts/postgres-bids-postgresql -- sh -c "export PGPASSWORD=password && psql -U user -d bid_db -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'" && \
    echo "=== Resetting user-stats-service DB ===" && \
    kubectl exec sts/postgres-stats-postgresql -- sh -c "export PGPASSWORD=password && psql -U user -d stats_db -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'" && \
    echo "=== All databases reset to version 0. Run 'tilt trigger' on migration jobs to re-apply. ==="
  ''',
  auto_init=False,
  trigger_mode=TRIGGER_MODE_MANUAL,
  labels=['dev-only'],
  resource_deps=['postgres-auth', 'postgres-bids', 'postgres-stats']
)

# Down auth-service to specific version
local_resource(
  'db-down-auth',
  cmd='kubectl exec deploy/auth-service-api -- /app/goose -dir /app/migrations postgres "postgres://user:password@postgres-auth-postgresql:5432/auth_db?sslmode=disable" down-to ' + str(DB_DOWN_VERSION_AUTH),
  auto_init=False,
  trigger_mode=TRIGGER_MODE_MANUAL,
  labels=['dev-only'],
  resource_deps=['auth-service-api']
)

# Down bid-service to specific version
local_resource(
  'db-down-bids',
  cmd='kubectl exec deploy/bid-service-api -- /app/goose -dir /app/migrations postgres "postgres://user:password@postgres-bids-postgresql:5432/bid_db?sslmode=disable" down-to ' + str(DB_DOWN_VERSION_BIDS),
  auto_init=False,
  trigger_mode=TRIGGER_MODE_MANUAL,
  labels=['dev-only'],
  resource_deps=['bid-service-api']
)

# Down user-stats-service to specific version
local_resource(
  'db-down-stats',
  cmd='kubectl exec deploy/user-stats-service-api -- /app/goose -dir /app/migrations postgres "postgres://user:password@postgres-stats-postgresql:5432/stats_db?sslmode=disable" down-to ' + str(DB_DOWN_VERSION_STATS),
  auto_init=False,
  trigger_mode=TRIGGER_MODE_MANUAL,
  labels=['dev-only'],
  resource_deps=['user-stats-service-api']
)
