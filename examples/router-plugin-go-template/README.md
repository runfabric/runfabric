# Router Plugin Go Template

Use this template to scaffold a `kind=router` plugin executable for RunFabric.

## 1) Copy template

```bash
mkdir -p my-router-plugin
cp -R examples/router-plugin-go-template/* my-router-plugin/
cd my-router-plugin
go mod init github.com/your-org/my-router-plugin
go mod tidy
```

## 2) Build binary

```bash
go build -o bin/my-router-plugin .
```

## 3) Register plugin

Add an extension manifest under `~/.runfabric/plugins/<id>/plugin.json` (or your configured plugin home):

```json
{
  "id": "my-router",
  "kind": "router",
  "name": "My Router Plugin",
  "version": "0.1.0",
  "executable": "/absolute/path/to/bin/my-router-plugin"
}
```

## 4) Select plugin in runfabric.yml

```yaml
extensions:
  routerPlugin: my-router
```
