#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_DIR=$(cd -- "${SCRIPT_DIR}/.." && pwd)
BENCHMARK_ROUNDS=${BENCHMARK_ROUNDS:-3}
BENCHMARK_RATES=${BENCHMARK_RATES:-100,500,1000}
BENCHMARK_FRAMEWORKS=${BENCHMARK_FRAMEWORKS:-go,rust,micronaut5,quarkus3,springboot4}
BENCHMARK_VARIANTS=${BENCHMARK_VARIANTS:-0}
BENCHMARK_SEED=${BENCHMARK_SEED:-42}
RESULTS_DIR=${RESULTS_DIR:-${SCRIPT_DIR}/results/$(date -u +%Y%m%dT%H%M%SZ)}
export POSTGRES_CPUSET=${POSTGRES_CPUSET:-0-1}
export POSTGRES_CPU_LIMIT=${POSTGRES_CPU_LIMIT:-2}
APP_CPUSET=${APP_CPUSET:-2-5}
APP_CPU_COUNT=${APP_CPU_COUNT:-4}
APP_MEMORY_LIMIT=${APP_MEMORY_LIMIT:-6G}
K6_CPUSET=${K6_CPUSET:-6-7}
export K6_SEED=${K6_SEED:-${BENCHMARK_SEED}}
export K6_PROFILE=${K6_PROFILE:-mixed}
export K6_BASE_URL=http://127.0.0.1:8080
export JAVA_HOME=${JAVA_HOME:-/opt/java/openjdk-27}
export PATH="${JAVA_HOME}/bin:/usr/local/go/bin:${PATH}"
JAVA_BIN="${JAVA_HOME}/bin/java"
[[ -x ${JAVA_BIN} ]] || { echo "Java 27 is required at ${JAVA_BIN}; set JAVA_HOME to a Java 27 installation." >&2; exit 1; }
java_spec_version=$("${JAVA_BIN}" -XshowSettings:properties -version 2>&1 | awk '$1 == "java.specification.version" { print $3 }')
[[ ${java_spec_version} == 27 ]] || { echo "Benchmark requires Java 27; JAVA_HOME selects Java ${java_spec_version}." >&2; exit 1; }
COMPOSE=(docker compose --project-name fruits-benchmark -f "${SCRIPT_DIR}/docker-compose.yml")
JVM_OPTS=(-XX:ActiveProcessorCount="${APP_CPU_COUNT}" -Xms1g -Xmx4g)
active_unit=''
sampler_pid=''

if [[ ${EUID} -ne 0 ]]; then echo 'Run with sudo on the dedicated Linux benchmark host.' >&2; exit 1; fi
for command in docker k6 python3 systemd-run taskset flock curl ss; do command -v "${command}" >/dev/null; done
exec 9>/var/lock/fruits-benchmark.lock
flock -n 9 || { echo 'A benchmark is already running.' >&2; exit 1; }
[[ -z $(ss -H -ltn 'sport = :8080') ]] || { echo 'Port 8080 is already occupied; stop that service first.' >&2; exit 1; }
[[ ${BENCHMARK_ROUNDS} =~ ^[1-9][0-9]*$ && ${BENCHMARK_SEED} =~ ^[0-9]+$ ]] || { echo 'Invalid rounds or seed' >&2; exit 1; }
[[ ${BENCHMARK_RATES} =~ ^[1-9][0-9]*(,[1-9][0-9]*)*$ ]] || { echo 'BENCHMARK_RATES must be comma-separated positive integers' >&2; exit 1; }
[[ ${APP_CPU_COUNT} =~ ^[1-9][0-9]*$ ]] || { echo 'APP_CPU_COUNT must be positive' >&2; exit 1; }
[[ ${BENCHMARK_VARIANTS} == 0 || ${BENCHMARK_VARIANTS} == 1 ]] || { echo 'BENCHMARK_VARIANTS must be 0 or 1' >&2; exit 1; }

