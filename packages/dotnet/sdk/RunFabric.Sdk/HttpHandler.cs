using System.Collections.Generic;
using System.IO;
using System.Text.Json;
using System.Threading.Tasks;

namespace RunFabric.Sdk;

/// <summary>Adapts a RunFabric Handler to HTTP request/response (e.g. for ASP.NET Core middleware).</summary>
public static class HttpHandler
{
    private static readonly JsonSerializerOptions JsonOptions = new() { PropertyNamingPolicy = JsonNamingPolicy.CamelCase };

    /// <summary>Process request body as JSON event, call handler, write JSON response to stream.</summary>
    public static async Task InvokeAsync(
        Stream requestBody,
        Stream responseBody,
        Handler handler,
        string? stage = null,
        string? functionName = null,
        string? requestId = null)
    {
        var ctx = new HandlerContext(stage, functionName, requestId);
        IReadOnlyDictionary<string, object?> eventData;
        try
        {
            using var reader = new StreamReader(requestBody);
            var json = await reader.ReadToEndAsync();
            var parsed = JsonSerializer.Deserialize<Dictionary<string, object?>>(json ?? "{}");
            eventData = parsed ?? new Dictionary<string, object?>();
        }
        catch
        {
            eventData = new Dictionary<string, object?>();
        }

        var result = handler(eventData, ctx);
        var outJson = JsonSerializer.Serialize(result ?? new Dictionary<string, object?>(), JsonOptions);
        await using var writer = new StreamWriter(responseBody, leaveOpen: true);
        await writer.WriteAsync(outJson);
    }
}
