using System.Text.Json;
using System.Text.Json.Nodes;
using AGUI.Abstractions;

namespace AGUI.ClaudeManagedAgents;

internal sealed class UnsupportedContentException : Exception
{
    internal UnsupportedContentException() : base("This user message contains content that Claude Managed Agents cannot accept. Use image or document blocks with a supported source.") { }
}

internal static class ManagedAgentsContent
{
    internal static JsonArray Blocks(AGUIContent content)
    {
        var blocks = new JsonArray();
        foreach (var part in content)
        {
            if (part is AGUITextInputContent text)
            {
                blocks.Add(new JsonObject { ["type"] = "text", ["text"] = text.Text });
                continue;
            }
            if (part is not AGUIImageInputContent && part is not AGUIDocumentInputContent) throw new UnsupportedContentException();
            var media = (AGUIMediaInputContent)part;
            string? mime = media.Source switch
            {
                AGUIInputContentDataSource data => data.MimeType,
                AGUIInputContentUrlSource url => url.MimeType,
                AGUIInputContentFileSource file => file.MimeType,
                _ => null,
            };
            string[] allowed = part is AGUIImageInputContent ? ["image/png", "image/jpeg", "image/gif", "image/webp"] : ["application/pdf", "text/plain"];
            if (mime is not null && !allowed.Contains(mime, StringComparer.Ordinal)) throw new UnsupportedContentException();
            JsonObject source = media.Source switch
            {
                AGUIInputContentDataSource data => new() { ["type"] = "base64", ["data"] = data.Value, ["media_type"] = data.MimeType },
                AGUIInputContentUrlSource url => new() { ["type"] = "url", ["url"] = url.Value },
                AGUIInputContentFileSource file when file.Provider is null or "anthropic" => new() { ["type"] = "file", ["file_id"] = file.Value },
                _ => throw new UnsupportedContentException(),
            };
            var block = new JsonObject { ["type"] = part.Type, ["source"] = source };
            if (part is AGUIDocumentInputContent && media.Metadata is JsonElement metadata && metadata.ValueKind == JsonValueKind.Object && metadata.TryGetProperty("filename", out var filename) && filename.ValueKind == JsonValueKind.String)
                block["title"] = filename.GetString();
            blocks.Add(block);
        }
        return blocks;
    }

    internal static JsonArray ToolBlocks(AGUIToolMessage message)
    {
        if (message.Content.IsText)
            return new JsonArray(new JsonObject { ["type"] = "text", ["text"] = string.Join("\n", new[] { message.Content.ToString(), message.Error }.Where(p => !string.IsNullOrEmpty(p))) });
        var blocks = Blocks(message.Content);
        if (!string.IsNullOrEmpty(message.Error)) blocks.Add(new JsonObject { ["type"] = "text", ["text"] = message.Error });
        return blocks;
    }

    internal static AGUIContent ToolContent(IEnumerable<JsonElement>? content)
    {
        var blocks = content?.ToArray() ?? [];
        if (blocks.All(b => b.TryGetProperty("type", out var type) && type.GetString() == "text" && b.TryGetProperty("text", out var text) && text.ValueKind == JsonValueKind.String && b.EnumerateObject().All(p => p.Name is "type" or "text")))
            return string.Join("\n", blocks.Select(b => b.GetProperty("text").GetString()));
        var parts = new List<AGUIInputContent>();
        foreach (var block in blocks)
        {
            var kind = block.GetProperty("type").GetString();
            var metadata = JsonSerializer.SerializeToElement(new { claudeManagedAgents = block });
            AGUIInputContentSource? source = null;
            if (kind is "image" or "document" && block.TryGetProperty("source", out var s) && s.TryGetProperty("type", out var st))
                source = st.GetString() switch
                {
                    "base64" => new AGUIInputContentDataSource { Value = s.GetProperty("data").GetString()!, MimeType = s.GetProperty("media_type").GetString()! },
                    "url" => new AGUIInputContentUrlSource { Value = s.GetProperty("url").GetString()! },
                    "file" => new AGUIInputContentFileSource { Value = s.GetProperty("file_id").GetString()!, Provider = "anthropic" },
                    _ => null,
                };
            if (source is not null)
                parts.Add(kind == "image" ? new AGUIImageInputContent { Source = source, Metadata = metadata } : new AGUIDocumentInputContent { Source = source, Metadata = metadata });
            else
                parts.Add(new AGUITextInputContent { Text = kind == "text" ? block.GetProperty("text").GetString()! : block.GetRawText(), Metadata = metadata });
        }
        return new AGUIContent(parts);
    }
}
