# Kinesis-Horizon

Using the following command to build `kinesis-horizon`.

To build your local docker image use this command:

```bash
export TAG=horizon224-119:locall_test
docker build -t $TAG . -f Dockerfile.kinesis > dockerprocess-test
```

## Release build

```bash
export VERSION="" # upstream version-kinesis.<patch-version> e.g 2.8.3-kinesis.2
export IMAGE="@abxit/kinesis-horizon:$VERSION"

docker build --build-arg VERSION=$VERSION -t $IMAGE . -f Dockerfile.kinesis
```

## To run tests

In vscode, open code as dev container. 
```
sudo service postgresql start
go test ./...
```
