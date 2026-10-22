import statistics
import unittest
from summarize import aggregate


def record(round_number, passed=True, p95=20):
    return dict(framework='example', rate=100, round=round_number, passed=passed, complete=True,
                achieved_rps=100, p50_ms=10, p95_ms=p95, p99_ms=50,
                error_rate=0, check_failure_rate=0, dropped_iterations=0)


class SummaryTest(unittest.TestCase):
    def test_variability_and_all_repetitions_required(self):
        rows = aggregate([record(1, p95=10), record(2, p95=20), record(3, p95=30)], 3)
        self.assertTrue(rows[0]['slo_pass'])
        self.assertEqual(rows[0]['median_p95_ms'], 20)
        self.assertEqual(rows[0]['stdev_p95_ms'], 10)
        self.assertEqual(rows[0]['p95_cv_percent'], 50)
        self.assertFalse(aggregate([record(1), record(2)], 3)[0]['slo_pass'])
        self.assertFalse(aggregate([record(1), record(2), record(2)], 3)[0]['slo_pass'])

    def test_failures_and_missing_samples_are_never_ranked_as_passes(self):
        self.assertFalse(aggregate([record(1), record(2, passed=False)], 2)[0]['slo_pass'])
        startup_failure = dict(framework='example', rate=100, round=1, passed=False, complete=False)
        result = aggregate([startup_failure], 1)[0]
        self.assertFalse(result['slo_pass'])
        self.assertIsNone(result['median_p95_ms'])

    def test_coefficient_of_variation_uses_mean_not_median(self):
        row = aggregate([record(1, p95=10), record(2, p95=20), record(3, p95=90)], 3)[0]
        self.assertEqual(row['median_p95_ms'], 20)
        self.assertEqual(row['p95_cv_percent'], round(100 * statistics.stdev([10, 20, 90]) / 40, 2))

    def test_rates_are_not_compared_as_same_load(self):
        higher = record(1)
        higher['rate'] = 1000
        self.assertEqual(len(aggregate([record(1), higher], 1)), 2)

    def test_missing_entire_group_and_nonfinite_results_fail(self):
        schedule = [dict(framework='example', rate=100, round=1), dict(framework='missing', rate=100, round=1)]
        rows = aggregate([record(1)], 1, schedule)
        self.assertEqual(len(rows), 2)
        self.assertFalse(rows[1]['slo_pass'])
        self.assertIsNone(rows[1]['median_p95_ms'])
        invalid = record(1, p95=float('nan'))
        self.assertFalse(aggregate([invalid], 1)[0]['slo_pass'])


if __name__ == '__main__':
    unittest.main()
