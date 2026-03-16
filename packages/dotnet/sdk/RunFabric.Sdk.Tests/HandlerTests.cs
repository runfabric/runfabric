using RunFabric.Sdk;
using Xunit;

namespace RunFabric.Sdk.Tests;

public class HandlerTests
{
    [Fact]
    public void HandlerContext_DefaultsStageToDev()
    {
        var ctx = new HandlerContext(null, null, null);
        Assert.Equal("dev", ctx.Stage);
    }

    [Fact]
    public void Handler_ReturnsResponse()
    {
        Handler h = (event, context) => new Dictionary<string, object?>
        {
            ["message"] = "hello",
            ["stage"] = context.Stage
        };
        var ctx = new HandlerContext("dev", "api", "req-1");
        var result = h(new Dictionary<string, object?> { ["name"] = "world" }, ctx);
        Assert.Equal("hello", result["message"]);
        Assert.Equal("dev", result["stage"]);
    }
}