# Reject overlapping/out-of-range CPU assignments before starting any workload.
python3 - "${POSTGRES_CPUSET}" "${APP_CPUSET}" "${K6_CPUSET}" "${APP_CPU_COUNT}" <<'PY'
import os, sys
def parse(value):
    result = set()
    for part in value.split(','):
        bounds = list(map(int, part.split('-')))
        result.update(range(bounds[0], bounds[-1] + 1))
    return result
sets = [parse(value) for value in sys.argv[1:4]]
assert all(sets), 'CPU sets must not be empty'
assert all(group <= os.sched_getaffinity(0) for group in sets), 'CPU set exceeds available host CPUs'
assert not any(sets[i] & sets[j] for i in range(3) for j in range(i)), 'CPU sets overlap'
assert len(sets[1]) == int(sys.argv[4]), 'APP_CPU_COUNT must match APP_CPUSET'
PY

mkdir -p "${RESULTS_DIR}"
RESULTS_DIR=$(cd -- "${RESULTS_DIR}" && pwd)
[[ ! -e ${RESULTS_DIR}/schedule.tsv ]] || { echo 'RESULTS_DIR already contains a run; choose a fresh directory' >&2; exit 1; }
mkdir -p "${RESULTS_DIR}/logs" "${RESULTS_DIR}/runs"

