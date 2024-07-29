# Deploy

## Development

Melange and APKO can be installed using make commands:

```bash
make melange apko
```

Images can be built with provided melange and apko make commands:

``` bash
make melange-build apko-build \
    IMAGE=ttl.sh/$(id -u -n)/local-artifact-mirror:24h
    VERSION=1.0.0
    MELANGE_CONFIG=deploy/packages/local-artifact-mirror/melange.tmpl.yaml
    APKO_CONFIG=deploy/images/local-artifact-mirror/apko.tmpl.yaml
```

*NOTE: On Docker Desktop, Settings > Advanced > "Allow the default Docker socket to be used" must be enabled.*
