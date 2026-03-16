# RunFabric SDK for Java

Handler contract and HTTP adapter for RunFabric functions in Java.

## Handler contract

```java
import dev.runfabric.Handler;
import dev.runfabric.HandlerContext;

Handler h = (event, context) -> {
    return Map.of("message", "hello", "stage", context.getStage());
};
```

## HTTP adapter

Use `HttpHandler` to wrap your handler for HTTP (e.g. servlet or programmatic request/response):

```java
import dev.runfabric.http.HttpHandler;

HttpHandler adapter = new HttpHandler(h);
adapter.handle(bodyReader, responseOut, "dev", "api", requestId);
```

## Build and test

```bash
mvn clean test
```
