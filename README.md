# CT Map/Reduce Tooling

## Installation

1. Install the python dependencies: `pip install -r python/requirements.txt`
1. Build the CT-to-Disk scraper: `go get github.com/jcjones/ct-mapreduce/cmd/ct-fetch`

## Configuration

1. Create a configuration file:

```
cat > ~/.ct-fetch.conf <<EOF
# Available directives:
#
# certPath = Path under which to store full DER-encoded certificates
# issuerCNFilter = Prefixes to match for CNs for permitted issuers, comma delimited
# runForever = Run forever, pausing `pollingDelay` between runs
# pollingDelayMean = Mean polling delay duration
# pollingDelayStdDev = A standard deviation, like 100, or 1000.
# logExpiredEntries = Add expired entries to the database
# numThreads = Use this many threads per CPU
# logList = URLs of the CT Logs, comma delimited
# cacheSize = Size of internal cache in entries, default is probably fine
#
# Examples
#
# Only accept certificates for Let's Encrypt's intermediates or the ISRG roots
issuerCNFilter = Let's Encrypt, ISRG
# Update the CT log list as you like, comma-delimited. Not currently tested with more than one log.
logList = https://ct.googleapis.com/icarus
# Choose if this should complete when it catches up to the CT logs, or be a daemon
runForever = false

# Optionally, a path with plenty of disk space if you want to save PEM files
# certPath = /ct

# Redis cache server
redisHost = 10.10.10.5:6379
redisTimeout = 2s

EOF
```

### Redis

You'll also need to configure your Redis instance with `maxmemory_policy:noeviction`, which is checked
programmatically and a warning will go to the logs if not set correctly.


### IAM for Google Cloud

Permissions that seem necessary are:
```
Cloud Datastore Owner
Cloud Datastore User
Monitoring Metric Writer
Cloud Memorystore Redis Editor
Storage Admin
Storage Object Admin
```

## Populating your storage and Redis with CT certificates

```
ct-fetch -config ~/.ct-fetch.conf
```
Note: Consider using `--offset X` to start from the `X`th log entry. Also, `--limit Y` will stop after
processing `Y` certificates.


## Tests

```
my_ip=$(ipconfig getifaddr en0) # macOS
docker run redis:4
# for some reason `docker run -p 6379:7000 redis:4 --port 7000` is needed for me

RedisHost=${my_ip}:6379 go test -v ./...
```

