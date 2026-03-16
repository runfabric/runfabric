namespace RunFabric.Sdk;

/// <summary>Request context passed to the handler (stage, function name, request ID).</summary>
public sealed class HandlerContext
{
    public string Stage { get; }
    public string? FunctionName { get; }
    public string? RequestId { get; }

    public HandlerContext(string? stage, string? functionName, string? requestId)
    {
        Stage = string.IsNullOrEmpty(stage) ? "dev" : stage;
        FunctionName = functionName;
        RequestId = requestId;
    }
}
