#!/usr/bin/python
# coding=utf-8
################################################################################

from test import CollectorTestCase
from test import get_collector_config
from test import unittest
from mock import Mock
from mock import patch

try:
    from cStringIO import StringIO
except ImportError:
    from StringIO import StringIO

from diamond.collector import Collector
from cpu import CPUCollector

################################################################################

def find_metric(metric_list, metric_name):
    return filter(lambda metric:metric["name"].find(metric_name) > -1, metric_list)


class TestCPUCollector(CollectorTestCase):
    def setUp(self):
        config = get_collector_config('CPUCollector', {
            'interval': 10,
            'normalize': False
        })

        self.collector = CPUCollector(config, None)

    def test_import(self):
        self.assertTrue(CPUCollector)

    @patch('__builtin__.open')
    @patch('os.access', Mock(return_value=True))
    @patch.object(Collector, 'publish')
    def test_should_open_proc_stat(self, publish_mock, open_mock):
        CPUCollector.PROC = '/proc/stat'
        open_mock.return_value = StringIO('')
        self.collector.collect()
        open_mock.assert_called_once_with('/proc/stat')

    @patch.object(Collector, 'publish')
    def test_should_work_with_synthetic_data(self, publish_mock):
        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu 100 200 300 400 500 0 0 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        self.assertPublishedMany(publish_mock, {})

        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu 110 220 330 440 550 0 0 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        self.assertPublishedMany(publish_mock, {
            'cpu.total.idle': 440,
            'cpu.total.iowait': 550,
            'cpu.total.nice': 220,
            'cpu.total.system': 330,
            'cpu.total.user': 110
        })

    @patch.object(Collector, 'publish')
    def test_should_work_with_real_data(self, publish_mock):
        CPUCollector.PROC = self.getFixturePath('proc_stat_1')
        self.collector.collect()

        self.assertPublishedMany(publish_mock, {})

        CPUCollector.PROC = self.getFixturePath('proc_stat_2')
        self.collector.collect()

        metrics = {
            'cpu.total.idle': 3925832001,
            'cpu.total.iowait': 575306,
            'cpu.total.nice': 1104382,
            'cpu.total.system': 8454154,
            'cpu.total.user': 29055791
        }

        self.setDocExample(collector=self.collector.__class__.__name__,
                           metrics=metrics,
                           defaultpath=self.collector.config['path'])
        self.assertPublishedMany(publish_mock, metrics)

    @patch.object(Collector, 'publish')
    def test_should_work_with_ec2_data(self, publish_mock):
        self.collector.config['interval'] = 30
        self.collector.config['xenfix'] = False
        patch_open = patch('os.path.isdir', Mock(return_value=True))
        patch_open.start()

        CPUCollector.PROC = self.getFixturePath('ec2_stat_1')
        self.collector.collect()

        self.assertPublishedMany(publish_mock, {})

        CPUCollector.PROC = self.getFixturePath('ec2_stat_2')
        self.collector.collect()

        patch_open.stop()

        metrics = {
            'cpu.total.idle': 2806608501,
            'cpu.total.iowait': 13567144,
            'cpu.total.nice': 15545,
            'cpu.total.system': 170762788,
            'cpu.total.user': 243646997
        }

        self.assertPublishedMany(publish_mock, metrics)

    @patch.object(Collector, 'publish')
    def test_total_metrics_enable_aggregation_false(self, publish_mock):
        self.collector.config['enableAggregation'] = False

        CPUCollector.PROC = self.getFixturePath('proc_stat_2')
        self.collector.collect()

        publishedMetrics = {
            'cpu.total.nice': 1104382,
            'cpu.total.irq': 3,
            'cpu.total.softirq': 59032,
            'cpu.total.user': 29055791
        }
        unpublishedMetrics = {
            'cpu.total.user_mode': 30160173,
            'cpu.total.irq_softirq': 59035
        }

        self.assertPublishedMany(publish_mock, publishedMetrics)

        self.collector.collect()
        self.assertUnpublishedMany(publish_mock, unpublishedMetrics)

    @patch.object(Collector, 'publish')
    def test_total_metrics_enable_aggregation_true(self, publish_mock):
        self.collector.config['enableAggregation'] = True

        CPUCollector.PROC = self.getFixturePath('proc_stat_2')
        self.collector.collect()

        publishedMetrics = {
            'cpu.total.nice': 1104382,
            'cpu.total.irq': 3,
            'cpu.total.softirq': 59032,
            'cpu.total.user': 29055791,
            'cpu.total.user_mode': 30160173,
            'cpu.total.irq_softirq': 59035
        }

        self.assertPublishedMany(publish_mock, publishedMetrics)

    @patch.object(Collector, 'publish')
    def test_total_metrics_enable_aggregation_true_blacklist(self, publish_mock):
        self.collector.config['enableAggregation'] = True

        CPUCollector.PROC = self.getFixturePath('proc_stat_2')
        self.collector.collect()

        publishedMetrics = {
            'cpu.total.nice': 1104382,
            'cpu.total.irq': 3,
            'cpu.total.softirq': 59032,
            'cpu.total.user': 29055791,
            'cpu.total.user_mode': 30160173,
            'cpu.total.irq_softirq': 59035
        }

        self.assertPublishedMany(publish_mock, publishedMetrics)

    @patch.object(Collector, 'publish')
    def test_core_metrics_enable_aggregation_false(self, publish_mock):
        self.collector.config['enableAggregation'] = False

        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu0 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        publishedMetrics = {
            'cpu.nice': 220,
            'cpu.irq': 660,
            'cpu.softirq': 770,
            'cpu.user': 110
        }

        self.assertPublishedMany(publish_mock, publishedMetrics)

        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu0 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()
        unpublishedMetrics = {

            'cpu.user_mode': 330,
            'cpu.irq_softirq': 1430
        }

        self.assertUnpublishedMany(publish_mock, unpublishedMetrics)

    @patch.object(Collector, 'publish')
    def test_core_metrics_enable_aggregation_true(self, publish_mock):
        self.collector.config['enableAggregation'] = True

        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu0 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        publishedMetrics = {
            'cpu.nice': 220,
            'cpu.irq': 660,
            'cpu.softirq': 770,
            'cpu.user': 110,
            'cpu.user_mode': 330,
            'cpu.irq_softirq': 1430
        }

        self.assertPublishedMany(publish_mock, publishedMetrics)

