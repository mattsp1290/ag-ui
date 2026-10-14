using System.Text.Json;
using AGUI.Abstractions;
using Xunit;

namespace AGUI.ClaudeManagedAgents.Tests;

public class ManagedAgentsContentTest
{
    private const string Idle = """{"type":"session.status_idle","id":"idle","stop_reason":{"type":"end_turn"}}""";

    private static async Task<(FakeManagedAgentsClient Fake, List<BaseEvent> Events, RunAgentInput Input)> RunAsync(AGUIContent content, params string[] native)
    {
        var fake = new FakeManagedAgentsClient([.. native, Idle]);
        var input = new RunAgentInput { ThreadId = "t", RunId = "r", State = JsonSerializer.SerializeToElement(new { cart = new { total = 42 } }), Messages = [new AGUIUserMessage { Id = "original-user", Content = content }], Tools = [] };
        var agent = new ManagedAgentsAgent(new ManagedAgentsAgentOptions { ManagedAgentId = "a", EnvironmentId = "e", Client = fake });
        var events = new List<BaseEvent>();
        await foreach (var item in agent.RunAsync(input)) events.Add(item);
        return (fake, events, input);
    }

    [Theory]
    [InlineData("image", "image/png")]
    [InlineData("document", "application/pdf")]
    public async Task PreservesSourcesAndOriginalInput(string kind, string mime)
    {
        AGUIMediaInputContent Part(AGUIInputContentSource source) => kind == "image"
            ? new AGUIImageInputContent { Source = source, Metadata = JsonSerializer.SerializeToElement(new { filename = "original.bin" }) }
            : new AGUIDocumentInputContent { Source = source, Metadata = JsonSerializer.SerializeToElement(new { filename = "original.bin" }) };
        var run = await RunAsync(new AGUIContent(new List<AGUIInputContent> {
            Part(new AGUIInputContentDataSource { Value = "AAEC/w==", MimeType = mime }),
            Part(new AGUIInputContentUrlSource { Value = "https://example.test/media", MimeType = mime }),
            Part(new AGUIInputContentFileSource { Value = "file_native", Provider = "anthropic", MimeType = mime }),
        }));
        Assert.Same(run.Input, Assert.IsType<RunStartedEvent>(run.Events[0]).Input);
        Assert.Equal(run.Input.State, Assert.IsType<StateSnapshotEvent>(run.Events[1]).Snapshot);
        var blocks = run.Fake.Sent[0][0].GetProperty("content");
        Assert.Equal("AAEC/w==", blocks[0].GetProperty("source").GetProperty("data").GetString());
        Assert.Equal(mime, blocks[0].GetProperty("source").GetProperty("media_type").GetString());
        Assert.Equal("https://example.test/media", blocks[1].GetProperty("source").GetProperty("url").GetString());
        Assert.Equal("file_native", blocks[2].GetProperty("source").GetProperty("file_id").GetString());
        Assert.IsType<RunFinishedEvent>(run.Events[^1]);
    }

    [Theory]
    [InlineData("audio")]
    [InlineData("video")]
    public async Task UnsupportedMediaIsRecordedBeforeError(string kind)
    {
        AGUIInputContentSource source = new AGUIInputContentDataSource { Value = "AAEC", MimeType = kind + "/test" };
        AGUIInputContent part = kind == "audio" ? new AGUIAudioInputContent { Source = source } : new AGUIVideoInputContent { Source = source };
        var run = await RunAsync(new AGUIContent(new List<AGUIInputContent> { part }));
        Assert.Same(run.Input, Assert.IsType<RunStartedEvent>(run.Events[0]).Input);
        Assert.Equal("unsupported_content", Assert.IsType<RunErrorEvent>(run.Events[^1]).Code);
        Assert.Empty(run.Fake.Sent);
        Assert.Empty(run.Fake.CreatedSessions);
    }

    [Fact]
    public async Task PreservesFullNativeResultWithOriginalIdentity()
    {
        var content = new object[] { new { type = "text", text = "  " + new string('x', 30000) + "\n" }, new { type = "image", source = new { type = "base64", data = "AAEC/w==", media_type = "image/png" } }, new { type = "search_result", title = "A &amp; B", source = "https://example.test", content = new[] {new { type = "text", text = new string('y', 2000) }} } };
        var native = JsonSerializer.Serialize(new { type = "agent.mcp_tool_result", id = "original-result", mcp_tool_use_id = "original-call", is_error = true, content });
        var run = await RunAsync("hello", native);
        var raw = Assert.Single(run.Events.OfType<RawEvent>());
        Assert.Equal("claude-managed-agents", raw.Source);
        Assert.True(JsonElement.DeepEquals(JsonSerializer.Deserialize<JsonElement>(native), raw.Event));
        var result = Assert.Single(run.Events.OfType<ToolCallResultEvent>());
        Assert.Equal("original-result", result.MessageId);
        Assert.Equal("original-call", result.ToolCallId);
        var preserved = result.Content.Select(p => (p is AGUIMediaInputContent media ? media.Metadata : ((AGUITextInputContent)p).Metadata)!.Value.GetProperty("claudeManagedAgents")).ToArray();
        Assert.True(JsonElement.DeepEquals(JsonSerializer.SerializeToElement(content), JsonSerializer.SerializeToElement(preserved)));
        Assert.Equal("AAEC/w==", Assert.IsType<AGUIInputContentDataSource>(Assert.IsType<AGUIImageInputContent>(result.Content[1]).Source).Value);
    }
}