stop_app() {
  if [[ -n ${sampler_pid} ]]; then kill "${sampler_pid}" 2>/dev/null || true; wait "${sampler_pid}" 2>/dev/null || true; sampler_pid=''; fi
  if [[ -n ${active_unit} ]]; then systemctl stop "${active_unit}" >/dev/null 2>&1 || true; systemctl reset-failed "${active_unit}" >/dev/null 2>&1 || true; active_unit=''; fi
}
cleanup() { stop_app; "${COMPOSE[@]}" down --volumes >/dev/null 2>&1 || true; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

frameworks=${BENCHMARK_FRAMEWORKS}
if [[ ${BENCHMARK_VARIANTS} == 1 ]]; then frameworks+=',go-pgo,springboot4-aot-cache'; fi
# Shuffle reproducibly in each round, including the rate order to reduce drift bias.
python3 - "${BENCHMARK_ROUNDS}" "${BENCHMARK_RATES}" "${frameworks}" "${BENCHMARK_SEED}" > "${RESULTS_DIR}/schedule.tsv" <<'PY'
import itertools, random, sys
frameworks = sys.argv[3].split(',')
allowed = {'go','go-pgo','rust','micronaut5','quarkus3','springboot4','springboot4-aot-cache'}
assert set(frameworks) <= allowed and len(set(frameworks)) == len(frameworks), 'Invalid or duplicate framework'
rates = [int(rate) for rate in sys.argv[2].split(',')]
assert len(set(rates)) == len(rates), 'Duplicate rate'
rng = random.Random(int(sys.argv[4]))
for round_number in range(1, int(sys.argv[1]) + 1):
    entries = list(itertools.product(frameworks, rates))
    rng.shuffle(entries)
    for framework, rate in entries:
        print(round_number, framework, rate, sep='\t')
PY

{
  date -u --iso-8601=seconds
  git -C "${REPO_DIR}" rev-parse HEAD
  git -C "${REPO_DIR}" status --short
  uname -a
  lscpu
  free -h
  "${JAVA_BIN}" -version 2>&1
  go version
  (cd "${REPO_DIR}/rust" && rustc --version) 2>/dev/null || true
  k6 version
  docker version
  "${COMPOSE[@]}" config
  # An explicit allowlist avoids capturing unrelated credentials such as K6_CLOUD_TOKEN.
  for setting in JAVA_HOME JAVA_BIN APP_CPUSET APP_CPU_COUNT APP_MEMORY_LIMIT K6_CPUSET POSTGRES_CPUSET POSTGRES_CPU_LIMIT POSTGRES_MEMORY_LIMIT \
    BENCHMARK_ROUNDS BENCHMARK_RATES BENCHMARK_FRAMEWORKS BENCHMARK_VARIANTS BENCHMARK_SEED \
    K6_BASE_URL K6_PROFILE K6_DURATION_SECONDS K6_BURST_MULTIPLIER K6_PREALLOCATED_VUS \
    K6_WARMUP_RAMP_SECONDS K6_WARMUP_HOLD_SECONDS K6_TIMEOUT_SECONDS K6_GRACE_SECONDS \
    K6_P95_MS K6_P99_MS K6_ERROR_RATE K6_MAX_CREATES K6_SEED; do
    printf '%s=%s\n' "${setting}" "${!setting-<default>}"
  done
} > "${RESULTS_DIR}/environment.txt"

"${COMPOSE[@]}" pull
docker image inspect postgres:18.6 > "${RESULTS_DIR}/postgres-image.json"
failed=0
while IFS=$'\t' read -r round framework rate; do
  label="${framework}-r${round}-rate${rate}"
  echo "[$(date -u --iso-8601=seconds)] ${label}"
  # The dedicated Compose project owns only disposable benchmark data.
  "${COMPOSE[@]}" down --volumes >/dev/null
  "${COMPOSE[@]}" up -d --wait --wait-timeout 180
  seed_count=$("${COMPOSE[@]}" exec -T postgres psql -U fruits -d fruits -Atqc 'SELECT count(*) FROM fruits')
  [[ ${seed_count} == 1000 ]] || { echo 'Incorrect database seed' >&2; exit 1; }
  "${COMPOSE[@]}" exec -T postgres psql -U fruits -d fruits -c 'SELECT version(); SHOW fsync; SHOW synchronous_commit; SHOW full_page_writes; SHOW autovacuum;' > "${RESULTS_DIR}/logs/${label}-postgres.txt"
  workdir="${REPO_DIR}/${framework}"
  case ${framework} in
    go) command=(env GOMAXPROCS="${APP_CPU_COUNT}" GOMEMLIMIT=4GiB ./server) ;;
    go-pgo) workdir="${REPO_DIR}/go"; command=(env GOMAXPROCS="${APP_CPU_COUNT}" GOMEMLIMIT=4GiB ./server_pgo) ;;
    rust) command=(env TOKIO_WORKER_THREADS="${APP_CPU_COUNT}" ./target/release/fruits-rust) ;;
    micronaut5) command=("${JAVA_BIN}" "${JVM_OPTS[@]}" -jar target/micronaut5.jar) ;;
    quarkus3) command=("${JAVA_BIN}" "${JVM_OPTS[@]}" -jar target/quarkus-app/quarkus-run.jar) ;;
    springboot4) command=("${JAVA_BIN}" "${JVM_OPTS[@]}" -jar target/app-jvm.jar) ;;
    springboot4-aot-cache) workdir="${REPO_DIR}/springboot4/target/springboot4-aot-cache"; command=("${JAVA_BIN}" "${JVM_OPTS[@]}" -XX:AOTCache=app.aot -Dspring.aot.enabled=true -jar springboot4.jar) ;;
  esac
  active_unit="fruits-bench-${label}"
  start_ns=$(date +%s%N)
  systemd-run --quiet --unit="${active_unit}" --service-type=exec --expand-environment=no --working-directory="${workdir}" \
    --property="AllowedCPUs=${APP_CPUSET}" --property="CPUQuota=$((APP_CPU_COUNT * 100))%" \
    --property="MemoryMax=${APP_MEMORY_LIMIT}" --property=MemorySwapMax=0 --property=TasksMax=8192 \
    --property=CPUAccounting=yes --property=MemoryAccounting=yes --property=TimeoutStopSec=15 \
    --property="StandardOutput=append:${RESULTS_DIR}/logs/${label}-server.txt" \
    --property="StandardError=append:${RESULTS_DIR}/logs/${label}-server.txt" \
    "${command[@]}"
  ready=0
  for ((attempt=0; attempt<180; attempt++)); do
    if curl --max-time 2 -fsS 'http://127.0.0.1:8080/fruits/Apple' >/dev/null; then ready=1; break; fi
    systemctl is-active --quiet "${active_unit}" || break
    sleep 1
  done
  startup_ms=$(( ($(date +%s%N) - start_ns) / 1000000 ))
  if [[ ${ready} == 0 ]]; then
    echo "${label}: server failed to start" >&2
    printf '{"framework":"%s","rate":%s,"round":%s,"passed":false,"complete":false,"failure":"startup"}\n' "${framework}" "${rate}" "${round}" > "${RESULTS_DIR}/runs/${label}.json"
    failed=1; stop_app; continue
  fi
  (
    while true; do
      date -u --iso-8601=seconds
      systemctl show "${active_unit}" --property=CPUUsageNSec --property=MemoryCurrent --property=MemoryPeak --property=TasksCurrent
      # Per-CPU ticks expose load-generator saturation on its dedicated cores.
      awk '/^cpu[0-9]/ { print }' /proc/stat
      cat /proc/loadavg /proc/pressure/cpu /proc/pressure/io /proc/pressure/memory
      docker stats --no-stream --format '{{json .}}' "$("${COMPOSE[@]}" ps -q postgres)"
      sleep 5
    done
  ) > "${RESULTS_DIR}/logs/${label}-resources.txt" 2>&1 &
  sampler_pid=$!
  exit_code=0
  K6_RATE="${rate}" K6_RUN_ID="${label}" K6_RESULT_FILE="${RESULTS_DIR}/logs/${label}-k6.json" \
    taskset --cpu-list "${K6_CPUSET}" k6 run --no-usage-report "${SCRIPT_DIR}/stress-k6.js" \
    > "${RESULTS_DIR}/logs/${label}-k6.txt" 2>&1 || exit_code=$?
  app_active=0
  systemctl is-active --quiet "${active_unit}" && app_active=1
  systemctl show "${active_unit}" --property=CPUUsageNSec --property=MemoryPeak --property=Result \
    --property=EffectiveCPUs --property=EffectiveMemoryMax --property=MemorySwapMax > "${RESULTS_DIR}/logs/${label}-app-final.txt" || true
  "${COMPOSE[@]}" exec -T postgres psql -U fruits -d fruits -c 'SELECT count(*) FROM fruits; SELECT * FROM pg_stat_database WHERE datname = current_database();' >> "${RESULTS_DIR}/logs/${label}-postgres.txt"
  python3 - "${RESULTS_DIR}" "${label}" "${framework}" "${rate}" "${round}" "${exit_code}" "${startup_ms}" "${app_active}" <<'PY'
import json, pathlib, sys
directory, label, framework, rate, round_number, exit_code, startup, app_active = sys.argv[1:]
directory = pathlib.Path(directory)
path = directory / 'logs' / f'{label}-k6.json'
result = json.loads(path.read_text()) if path.exists() else {'passed': False, 'complete': False, 'failure': 'missing k6 summary'}
result.update(framework=framework, rate=int(rate), round=int(round_number), k6_exit_code=int(exit_code), startup_ms=int(startup))
result['passed'] = result['passed'] and int(exit_code) == 0 and app_active == '1'
(directory / 'runs' / f'{label}.json').write_text(json.dumps(result, indent=2) + '\n')
PY
  [[ ${exit_code} == 0 && ${app_active} == 1 ]] || failed=1
  stop_app
done < "${RESULTS_DIR}/schedule.tsv"

python3 "${SCRIPT_DIR}/summarize.py" "${RESULTS_DIR}" "${BENCHMARK_ROUNDS}" || failed=1
echo "Results: ${RESULTS_DIR}"
exit "${failed}"