class TestCPUCollectorNormalize(CollectorTestCase):

    def setUp(self):
        config = get_collector_config('CPUCollector', {
            'interval': 1,
            'normalize': True,
        })

        self.collector = CPUCollector(config, None)

        self.num_cpu = 2

        # first measurement
        self.input_base = {
            'user': 100,
            'nice': 200,
            'system': 300,
            'idle': 400,
        }
        # second measurement
        self.input_next = {
            'user': 110,
            'nice': 220,
            'system': 330,
            'idle': 440,
        }
        self.expected = {
            'cpu.total.user': 110,
            'cpu.total.nice': 220,
            'cpu.total.system': 330,
            'cpu.total.idle': 440,
        }
        self.expected2 = {
            'cpu.total.user': 55,
            'cpu.total.nice': 110,
            'cpu.total.system': 165,
            'cpu.total.idle': 220,
        }
    # convert an input dict with values to a string that might come from
    # /proc/stat
    def input_dict_to_proc_string(self, cpu_id, dict_):
        return ("cpu%s %i %i %i %i 0 0 0 0 0 0" %
                (cpu_id,
                 dict_['user'],
                 dict_['nice'],
                 dict_['system'],
                 dict_['idle'],
                 )
                )

    @patch.object(Collector, 'publish')
    def test_should_work_proc_stat(self, publish_mock):
        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            "\n".join([self.input_dict_to_proc_string('', self.input_base),
                       self.input_dict_to_proc_string('0', self.input_base),
                       self.input_dict_to_proc_string('1', self.input_base),
                       ])
        )))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        self.assertPublishedMany(publish_mock, {})

        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            "\n".join([self.input_dict_to_proc_string('', self.input_next),
                       self.input_dict_to_proc_string('0', self.input_next),
                       self.input_dict_to_proc_string('1', self.input_next),
                       ])
        )))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        self.assertPublishedMany(publish_mock, self.expected)

    @patch.object(Collector, 'publish')
    @patch('cpu.os')
    @patch('cpu.psutil')
    def test_should_work_psutil(self, psutil_mock, os_mock, publish_mock):

        os_mock.access.return_value = False

        total = Mock(**self.input_base)
        cpu_time = [Mock(**self.input_base),
                    Mock(**self.input_base),
                    ]
        psutil_mock.cpu_times.side_effect = [cpu_time, total]

        self.collector.collect()

        self.assertPublishedMany(publish_mock, {})

        total = Mock(**self.input_next)
        cpu_time = [Mock(**self.input_next),
                    Mock(**self.input_next),
                    ]
        psutil_mock.cpu_times.side_effect = [cpu_time, total]

        self.collector.collect()

        self.assertPublishedMany(publish_mock, self.expected2)


class TestCPUCollectorDimensions(CollectorTestCase):
    def setUp(self):
        config = get_collector_config('CPUCollector', {
            'interval': 10,
            'normalize': False
        })

        self.collector = CPUCollector(config, None)

    @patch.object(Collector, 'flush')
    def test_core_dimension_core_metrics(self, publish_mock):
        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu0 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        for metric_name in ['cpu.user', 'cpu.idle', 'cpu.nice', 'cpu.softirq']:
            metrics = find_metric(self.collector.payload, metric_name)

            self.assertEqual(len(metrics), 1)
            self.assertTrue(metrics[0]['dimensions'].has_key('core'))

    @patch.object(Collector, 'flush')
    def test_core_dimension_total_metrics(self, publish_mock):
        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        for metric_name in ['cpu.total.user', 'cpu.total.idle', 'cpu.total.nice', 'cpu.total.softirq']:
            metrics = find_metric(self.collector.payload, metric_name)

            self.assertEqual(len(metrics), 1)
            self.assertFalse(metrics[0]['dimensions'].has_key('core'))

    @patch.object(Collector, 'flush')
    def test_core_dimension_core_metrics_aggregated(self, publish_mock):
        self.collector.config['enableAggregation'] = True
        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu0 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        for metric_name in ['cpu.user_mode', 'cpu.idle', 'cpu.nice', 'cpu.irq_softirq']:
            metrics = find_metric(self.collector.payload, metric_name)

            self.assertEqual(len(metrics), 1)
            self.assertTrue(metrics[0]['dimensions'].has_key('core'))

    @patch.object(Collector, 'flush')
    def test_core_dimension_total_metrics_aggregated(self, publish_mock):
        self.collector.config['enableAggregation'] = True
        patch_open = patch('__builtin__.open', Mock(return_value=StringIO(
            'cpu 110 220 330 440 550 660 770 0 0 0')))

        patch_open.start()
        self.collector.collect()
        patch_open.stop()

        for metric_name in ['cpu.total.user_mode', 'cpu.total.idle', 'cpu.total.nice', 'cpu.total.irq_softirq']:
            metrics = find_metric(self.collector.payload, metric_name)

            self.assertEqual(len(metrics), 1)
            self.assertFalse(metrics[0]['dimensions'].has_key('core'))

################################################################################
if __name__ == "__main__":
    unittest.main()
