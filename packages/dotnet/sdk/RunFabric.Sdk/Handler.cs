namespace RunFabric.Sdk;

/// <summary>RunFabric handler contract: (event, context) -> response dictionary.</summary>
/// <param name="event">Event payload (e.g. JSON body as dictionary).</param>
/// <param name="context">Request context (stage, function name, request ID).</param>
/// <returns>Response dictionary (serialized as JSON by the runtime).</returns>
public delegate IReadOnlyDictionary<string, object?> Handler(
    IReadOnlyDictionary<string, object?> event,
    HandlerContext context);
