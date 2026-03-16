package dev.runfabric;

/**
 * Request context passed to the handler (stage, function name, request ID).
 */
public final class HandlerContext {
    private final String stage;
    private final String functionName;
    private final String requestId;

    public HandlerContext(String stage, String functionName, String requestId) {
        this.stage = stage != null ? stage : "dev";
        this.functionName = functionName;
        this.requestId = requestId;
    }

    public String getStage() {
        return stage;
    }

    public String getFunctionName() {
        return functionName;
    }

    public String getRequestId() {
        return requestId;
    }
}
