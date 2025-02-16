# Kinesis-Horizon

Using the following command to build `kinesis-horizon`.

## XDR Generate

```bash
make xdr
```

## Docker
To build your local docker image use this command:

```bash
export TAG=kinesis-horizon:local
docker build -t $TAG . -f Dockerfile.kinesis
```

## Release build

```bash
export VERSION="" # upstream version-kinesis.<patch-version> e.g 2.8.3-kinesis.2
export IMAGE="@abxit/kinesis-horizon:$VERSION"

docker build --build-arg VERSION=$VERSION -t $IMAGE . -f Dockerfile.kinesis
```
