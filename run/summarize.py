#!/usr/bin/env python3
"""Summarize fixed offered-load runs, retaining failures and incomplete rounds."""
import csv
import json
import math
import statistics
import sys
from collections import defaultdict
from pathlib import Path


METRICS = ('achieved_rps', 'p50_ms', 'p95_ms', 'p99_ms', 'error_rate', 'check_failure_rate', 'dropped_iterations')


def aggregate(records, rounds, schedule=None):
    groups = defaultdict(list)
    expected = {(entry['framework'], entry['rate'], entry['round']) for entry in schedule} if schedule is not None else None
    seen = set()
    for original in records:
        record = dict(original)
        identity = (record['framework'], record['rate'], record['round'])
        seen.add(identity)
        if expected is not None and identity not in expected:
            record['passed'] = False
        if record.get('complete') and not all(isinstance(record.get(name), (int, float)) and math.isfinite(record[name]) and record[name] >= 0 for name in METRICS):
            record.update(complete=False, passed=False)
        groups[(record['framework'], record['rate'])].append(record)
    if expected is not None:
        for framework, rate, round_number in expected - seen:
            groups[(framework, rate)].append(dict(framework=framework, rate=rate, round=round_number, passed=False, complete=False))
    rows = []
    for (framework, rate), group in sorted(groups.items()):
        valid = [record for record in group if record.get('complete')]
        passed = len(group) == rounds and len({r['round'] for r in group}) == rounds and all(r.get('passed', False) for r in group)
        row = {'framework': framework, 'rate': rate, 'rounds': len(group), 'slo_pass': passed}
        for name in METRICS:
            values = [float(record[name]) for record in valid]
            row[f'median_{name}'] = round(statistics.median(values), 4) if values else None
            row[f'stdev_{name}'] = round(statistics.stdev(values), 4) if len(values) > 1 else 0 if values else None
        p95_values = [float(record['p95_ms']) for record in valid]
        p95_mean = statistics.mean(p95_values) if p95_values else None
        p95_stdev = statistics.stdev(p95_values) if len(p95_values) > 1 else 0
        row['p95_cv_percent'] = round(100 * p95_stdev / p95_mean, 2) if p95_mean else None
        rows.append(row)
    return rows


def write_results(directory, rounds):
    records = [json.loads(path.read_text(encoding='utf-8')) for path in sorted(directory.glob('runs/*.json'))]
    with (directory / 'schedule.tsv').open(encoding='utf-8') as schedule_file:
        schedule = [dict(round=int(row[0]), framework=row[1], rate=int(row[2])) for row in csv.reader(schedule_file, delimiter='\t')]
    rows = aggregate(records, rounds, schedule)
    if not rows:
        raise ValueError('No benchmark runs were recorded')
    with (directory / 'summary.tsv').open('w', newline='', encoding='utf-8') as output:
        writer = csv.DictWriter(output, fieldnames=list(rows[0]), delimiter='\t')
        writer.writeheader()
        writer.writerows(rows)
    capacity = []
    for framework in sorted({row['framework'] for row in rows}):
        passed = [row for row in rows if row['framework'] == framework and row['slo_pass']]
        best = max(passed, key=lambda row: row['rate']) if passed else None
        capacity.append({'framework': framework, 'highest_tested_passing_rate': best['rate'] if best else None,
                         'median_p95_at_that_rate_ms': best['median_p95_ms'] if best else None})
    report = {'rounds_required': rounds, 'groups': rows, 'capacity': capacity}
    (directory / 'summary.json').write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(capacity, indent=2))
    return all(row['slo_pass'] for row in rows)


if __name__ == '__main__':
    sys.exit(0 if write_results(Path(sys.argv[1]), int(sys.argv[2])) else 1)
