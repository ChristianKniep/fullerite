#!/usr/bin/python
# coding=utf-8
################################################################################

from test import CollectorTestCase
from test import get_collector_config
from test import unittest
from mock import Mock
from mock import patch

from diamond.collector import Collector
from traceroute import TracerouteCollector

################################################################################


class TestTracerouteCollector(CollectorTestCase):
    def setUp(self):
        config = get_collector_config('TracerouteCollector', {
        })

        self.collector = TracerouteCollector(config, None)
        self.collector.config['bin'] = 'dummy'

    def test_import(self):
        self.assertTrue(TracerouteCollector)

    @patch.object(Collector, 'publish')
    def test_should_work_with_real_data(self, publish_mock):

        with patch('traceroute.Popen') as process_mock:
            with patch.object(process_mock.return_value, 'communicate') as comm_mock:
                comm_mock.return_value = [self.getFixture('traceroute').getvalue(), '']
                self.collector.collect()

        rtts = self.getFixture('rtts').getvalue().split('\n')
        for idx, call in enumerate(publish_mock.mock_calls):
            name, args, kwargs = call
            metric_name, metric_value = args

            self.assertEquals(metric_name, 'yelp.RoundTripTime')
            self.assertEquals(metric_value, float(rtts[idx]))

    @patch.object(Collector, 'publish_metric')
    def test_sent_dimensions(self, publish_metric_mock):

        with patch('traceroute.Popen') as process_mock:
            with patch.object(process_mock.return_value, 'communicate') as comm_mock:
                comm_mock.return_value = [self.getFixture('traceroute').getvalue(), '']
                self.collector.collect()

        hops = self.getFixture('hops').getvalue().split('\n')
        for idx, call in enumerate(publish_metric_mock.mock_calls):
            name, args, kwargs = call
            metric = args[0]
            hop, ip = hops[idx].strip().split('|')
            self.assertEquals(metric.dimensions, {
               'hop': hop,
               'ip': ip,
            })

    @patch.object(Collector, 'publish')
    def test_should_fail_gracefully(self, publish_mock):

        with patch('traceroute.Popen') as process_mock:
            with patch.object(process_mock.return_value, 'communicate') as comm_mock:
                with patch.object(self.collector.log, 'error') as error_logger:
                    comm_mock.return_value = [None, 'Failed to run collector']
                    self.collector.collect()

        error_logger.assert_called_once_with('Error running traceroute process')
        self.assertPublishedMany(publish_mock, {})



################################################################################
if __name__ == "__main__":
    unittest.main()
