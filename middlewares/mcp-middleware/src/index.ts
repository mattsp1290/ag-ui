import {
  Middleware,
  EventType,
  type AbstractAgent,
  type BaseEvent,
  type Message,
  type RunAgentInput,
  type Tool,
  type ToolCall,
  type ToolCallResultEvent,
} from "@ag-ui/client";
import { Observable, type Subscription } from "rxjs";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { SSEClientTransport } from "@modelcontextprotocol/sdk/client/sse.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

/**
 * MCP Client configuration for HTTP (streamable) transport.
 */
export interface MCPClientConfigHTTP {
  type: "http";
  url: string;
  headers?: Record<string, string>;
  serverId?: string;
}

/**
 * MCP Client configuration for SSE transport.
 */
export interface MCPClientConfigSSE {
  type: "sse";
  url: string;
  headers?: Record<string, string>;
  serverId?: string;
}

/**
 * MCP Client configuration — one of the supported transports.
 */
export type MCPClientConfig = MCPClientConfigHTTP | MCPClientConfigSSE;

/**
 * Maximum length of a tool name. Bounded by the strictest mainstream LLM
 * provider constraint (OpenAI function names: `^[a-zA-Z0-9_-]{1,64}$`),
 * which is also why `__` — not `:` or `/` — is used as the delimiter.
 */
export const MAX_TOOL_NAME_LENGTH = 64;

/**
 * The namespace prefix applied to every MCP-sourced tool. Mirrors the
 * Claude Agent SDK convention: `mcp__{server}__{tool}`.
 */
export const MCP_TOOL_NAME_PREFIX = "mcp";

/**
 * Default cap on the number of MCP tool-execution rounds in a single
 * `run()`. Prevents a runaway loop (and unbounded cost) if the model keeps
 * calling MCP tools forever.
 */
export const DEFAULT_MAX_ITERATIONS = 32;

/**
 * Options for {@link MCPMiddleware}.
 */
export interface MCPMiddlewareOptions {
  /**
   * Maximum number of MCP tool-execution rounds before the middleware stops
   * looping and lets the run finish. Defaults to {@link DEFAULT_MAX_ITERATIONS}.
   */
  maxIterations?: number;
}

/**
 * A tool resolved from an MCP server, carrying the metadata needed to map
 * the exposed (prefixed) name back to its origin. The mapping is kept as a
 * descriptor — never reconstructed by string-splitting the exposed name —
 * so server ids or tool names containing `__` can't corrupt the round-trip.
 */
export interface ResolvedMCPTool {
  /** The (prefixed, possibly truncated/deduped) tool exposed to the agent. */
  tool: Tool;
  /** The original tool name as reported by the MCP server. */
  originalName: string;
  /** The server this tool came from. */
  serverConfig: MCPClientConfig;
}

/**
 * Restrict a name segment to characters valid across LLM providers.
 */
function sanitizeSegment(segment: string): string {
  return segment.replace(/[^a-zA-Z0-9_-]/g, "_");
}

/**
 * Build a unique, length-bounded, namespaced tool name.
 *
 * Shape: `mcp__{serverId}__{toolName}` (sanitized), truncated to
 * {@link MAX_TOOL_NAME_LENGTH}. If the truncated name collides with one
 * already in `used`, a `_N` suffix is appended (and the base re-truncated to
 * make room) until unique.
 */
function makeUniqueToolName(
  serverId: string,
  toolName: string,
  used: Set<string>,
): string {
  const base = `${MCP_TOOL_NAME_PREFIX}__${sanitizeSegment(serverId)}__${sanitizeSegment(toolName)}`;
  let candidate = base.slice(0, MAX_TOOL_NAME_LENGTH);
  if (!used.has(candidate)) {
    return candidate;
  }
  for (let i = 1; ; i++) {
    const suffix = `_${i}`;
    candidate = base.slice(0, MAX_TOOL_NAME_LENGTH - suffix.length) + suffix;
    if (!used.has(candidate)) {
      return candidate;
    }
  }
}

/**
 * Collect assistant tool calls that have no corresponding `role: "tool"`
 * result message — i.e. the still-open tool calls.
 */
function getOpenToolCalls(messages: Message[]): ToolCall[] {
  const allToolCalls: ToolCall[] = [];
  for (const message of messages) {
    if (message.role === "assistant" && "toolCalls" in message && message.toolCalls) {
      allToolCalls.push(...message.toolCalls);
    }
  }
  const resolvedIds = new Set<string>();
  for (const message of messages) {
    if (message.role === "tool" && "toolCallId" in message) {
      resolvedIds.add(message.toolCallId);
    }
  }
  return allToolCalls.filter((tc) => !resolvedIds.has(tc.id));
}

