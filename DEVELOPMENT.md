# Working on fullerite

## Clone and run the tests

To clone fullerite and run its tests - you can run following command:

```
 git clone git@github.com:Yelp/fullerite
 cd fullerite
 make
```

## Running Fullerite locally

Running `make` command above should result in fullerite binary in `bin` directory.
You can run fullerite using `bin/fullerite` command.

However - typically fullerite also requires some configuration before it can
run properly. You can look into example configuration file in `examples/config/fullerite.conf.example`
and adjust it according to your needs.

A more shorter configuration is:

```
{
  "defaultDimensions": {
    "application": "fullerite",
    "host": "localhost"
  },
  "fulleritePort": 49191,
  "interval": 10,
  "prefix": "",
  "diamondCollectorsPath": "src/diamond/collectors",
  "diamondCollectors": [""] ,
  "collectorsConfigPath": "examples/config",
  "internalServer": {
    "port": "49090",
    "path": "/metrics"
  },
  "collectors": ["Diamond"],
  "handlers": {
    "Log": {}
  }
}
```

The most important bits here is `collectors` and `diamondCollectors` keys. We will talk about them
later in this document.

Once you have created the configuration file - lets save it in root of fullerite directory with the name `fullerite.conf`.

Now you can start fullerite with following command:

```
~> ./bin/fullerite -c fullerite.conf
```


## Adding a collector

A collector in fullerite can be written using `go` programming language
or can be written in `Python`.

### Writing a collector in Go

Depending on your needs - you may want to write your collector in `go`. A collector
must implement `Collector` interface as defined in `src/fullerite/collector/collector.go`.

Fullerite will call `Collect` method defined in your collector periodically to generate
metrics.

#### Testing your collector

First thing to make sure is - your collector has unit tests, after that typically
you also need to manually run the collector locally and ensure that it works as expected.

To run your collector, modify `fullerite.conf` file to include your collector name.
For example:

```
"collectors": ["NewCollector"]
```

You also need to create a configuration file for particular collector even if it is just `{}`. Assuming
you are following along with example configuration above - create a file called `NewCollector.conf`
in `examples/config/` directory.

And now you can go ahead and run fullerite as documented above.

### Adding a collector in Python

If you are already familiar with Python - it is very easy to write a collector in Python.
For this example - lets try to run CPU collector located at `diamond/collectors/cpu/cpu.py`.

First you need to modify `fullerite.conf` file created above and make sure that this collector
is enabled:

```
"diamondCollectors": [ "CPUCollector" ]
```

You also need to define a configuration file for this collector even if its contents are just `{}`.
If you are running from Fullerite clone - `examples/config` director should already contain `CPUCollector.conf`
file with `{}` as its contents.

After that you can start fullerite as documented above.

But in addition to that you also need to start diamond server:

```
~> python src/diamond/server.py -c fullerite.conf
```

This should get `CPUCollector` running locally.

#### Testing your Python collector.

You should write unit tests for your collector. The best way to learn about them is - read one
of the existing collector code.

Fullerite has inherited many Python collectors from Diamond because of its heritage. Unit test
of some of the collectors written in Python are not running successfully and hence tests of such
collectors are blacklisted from running. The blacklist is located in `src/diamond/blacklist_test.txt`.

You can run test of your individual collector via:

```
~> python src/diamond/test.py -c src/diamond/collectors/your_collector
```

If you are modifying a collector whose tests are blacklisted, it is a good idea to remove the name of the collector
test suite from blacklist file and make sure all its test pass before opening the pull request.

Sometimes unit tests are not enough to test a collector and in such cases - it is a good idea to manually run
the collector as documented above and make sure it is emitting metrics correctly.
