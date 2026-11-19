#!/usr/bin/env bash
# Deploys the real template.yaml to floci via a genuine sam deploy, so the
# functional suite runs against a real API Gateway JWT authorizer (sam local
# start-api never emulated it - see internal/middleware.RequireAuthorizerIdentity).
# oidc-testkit-gen produces the signing key the suite mints tokens with, plus
# the discovery doc + JWKS published to floci S3 for the deployed authorizer.
set -euo pipefail

export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-2}"
# sam deploy honors this (botocore reads it); there is no --endpoint-url
# flag, which is all samlocal wraps.
export AWS_ENDPOINT_URL="${MYKEEBS_FLOCI_ENDPOINT:-http://localhost.floci.io:4566}"

STACK="${MYKEEBS_FLOCI_STACK:-mykeebs-api-floci}"
ENDPOINT="${MYKEEBS_FLOCI_ENDPOINT:-http://localhost.floci.io:4566}"
OIDC_BUCKET="mykeebs-api-floci-oidc"
# The issuer is just where the discovery doc + JWKS are published; the same
# string is threaded to --issuer, OidcIssuerBaseUrl, and MYKEEBS_OIDC_ISSUER.
# It must be https: floci's JWT authorizer refuses a plain-http issuer
# unless its host is a literal private IP. floci serves http and https on
# the same port, so only the issuer changes scheme.
ISSUER="${ENDPOINT/#http:/https:}/$OIDC_BUCKET"
AUDIENCE="client_local_mykeebs-api"
OIDC_TESTKIT_VERSION="v1.0.4"

# floci serves TLS with a certificate from our own CA, generated before floci
# starts so its trust store (docker-compose.floci.yml) can already hold the
# CA: floci's own CA doesn't exist until it has started. The trust store is
# the JDK's public roots plus our CA, because floci also builds the CA bundle
# it injects into Lambda containers from it. Reused across runs; delete the
# directory to regenerate.
CERT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.floci-certs"
if [[ ! -f "$CERT_DIR/truststore.p12" ]] ||
  ! openssl x509 -checkend 2592000 -noout -in "$CERT_DIR/server.leaf.crt" >/dev/null 2>&1; then
  rm -rf "$CERT_DIR"
  mkdir -p "$CERT_DIR"

  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
    -subj "/CN=mykeebs-api floci test CA" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign"

  # Go requires the name in the SAN, not the CN.
  openssl req -newkey rsa:2048 -nodes \
    -keyout "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -subj "/CN=localhost.floci.io"
  openssl x509 -req -in "$CERT_DIR/server.csr" -days 825 \
    -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
    -CAserial "$CERT_DIR/ca.srl" -CAcreateserial \
    -out "$CERT_DIR/server.leaf.crt" \
    -extfile <(printf '%s\n' \
      "subjectAltName=DNS:localhost.floci.io,DNS:*.localhost.floci.io,DNS:localhost,IP:127.0.0.1" \
      "basicConstraints=CA:FALSE" \
      "keyUsage=critical,digitalSignature,keyEncipherment" \
      "extendedKeyUsage=serverAuth")

  # Leaf then CA: floci serves this as the chain and appends it to the
  # container CA bundle.
  cat "$CERT_DIR/server.leaf.crt" "$CERT_DIR/ca.crt" > "$CERT_DIR/server.crt"

  # floci runs as uid 1001 and must read the key. Throwaway test material.
  chmod 644 "$CERT_DIR/server.key"

  docker run --rm --user "$(id -u):$(id -g)" -v "$CERT_DIR":/w eclipse-temurin:25-jre sh -c '
    keytool -importkeystore -noprompt \
      -srckeystore "$JAVA_HOME/lib/security/cacerts" -srcstorepass changeit \
      -destkeystore /w/truststore.p12 -deststoretype PKCS12 -deststorepass changeit &&
    keytool -importcert -noprompt -alias mykeebs-api-floci-ca -file /w/ca.crt \
      -keystore /w/truststore.p12 -storetype PKCS12 -storepass changeit' >/dev/null 2>&1 ||
    { echo "func-setup: could not build $CERT_DIR/truststore.p12" >&2; exit 1; }
fi

docker compose -f docker-compose.floci.yml up -d floci

for _ in $(seq 1 30); do
  curl -sf -o /dev/null "$ENDPOINT/_floci/health" && break
  sleep 1
done

sam build

# sam deploy --resolve-image-repos fabricates the standard AWS ECR hostname
# itself rather than reading floci's returned repositoryUri, so it always
# tries to push to a real AWS host. samconfig.toml/ci.yml already avoid
# --resolve-image-repos for the equivalent real-AWS reason - mirror that:
# create the repo explicitly and pass --image-repositories.
ECR_REPO="mykeebs-api-floci"
aws ecr create-repository --repository-name "$ECR_REPO" >/dev/null 2>&1 || true
REPO_URI=$(aws ecr describe-repositories \
  --repository-names "$ECR_REPO" --query 'repositories[0].repositoryUri' --output text)

OIDC_DIR="$(mktemp -d)"
KEY_PATH="$OIDC_DIR/signing-key.pem"
go run "github.com/roguekeyboards/oidc-testkit/cmd/oidc-testkit-gen@${OIDC_TESTKIT_VERSION}" \
  --issuer "$ISSUER" --out-dir "$OIDC_DIR" --key-out "$KEY_PATH" >/dev/null

