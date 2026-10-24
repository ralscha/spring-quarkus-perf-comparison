#!/usr/bin/env bash
set -euo pipefail

# Run on a fresh Debian 13 x86_64 host. The repository revision is selected by
# cloud-init before this script; the same checkout supplies builds and workload.
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_DIR=$(cd -- "${SCRIPT_DIR}/.." && pwd)
export DEBIAN_FRONTEND=noninteractive
export JAVA_HOME=/opt/java/openjdk-27
export CARGO_HOME=/opt/rust/cargo
export RUSTUP_HOME=/opt/rust/rustup
export PATH="${JAVA_HOME}/bin:${CARGO_HOME}/bin:/usr/local/go/bin:${PATH}"
GO_VERSION=1.27.3
RUST_VERSION=1.99.1
K6_VERSION=2.3.0
[[ ${EUID} -eq 0 ]] || { echo 'Run as root on the dedicated benchmark host' >&2; exit 1; }
[[ $(uname -m) == x86_64 ]] || { echo 'Bootstrap currently supports x86_64 hosts' >&2; exit 1; }
# Cloud-init may omit HOME; Maven Wrapper and other build tools need it.
export HOME="${HOME:-/root}"
exec > >(tee -a /var/log/bootstrap-benchmark-host.log) 2>&1

apt-get update
apt-get install -y build-essential ca-certificates curl git gnupg jq python3 screen tar unzip xz-utils
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y containerd.io docker-buildx-plugin docker-ce docker-ce-cli docker-compose-plugin
systemctl enable --now docker

download_dir=$(mktemp -d)
profile_pid=''
COMPOSE=(docker compose --project-name fruits-benchmark-training -f "${SCRIPT_DIR}/docker-compose.yml")
cleanup() {
  if [[ -n ${profile_pid} ]]; then kill "${profile_pid}" 2>/dev/null || true; wait "${profile_pid}" 2>/dev/null || true; fi
  "${COMPOSE[@]}" down --volumes >/dev/null 2>&1 || true
  rm -rf -- "${download_dir}"
}
trap cleanup EXIT

# Resolve the latest Java 27 patch, record metadata, and verify the archive.
curl -fsSL 'https://api.adoptium.net/v3/assets/latest/27/hotspot?architecture=x64&image_type=jdk&os=linux&vendor=eclipse' > "${download_dir}/java.json"
java_url=$(jq -er '.[0].binary.package.link' "${download_dir}/java.json")
java_checksum=$(jq -er '.[0].binary.package.checksum' "${download_dir}/java.json")
curl -fsSL "${java_url}" -o "${download_dir}/jdk.tar.gz"
printf '%s  %s\n' "${java_checksum}" "${download_dir}/jdk.tar.gz" | sha256sum --check -
install -d "${JAVA_HOME}"
tar -xzf "${download_dir}/jdk.tar.gz" --strip-components=1 -C "${JAVA_HOME}"
ln -sf "${JAVA_HOME}/bin/java" /usr/local/bin/java
ln -sf "${JAVA_HOME}/bin/javac" /usr/local/bin/javac
JAVA_BIN="${JAVA_HOME}/bin/java"
java_spec_version=$("${JAVA_BIN}" -XshowSettings:properties -version 2>&1 | awk '$1 == "java.specification.version" { print $3 }')
[[ ${java_spec_version} == 27 ]] || { echo "Bootstrap requires Java 27; downloaded Java ${java_spec_version}." >&2; exit 1; }

curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o "${download_dir}/go.tar.gz"
curl -fsSL "https://go.dev/dl/?mode=json&include=all" > "${download_dir}/go.json"
go_checksum=$(jq -er --arg name "go${GO_VERSION}.linux-amd64.tar.gz" '.[] | .files[] | select(.filename == $name) | .sha256' "${download_dir}/go.json")
printf '%s  %s\n' "${go_checksum}" "${download_dir}/go.tar.gz" | sha256sum --check -
rm -rf /usr/local/go
tar -C /usr/local -xzf "${download_dir}/go.tar.gz"
ln -sf /usr/local/go/bin/go /usr/local/bin/go

curl -fsSL https://sh.rustup.rs -o "${download_dir}/rustup.sh"
sh "${download_dir}/rustup.sh" -y --no-modify-path --profile minimal --default-toolchain "${RUST_VERSION}"
curl -fsSL "https://github.com/grafana/k6/releases/download/v${K6_VERSION}/k6-v${K6_VERSION}-linux-amd64.tar.gz" -o "${download_dir}/k6-v${K6_VERSION}-linux-amd64.tar.gz"
curl -fsSL "https://github.com/grafana/k6/releases/download/v${K6_VERSION}/k6-v${K6_VERSION}-checksums.txt" -o "${download_dir}/k6-checksums.txt"
(cd "${download_dir}" && grep " k6-v${K6_VERSION}-linux-amd64.tar.gz$" k6-checksums.txt | sha256sum --check -)
tar -xzf "${download_dir}/k6-v${K6_VERSION}-linux-amd64.tar.gz" -C "${download_dir}"
install -m 0755 "${download_dir}/k6-v${K6_VERSION}-linux-amd64/k6" /usr/local/bin/k6