/**
 * Extract text content from an MCP `callTool` result, falling back to a JSON
 * stringification of the content when it isn't plain text.
 */
function extractTextContent(mcpResult: unknown): string {
  const result = mcpResult as { content?: unknown };
  if (Array.isArray(result.content)) {
    const text = result.content
      .filter(
        (c): c is { type: "text"; text: string } =>
          !!c &&
          typeof c === "object" &&
          (c as { type?: unknown }).type === "text" &&
          typeof (c as { text?: unknown }).text === "string",
      )
      .map((c) => c.text)
      .join("\n");
    return text || JSON.stringify(result.content);
  }
  return JSON.stringify(result.content ?? result);
}

/**
 * AG-UI middleware that lists tools from one or more MCP servers, injects
 * them into the agent run (namespaced as `mcp__{server}__{tool}`), and
 * executes the resulting MCP tool calls server-side.
 *
 * Loop, on each agent `RUN_FINISHED`:
 *   - Find open tool calls (assistant calls without a result message).
 *   - Of those, execute the ones that target our injected MCP tools and emit
 *     a `TOOL_CALL_RESULT` for each.
 *   - If no open tool calls remain afterwards, start another run with the new
 *     result messages appended (same threadId, fresh runId).
 *   - If open tool calls still remain (e.g. frontend tools), stop and let the
 *     frontend resolve them.
 *
 * If a run produces no open tool calls targeting our MCP tools, the
 * middleware does not interfere at all — every event is forwarded verbatim.
 */
export class MCPMiddleware extends Middleware {
  private readonly mcpServers: MCPClientConfig[];
  private readonly maxIterations: number;

  constructor(
    mcpServers: MCPClientConfig[] = [],
    options: MCPMiddlewareOptions = {},
  ) {
    super();
    this.mcpServers = mcpServers;
    this.maxIterations = options.maxIterations ?? DEFAULT_MAX_ITERATIONS;
  }

  run(input: RunAgentInput, next: AbstractAgent): Observable<BaseEvent> {
    if (this.mcpServers.length === 0) {
      return this.runNext(input, next);
    }

    return new Observable<BaseEvent>((subscriber) => {
      let cancelled = false;
      let activeSub: Subscription | undefined;
      // Number of MCP tool-execution rounds performed so far in this run.
      let toolRounds = 0;

      // Run the agent once; on completion decide whether to execute MCP tool
      // calls and loop. `toolMap` (exposed name -> origin) is built once and
      // reused across iterations.
      const runOnce = (
        runInput: RunAgentInput,
        toolMap: Map<string, ResolvedMCPTool>,
      ): void => {
        let latestMessages: Message[] = runInput.messages;
        let errored = false;

        activeSub = this.runNextWithState(runInput, next).subscribe({
          next: ({ event, messages }) => {
            latestMessages = messages;
            if (event.type === EventType.RUN_ERROR) {
              errored = true;
            }
            subscriber.next(event); // forward every event verbatim
          },
          error: (err) => subscriber.error(err),
          complete: () => {
            void onRunComplete(runInput, latestMessages, toolMap, errored);
          },
        });
      };

      const onRunComplete = async (
        runInput: RunAgentInput,
        messages: Message[],
        toolMap: Map<string, ResolvedMCPTool>,
        errored: boolean,
      ): Promise<void> => {
        if (cancelled) return;

        // The run errored — do not execute tools or loop; the RUN_ERROR has
        // already been forwarded.
        if (errored) {
          subscriber.complete();
          return;
        }

        const openCalls = getOpenToolCalls(messages);
        const ourCalls = openCalls.filter((tc) => toolMap.has(tc.function.name));

        // Nothing for us — do not interfere; the run is finished.
        if (ourCalls.length === 0) {
          subscriber.complete();
          return;
        }

        // Runaway guard: refuse to execute beyond the iteration cap.
        if (toolRounds >= this.maxIterations) {
          console.warn(
            `[MCPMiddleware] Reached maxIterations (${this.maxIterations}); ` +
              `leaving ${ourCalls.length} MCP tool call(s) unexecuted.`,
          );
          subscriber.complete();
          return;
        }
        toolRounds++;

        // Execute our MCP tool calls (in parallel), then emit results in
        // their original order so message ordering is deterministic.
        const executed = await Promise.all(
          ourCalls.map(async (tc) => {
            const resolved = toolMap.get(tc.function.name)!;
            const content = await this.executeToolCall(resolved, tc);
            return { tc, content };
          }),
        );
        if (cancelled) return;

        const resultMessages: Message[] = [];
        for (const { tc, content } of executed) {
          const messageId = crypto.randomUUID();
          const resultEvent: ToolCallResultEvent = {
            type: EventType.TOOL_CALL_RESULT,
            messageId,
            toolCallId: tc.id,
            content,
            role: "tool",
          };
          subscriber.next(resultEvent);
          resultMessages.push({
            id: messageId,
            role: "tool",
            content,
            toolCallId: tc.id,
          });
        }

        const updatedMessages = [...messages, ...resultMessages];

        // Scenario 2: other (e.g. frontend) tool calls are still open — stop
        // and let the frontend take over.
        if (getOpenToolCalls(updatedMessages).length > 0) {
          subscriber.complete();
          return;
        }

        // Scenario 1: everything is resolved — run again with the results.
        runOnce(
          { ...runInput, runId: crypto.randomUUID(), messages: updatedMessages },
          toolMap,
        );
      };

      // Bootstrap: list tools once, inject, run.
      void (async () => {
        try {
          const resolved = await this.resolveTools(
            new Set(input.tools.map((t) => t.name)),
          );
          if (cancelled) return;
          const toolMap = new Map<string, ResolvedMCPTool>(
            resolved.map((r) => [r.tool.name, r]),
          );
          runOnce(
            { ...input, tools: [...input.tools, ...resolved.map((r) => r.tool)] },
            toolMap,
          );
        } catch (err) {
          subscriber.error(err);
        }
      })();

      return () => {
        cancelled = true;
        activeSub?.unsubscribe();
      };
    });
  }

