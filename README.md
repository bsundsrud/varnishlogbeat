# Varnishlogbeat

Welcome to Varnishlogbeat. Based on the original
[varnishlogbeat by phenomenes](https://github.com/phenomenes/varnishlogbeat)
but updated for elastic 7.5.0. Reads log data from a Varnish Shared Memory
file and ships it to ELK.

## Getting Started with Varnishlogbeat

### Requirements

* [Golang](https://golang.org/dl/) 1.13.5
* varnish-dev 5.2+

### Build

To build the binary for Varnishlogbeat run the command below. This will generate
a binary in the same directory with the name varnishlogbeat.

```
make
```

### Run

To run Varnishlogbeat with debugging output enabled, run:

```
./varnishlogbeat -c varnishlogbeat.yml -e -d "*"
```

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for
the fields which is automatically generated based on `fields.yml` by running the
following command.

```
make update
```

### Cleanup

To clean  Varnishlogbeat source code, run the following command:

```
make fmt
```

To clean up the build directory and generated artifacts, run:

```
make clean
```
