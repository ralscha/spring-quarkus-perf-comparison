"""Exercise the production schedule loop with fake host services and load tests."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPT_DIR = Path(__file__).resolve().parent
BASH = shutil.which('bash')
if os.name == 'nt':
    # The Windows bash launcher requires WSL; Git Bash runs this harness locally.
    BASH = shutil.which('bash', path=r'C:\Program Files\Git\bin')

HOST_STUBS = r'''
set -euo pipefail
COMPOSE=(docker compose)
JVM_OPTS=()
active_unit=''
sampler_pid=''
APP_CPU_COUNT=4
APP_CPUSET=2-5
APP_MEMORY_LIMIT=6G
K6_CPUSET=6-7
JAVA_BIN=/unused/java

python3() { "${TEST_PYTHON}" "$@"; }
docker() {
  # End the background resource sampler without requiring Linux host metrics.
  if [[ " $* " == *' stats '* ]]; then return 1; fi
  if [[ " $* " == *' exec '* ]]; then
    cat >> "${RESULTS_DIR}/consumed-stdin.txt"
    if [[ " $* " == *' -Atqc '* ]]; then echo 1000; fi
  fi
}
systemd-run() { :; }
systemctl() {
  if [[ $1 == is-active && ${label} == go-r1-rate100 && ${TEST_FAILURE} == startup ]]; then return 1; fi
}
curl() {
  if [[ ${label} == go-r1-rate100 && ${TEST_FAILURE} == startup ]]; then
    echo 'curl: simulated connection failure' >&2
    return 7
  fi
}
taskset() {
  # Both database commands and load generators can try to read all of stdin.
  cat >> "${RESULTS_DIR}/consumed-stdin.txt"
  if [[ ${label} == go-r1-rate100 && ${TEST_FAILURE} == k6 ]]; then
    echo 'simulated load-test failure' >&2
    return 42
  fi
  python3 - "${K6_RESULT_FILE}" <<'PY'
import json, pathlib, sys
result = dict(passed=True, complete=True, achieved_rps=100,
              p50_ms=10, p95_ms=20, p99_ms=50, error_rate=0,
              check_failure_rate=0, dropped_iterations=0)
pathlib.Path(sys.argv[1]).write_text(json.dumps(result))
PY
}
'''


@unittest.skipUnless(BASH, 'Bash is required for the runner regression tests')
class RunnerTest(unittest.TestCase):
    def run_schedule(self, failure=''):
        source = (SCRIPT_DIR / 'run-benchmarks.sh').read_text(encoding='utf-8')
        # Exercise the real cleanup and execution loop, excluding privileged
        # provisioning checks and metadata collection before the schedule runs.
        cleanup = source[source.index('stop_app() {'):source.index('frameworks=${BENCHMARK_FRAMEWORKS}')]
        loop = source[source.index('failed=0\n'):]
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / 'logs').mkdir()
            (directory / 'runs').mkdir()
            (directory / 'schedule.tsv').write_text(
                '1\tgo\t100\n1\tgo\t500\n2\tgo\t100\n2\tgo\t500\n', encoding='utf-8', newline='\n')
            env = dict(os.environ, SCRIPT_DIR=SCRIPT_DIR.as_posix(),
                       REPO_DIR=directory.as_posix(), RESULTS_DIR=directory.as_posix(),
                       BENCHMARK_ROUNDS='2', TEST_FAILURE=failure,
                       TEST_PYTHON=Path(sys.executable).as_posix())
            process = subprocess.run([BASH, '-c', HOST_STUBS + cleanup + loop],
                                     env=env, stdin=subprocess.DEVNULL, capture_output=True,
                                     text=True, timeout=30)
            records = {path.stem: json.loads(path.read_text()) for path in (directory / 'runs').glob('*.json')}
            self.assertEqual(set(records), {'go-r1-rate100', 'go-r1-rate500', 'go-r2-rate100', 'go-r2-rate500'},
                             process.stdout + process.stderr)
            self.assertEqual((directory / 'consumed-stdin.txt').read_text(), '')
            report = json.loads((directory / 'summary.json').read_text())
            return process, records, report

    def test_stdin_readers_do_not_skip_scheduled_runs(self):
        process, records, report = self.run_schedule()
        self.assertEqual(process.returncode, 0, process.stdout + process.stderr)
        self.assertTrue(all(record['passed'] for record in records.values()))
        self.assertEqual(report['capacity'][0]['highest_tested_passing_rate'], 500)

    def test_startup_failure_is_reported_and_later_runs_continue(self):
        process, records, report = self.run_schedule('startup')
        self.assertEqual(process.returncode, 1, process.stdout + process.stderr)
        self.assertEqual(records['go-r1-rate100']['failure'], 'startup')
        self.assertTrue(records['go-r2-rate100']['passed'])
        self.assertIn('go-r1-rate100: server failed to start; see ', process.stderr)
        self.assertEqual(report['capacity'][0]['highest_tested_passing_rate'], 500)

    def test_load_test_failure_is_reported_and_later_runs_continue(self):
        process, records, report = self.run_schedule('k6')
        self.assertEqual(process.returncode, 1, process.stdout + process.stderr)
        self.assertEqual(records['go-r1-rate100']['k6_exit_code'], 42)
        self.assertFalse(records['go-r1-rate100']['passed'])
        self.assertTrue(records['go-r2-rate100']['passed'])
        self.assertIn('go-r1-rate100: benchmark failed (k6 exit 42, app active 1); see ', process.stderr)
        self.assertEqual(report['capacity'][0]['highest_tested_passing_rate'], 500)


if __name__ == '__main__':
    unittest.main()
