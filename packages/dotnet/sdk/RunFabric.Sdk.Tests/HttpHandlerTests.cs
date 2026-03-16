using System.Collections.Generic;
using System.IO;
using System.Text;
using System.Threading.Tasks;
using RunFabric.Sdk;
using Xunit;

namespace RunFabric.Sdk.Tests;

public class HttpHandlerTests
{
    [Fact]
    public async Task InvokeAsync_WritesJsonResponse()
    {
        Handler h = (event, context) => new Dictionary<string, object?>
        {
            ["ok"] = true,
            ["stage"] = context.Stage
        };
        var requestBody = new MemoryStream(Encoding.UTF8.GetBytes("{\"x\":1}"));
        var responseBody = new MemoryStream();
        await HttpHandler.InvokeAsync(requestBody, responseBody, h, "dev", "api", "req-1");
        responseBody.Position = 0;
        var json = await new StreamReader(responseBody).ReadToEndAsync();
        Assert.Contains("ok", json);
        Assert.Contains("dev", json);
    }
}
