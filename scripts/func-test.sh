#!/usr/bin/env bash
set -euo pipefail
# Table names/API URL/OIDC issuer+audience+signing-key path are real,
# stack-derived (or run-generated) values from a floci or CI deploy, not fixed
# local names - func-setup.sh writes them to .env.floci, which we source below;
# ci.yml sets them itself for its real deployed stack, so only source
# .env.floci when it exists.
ENV_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env.floci"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

: "${MYKEEBS_SWITCH_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_EC_PART_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_KEYBOARD_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_KEYCAP_SET_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_BUILD_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_PROFILE_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_PROFILE_USERNAME_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_UPLOAD_COUNTER_TABLE_NAME:?run scripts/func-setup.sh first}"
: "${MYKEEBS_API_BASE_URL:?run scripts/func-setup.sh first}"
: "${MYKEEBS_OIDC_ISSUER:?run scripts/func-setup.sh first}"
: "${MYKEEBS_OIDC_AUDIENCE:?run scripts/func-setup.sh first}"
: "${MYKEEBS_OIDC_SIGNING_KEY_PATH:?run scripts/func-setup.sh first}"
: "${MYKEEBS_IMAGES_DOMAIN:?run scripts/func-setup.sh first}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-2}"
# The authorize suite drives the consent page in headless Chromium; CI's
# fresh runners also need its system libraries.
go tool playwright install ${CI:+--with-deps} chromium
go tool ginkgo run -p ./test/functional/...
