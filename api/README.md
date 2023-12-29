# Octopus Gateway

## Database migrations (golang-migrate)

```bash
migrate -source file://./migrations -database "postgres://user:password@host:port/database" up 2
```

## API

```bash
curl -X POST -H "Content-Type: application/json" -d '{"id":"myriad", "rpc":"http://...", "ws":"ws://..."}' host:port/chains
curl -X POST -H "Content-Type: application/json" -d '{"id":"oyster", "rpc":"http://...", "grpc":"grpc://..."}' host:port/chains
```
