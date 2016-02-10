#!/usr/bin/python

import json
import sys

metrics = {}
dimensions = {"dim1": "val1"}

metrics['first'] = {
    "name": "example",
    "value": 2.0,
    "dimensions":dimensions,
    "metricType": "gauge"
}

metrics['second'] = {
    "name": "anotherExample",
    "value": 2.0,
    "dimensions":dimensions,
    "metricType": "cumcounter"
}

# Send one metric
sys.stdout.write(json.dumps(metrics['first']))

# Send them all
sys.stdout.write(json.dumps(metrics.values()))