install -d "${REPO_DIR}/build-metadata"
cp "${download_dir}/java.json" "${REPO_DIR}/build-metadata/java.json"
{
  git -C "${REPO_DIR}" rev-parse HEAD
  "${JAVA_BIN}" -version 2>&1
  go version
  rustc --version
  k6 version
} > "${REPO_DIR}/build-metadata/versions.txt"

for module in micronaut5 quarkus3 springboot4; do
  chmod +x "${REPO_DIR}/${module}/mvnw"
  (cd "${REPO_DIR}/${module}" && ./mvnw -B -DskipTests clean package)
done
cp "${REPO_DIR}/springboot4/target/springboot4.jar" "${REPO_DIR}/springboot4/target/app-jvm.jar"
(cd "${REPO_DIR}/go" && go build -trimpath -pgo=off -o server .)
(cd "${REPO_DIR}/rust" && cargo build --release --locked)

"${COMPOSE[@]}" pull
"${COMPOSE[@]}" up -d --wait --wait-timeout 180
wait_for_app() {
  for ((attempt=0; attempt<180; attempt++)); do
    if curl --max-time 2 -fsS http://127.0.0.1:8080/fruits/Apple >/dev/null; then return 0; fi
    kill -0 "${profile_pid}" 2>/dev/null || return 1
    sleep 1
  done
  return 1
}

# Train PGO with the same mix and data, separately from measured runs.
(cd "${REPO_DIR}/go" && exec env GOMAXPROCS=4 GOMEMLIMIT=4GiB ./server -cpuprofile "${REPO_DIR}/go/server.pprof") > "${REPO_DIR}/build-metadata/go-training.log" 2>&1 &
profile_pid=$!
wait_for_app
K6_RATE=100 K6_RUN_ID=pgo-training K6_RESULT_FILE="${REPO_DIR}/build-metadata/go-training.json" \
  k6 run --no-thresholds --no-usage-report "${SCRIPT_DIR}/stress-k6.js" > "${REPO_DIR}/build-metadata/go-training-k6.txt" 2>&1
python3 - "${REPO_DIR}/build-metadata/go-training.json" <<'PY'
import json, sys
result = json.load(open(sys.argv[1]))
metrics = result['metrics']
assert result['requests'] > 0 and result['complete'], 'PGO training did not complete'
assert result['warmup']['complete'], 'PGO warmup did not complete'
assert metrics['checks']['values']['rate'] == 1, 'PGO training returned incorrect responses'
assert metrics['http_req_failed']['values']['rate'] == 0, 'PGO training had HTTP failures'
assert metrics.get('dropped_iterations', {}).get('values', {}).get('count', 0) == 0, 'PGO training dropped arrivals'
PY
kill "${profile_pid}"
wait "${profile_pid}"
profile_pid=''
test -s "${REPO_DIR}/go/server.pprof"
(cd "${REPO_DIR}/go" && go build -trimpath -pgo=server.pprof -o server_pgo .)
"${COMPOSE[@]}" down --volumes

# Keep the normal artifact and the trained AOT artifact independently runnable.
(cd "${REPO_DIR}/springboot4" && ./mvnw -B -DskipTests compile spring-boot:process-aot package)
extracted_dir="${REPO_DIR}/springboot4/target/springboot4-aot-cache"
"${JAVA_BIN}" -Djarmode=tools -jar "${REPO_DIR}/springboot4/target/springboot4.jar" extract --destination "${extracted_dir}"
"${COMPOSE[@]}" up -d --wait --wait-timeout 180
(cd "${extracted_dir}" && "${JAVA_BIN}" -XX:AOTCacheOutput=app.aot -Dspring.aot.enabled=true -Dspring.context.exit=onRefresh -jar springboot4.jar)
test -s "${extracted_dir}/app.aot"

cat > /usr/local/bin/run-benchmarks.sh <<EOF
#!/usr/bin/env bash
export JAVA_HOME=${JAVA_HOME}
export PATH=${JAVA_HOME}/bin:${CARGO_HOME}/bin:/usr/local/go/bin:\$PATH
export CARGO_HOME=${CARGO_HOME}
export RUSTUP_HOME=${RUSTUP_HOME}
exec bash '${SCRIPT_DIR}/run-benchmarks.sh' "\$@"
EOF
chmod +x /usr/local/bin/run-benchmarks.sh
echo 'Bootstrap complete. Run /usr/local/bin/run-benchmarks.sh on the idle dedicated host.'
