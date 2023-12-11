# Migration

After adding `migrations/*.sql` script run `go-bindata` command to generate `bindata.go` and check-in the generated file.

```bash
go install github.com/kevinburke/go-bindata/go-bindata@v3.18.0+incompatible

$GOPATH/bin/go-bindata -nometadata -pkg schema -o bindata.go migrations/
```