aws s3api create-bucket --bucket "$OIDC_BUCKET" \
  --region "$AWS_DEFAULT_REGION" \
  --create-bucket-configuration LocationConstraint="$AWS_DEFAULT_REGION" \
  >/dev/null 2>&1 || true
aws s3 cp "$OIDC_DIR/openid-configuration" \
  "s3://$OIDC_BUCKET/.well-known/openid-configuration" \
  --content-type application/json >/dev/null
aws s3 cp "$OIDC_DIR/jwks.json" \
  "s3://$OIDC_BUCKET/jwks.json" \
  --content-type application/json >/dev/null

# template.yaml's roles need the stack boundary, and its distribution reads
# the image delivery stack's IDs from SSM.
scripts/bootstrap.sh floci >/dev/null

# floci serves a distribution's aliases on its own port, which the
# distribution's domain can't carry, so ImagesBaseUrl adds it. floci's ACM
# issues the alias's certificate at once, with no validation, and the test CA
# above already covers the name.
ENDPOINT_HOST=${ENDPOINT#*://}
IMAGES_DOMAIN="images.${ENDPOINT_HOST%%:*}"
IMAGES_CERT=$(aws acm list-certificates --region us-east-1 \
  --query "CertificateSummaryList[?DomainName=='$IMAGES_DOMAIN'].CertificateArn | [0]" --output text)
if [[ -z "$IMAGES_CERT" || "$IMAGES_CERT" == None ]]; then
  IMAGES_CERT=$(aws acm request-certificate --region us-east-1 \
    --domain-name "$IMAGES_DOMAIN" --validation-method DNS \
    --query CertificateArn --output text)
fi

# ApiReservedConcurrency is set only because floci can't deploy its empty
# default: it resolves the AWS::NoValue to "" and fails to parse it as an
# integer. Drop it once floci omits the property instead.
sam deploy \
  --stack-name "$STACK" \
  --no-confirm-changeset --no-fail-on-empty-changeset \
  --resolve-s3 \
  --image-repositories "ApiFunction=$REPO_URI" \
  --image-repositories "ModerateFunction=$REPO_URI" \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides \
    "OidcIssuerBaseUrl=$ISSUER" \
    "OidcAudience=$AUDIENCE" \
    "IdpConsentPublicToken=public-token-test-local-mykeebs-api" \
    "CorsAllowOrigins=http://localhost:5173" \
    "DailyUploadLimit=1000000" \
    "ImagesDomainName=$IMAGES_DOMAIN" \
    "ImagesDomainCertificateArn=$IMAGES_CERT" \
    "ImagesBaseUrl=https://images.$ENDPOINT_HOST" \
    "ApiReservedConcurrency=50"

out() {
  aws cloudformation describe-stacks --stack-name "$STACK" \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}

API_ID=$(aws apigatewayv2 get-apis --query 'Items[0].ApiId' --output text)

IMAGES_BUCKET=$(out ImagesBucketName)

# func-test.sh sources this directly, so `mise run func-test` picks up a
# fresh deploy's stack-derived values without any manual export step.
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env.floci"
cat <<ENVEOF > "$ENV_FILE"
export MYKEEBS_API_BASE_URL='$ENDPOINT/execute-api/$API_ID/\$default'
export MYKEEBS_DYNAMODB_ENDPOINT_URL=$ENDPOINT
export MYKEEBS_S3_ENDPOINT_URL=$ENDPOINT
export MYKEEBS_SWITCH_TABLE_NAME=$(out SwitchTableName)
export MYKEEBS_EC_PART_TABLE_NAME=$(out ECPartTableName)
export MYKEEBS_KEYBOARD_TABLE_NAME=$(out KeyboardTableName)
export MYKEEBS_KEYCAP_SET_TABLE_NAME=$(out KeycapSetTableName)
export MYKEEBS_BUILD_TABLE_NAME=$(out BuildTableName)
export MYKEEBS_PROFILE_TABLE_NAME=$(out ProfileTableName)
export MYKEEBS_PROFILE_USERNAME_TABLE_NAME=$(out ProfileUsernameTableName)
export MYKEEBS_UPLOAD_COUNTER_TABLE_NAME=$(out UploadCounterTableName)
export MYKEEBS_IMAGES_BUCKET_NAME=$IMAGES_BUCKET
export MYKEEBS_IMAGES_DOMAIN=$(out ImagesDomain)
export MYKEEBS_IMAGES_DISTRIBUTION_ID=$(out ImagesDistributionId)
export MYKEEBS_IMAGES_INVALIDATION=true
export MYKEEBS_ALARM_TOPIC_ARN=$(out AlarmTopicArn)
export MYKEEBS_STACK_NAME=$STACK
export MYKEEBS_AWS_ENDPOINT_URL=$ENDPOINT
export MYKEEBS_CA_CERT_PATH='$CERT_DIR/ca.crt'
export MYKEEBS_OIDC_ISSUER='$ISSUER'
export MYKEEBS_OIDC_AUDIENCE='$AUDIENCE'
export MYKEEBS_OIDC_SIGNING_KEY_PATH='$KEY_PATH'
export MYKEEBS_OIDC_GEN_DIR='$OIDC_DIR'
ENVEOF

echo "Deployed. Wrote $ENV_FILE - mise run func-test sources it automatically."