  /**
   * Connect to each configured server, list its tools, and return them as
   * namespaced, deduped {@link ResolvedMCPTool}s. A server that fails to
   * connect or list is logged and skipped — one bad server never blocks the
   * run or the other servers' tools.
   */
  private async resolveTools(
    existingNames: Set<string>,
  ): Promise<ResolvedMCPTool[]> {
    const used = new Set(existingNames);
    const resolved: ResolvedMCPTool[] = [];

    let index = 0;
    for (const serverConfig of this.mcpServers) {
      const serverId = serverConfig.serverId ?? `server${index}`;
      index++;

      let client: Client | undefined;
      try {
        client = await this.connect(serverConfig);
        const { tools } = await client.listTools();
        for (const mcpTool of tools) {
          const name = makeUniqueToolName(serverId, mcpTool.name, used);
          used.add(name);
          resolved.push({
            tool: {
              name,
              description: mcpTool.description ?? "",
              parameters: mcpTool.inputSchema ?? {
                type: "object",
                properties: {},
              },
            },
            originalName: mcpTool.name,
            serverConfig,
          });
        }
      } catch (error) {
        console.error(
          `[MCPMiddleware] Failed to list tools from MCP server ${serverConfig.url}:`,
          error,
        );
      } finally {
        await client?.close();
      }
    }

    return resolved;
  }

  /**
   * Execute a single MCP tool call against its origin server and return the
   * result as text. Errors are caught and returned as the result content so
   * the agentic loop can react rather than crash.
   */
  private async executeToolCall(
    resolved: ResolvedMCPTool,
    toolCall: ToolCall,
  ): Promise<string> {
    let args: Record<string, unknown> = {};
    try {
      args = toolCall.function.arguments
        ? (JSON.parse(toolCall.function.arguments) as Record<string, unknown>)
        : {};
    } catch {
      // Leave args empty if the model emitted malformed JSON.
    }

    let client: Client | undefined;
    try {
      client = await this.connect(resolved.serverConfig);
      const result = await client.callTool({
        name: resolved.originalName,
        arguments: args,
      });
      return extractTextContent(result);
    } catch (error) {
      return `Error executing tool ${resolved.originalName}: ${String(error)}`;
    } finally {
      await client?.close();
    }
  }

  /**
   * Open a connected MCP client for a server config.
   */
  private async connect(serverConfig: MCPClientConfig): Promise<Client> {
    const transport =
      serverConfig.type === "sse"
        ? new SSEClientTransport(new URL(serverConfig.url))
        : new StreamableHTTPClientTransport(new URL(serverConfig.url));
    const client = new Client({
      name: "ag-ui-mcp-middleware",
      version: "0.0.1",
    });
    await client.connect(transport);
    return client;
  }
}
