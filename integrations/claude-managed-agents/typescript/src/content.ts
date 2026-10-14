/** Lossless content conversion. Original filenames/metadata remain in RUN_STARTED.input. */
import type { Message, ContentPart } from "@ag-ui/client";
import type { BetaManagedAgentsUserMessageEventParams } from "@anthropic-ai/sdk/resources/beta/sessions/events";

export class UnsupportedContentError extends Error {
  constructor() {
    super(
      "This user message contains content that Claude Managed Agents cannot accept. Use image or document blocks with a supported source.",
    );
  }
}

type Blocks = BetaManagedAgentsUserMessageEventParams["content"];
export const userContent = (
  message: Extract<Message, { role: "user" | "tool" }>,
): Blocks => {
  if (typeof message.content === "string")
    return message.content.trim()
      ? [{ type: "text", text: message.content }]
      : [];
  return (message.content ?? []).map((part): Blocks[number] => {
    if (part.type === "text") return { type: "text", text: part.text };
    if (part.type !== "image" && part.type !== "document")
      throw new UnsupportedContentError();
    const mime = part.source.mimeType;
    if (
      mime &&
      !(part.type === "image"
        ? ["image/png", "image/jpeg", "image/gif", "image/webp"].includes(mime)
        : ["application/pdf", "text/plain"].includes(mime))
    )
      throw new UnsupportedContentError();
    let source: Exclude<Blocks[number], { type: "text" }>["source"];
    if (
      part.source.type === "file" &&
      (!part.source.provider || part.source.provider === "anthropic")
    )
      source = { type: "file", file_id: part.source.value };
    else if (part.source.type === "url")
      source = { type: "url", url: part.source.value };
    else if (part.source.type === "data")
      source = {
        type: "base64",
        data: part.source.value,
        media_type: part.source.mimeType,
      };
    else throw new UnsupportedContentError();
    const filename =
      part.metadata &&
      typeof part.metadata === "object" &&
      "filename" in part.metadata &&
      typeof part.metadata.filename === "string"
        ? part.metadata.filename
        : undefined;
    return part.type === "image"
      ? { type: "image", source }
      : { type: "document", source, ...(filename ? { title: filename } : {}) };
  });
};

/** Preserve native rich blocks in standard AG-UI parts; RAW carries the exact event. */
export const toolContent = (
  blocks: ReadonlyArray<{ type: string; text?: string }> | null | undefined,
): string | ContentPart[] => {
  const content = blocks ?? [];
  if (
    content.every(
      (b) =>
        b.type === "text" &&
        typeof b.text === "string" &&
        Object.keys(b).every((k) => k === "type" || k === "text"),
    )
  )
    return content.map((b) => b.text).join("\n");
  return content.map((block): ContentPart => {
    const b = block as Record<string, any>;
    if (b.type === "text" && typeof b.text === "string")
      return {
        type: "text",
        text: b.text,
        metadata: { claudeManagedAgents: b },
      };
    if ((b.type === "image" || b.type === "document") && b.source) {
      const s = b.source;
      const source =
        s.type === "base64"
          ? { type: "data" as const, value: s.data, mimeType: s.media_type }
          : s.type === "url"
            ? { type: "url" as const, value: s.url }
            : s.type === "file"
              ? {
                  type: "file" as const,
                  value: s.file_id,
                  provider: "anthropic",
                }
              : undefined;
      if (source)
        return { type: b.type, source, metadata: { claudeManagedAgents: b } };
    }
    // Unknown/native-only block shapes stay visible and recoverable, never a label.
    return {
      type: "text",
      text: JSON.stringify(b),
      metadata: { claudeManagedAgents: b },
    };
  });
};
