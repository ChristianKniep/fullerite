# coding=utf-8

"""
This class collects data on NUMA Zone page stats

#### Dependencies

* /proc/zoneinfo

"""

import os
from re import compile as re_compile

import diamond.collector

node_re = re_compile(r'^Node\s+(?P<node>\d+),\s+zone\s+(?P<zone>\w+)$')


class NUMAZoneInfoCollector(diamond.collector.Collector):

    def get_default_config(self):
        """
        Returns the default collector settings
        """
        config = super(NUMAZoneInfoCollector, self).get_default_config()
        config.update({
            'path': '/proc/zoneinfo',
        })
        return config

    def collect(self):
        try:
            filepath = self.config['path']

            if not os.access(filepath, os.R_OK):
                self.log.error('Permission to access %s denied', filepath)
                return None

            file_handle = open(filepath, 'r')
            metric = ''
            numlines_to_process = 0

            for line in file_handle:
                match = node_re.match(line)

                if numlines_to_process > 0:
                    numlines_to_process -= 1
                    statname, metric_value = line.split('pages')[-1].split()
                    metric_name = ''.join([metric, statname])

                    self.publish(metric_name, metric_value)

                if match:
                    self.log.debug("Matched: %s %s" %
                                  (match.group('node'), match.group('zone')))

                    node = match.group('node') or ''
                    zone = match.group('zone') or ''
                    metric = "node{0!s}-zone-{0!s}-".format(node, zone)

                    # We get 4 lines afterwards for free, min, low, and high
                    # page thresholds
                    numlines_to_process = 4

            file_handle.close()
        except Exception as e:
            self.log.error('Failed because: %s' % str(e))
            return None
