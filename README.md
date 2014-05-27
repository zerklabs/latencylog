latencylog
==========

Measures the time taken to establish a TCP connection to a remote host.

## Basic Usage:

```
latencylog --tcp-address 1.1.1.1:80
```

## Storing data with HTTP

This has the ability to POST each snapshot to an HTTP endpoint. The following JSON format is used:

```
{
  "duration": 0.00,
  "durationformat": "ms",
  "distanceestimate": "far || near || close",
  "start": time,
  "end": time,
  "host": "0.0.0.0",
  "port": 0,
  "localip": "0.0.0.0",
  "localport": 0
}
```

Internally, we use this with [http_to_logstash][http_to_logstash], but any HTTP endpoint should work (as long as it handles a POST). To specify the http parameters:

```
latencylog --tcp-address 1.1.1.1:80 --http-address 2.2.2.2 --http-path "/put"
```

It defaults to using the path /put if not overridden.



[http_to_logstash]: https://github.com/zerklabs/http_to_logstash
