/**
 * A2UI subagent tool factory for LangGraph TS agents.
 *
 * Thin adapter over ``@ag-ui/a2ui-toolkit`` — the heavy lifting (op builders,
 * prompt assembly, history walkers, output envelope) lives in the toolkit so
 * each new framework adapter (ADK, Mastra, Strands, …) only owns the
 * framework-specific glue: tool decorator, runtime state access, model
 * binding + invoke.
 *
 * Example usage in a chat node:
 *
 *   import { getA2UITools } from "@ag-ui/langgraph";
 *
 *   const a2ui = getA2UITools(new ChatOpenAI({ model: "gpt-4o" }));
 *
 *   const modelWithTools = chatModel.bindTools(
 *     [...state.tools, a2ui],
 *     { parallel_tool_calls: false },
 *   );
 */

import { tool, type ToolRuntime } from "@langchain/core/tools";
import { SystemMessage } from "@langchain/core/messages";
import {
  A2UI_OPERATIONS_KEY,
  BASIC_CATALOG_ID,
  RENDER_A2UI_TOOL_DEF,
  assembleOps,
  buildContextPrompt,
  buildSubagentPrompt,
  findPriorSurface,
  wrapAsOperationsEnvelope,
} from "@ag-ui/a2ui-toolkit";

/**
 * Loose type for the subagent model.
 *
 * Typed as `any` (rather than `BaseChatModel`) to tolerate `@langchain/core` version
 * skew between this package and the consumer — e.g. `ChatOpenAI` shipping its own
 * peer-pinned core. The factory only needs `bindTools` + `invoke`, which is checked
 * at runtime.
 */
export type A2UISubagentModel = any;

// Re-export the toolkit constants for callers that previously imported them
// from this package — keeps the public surface stable.
export { A2UI_OPERATIONS_KEY, BASIC_CATALOG_ID };

export interface A2UISubagentToolOptions {
  /** Optional extra rules appended to the subagent's system prompt. */
  compositionGuide?: string;
  /** Surface id used when the subagent omits `surfaceId`. */
  defaultSurfaceId?: string;
  /** Catalog id used when the subagent omits `catalogId`. */
  defaultCatalogId?: string;
  /** Name advertised to the main agent's planner. */
  toolName?: string;
  /** Description shown to the main agent's planner. */
  toolDescription?: string;
}

/** Tool arguments exposed to the main agent's planner. */
interface GenerateA2UIArgs {
  /**
   * `"create"` to render a new surface, `"update"` to modify a surface
   * previously rendered in this conversation. Defaults to `"create"`.
   */
  intent?: "create" | "update";
  /**
   * Required when `intent="update"`. The surface id of the prior render
   * to modify.
   */
  target_surface_id?: string;
  /** Optional natural-language description of the changes to apply on update. */
  changes?: string;
}

/**
 * Build a LangGraph tool that delegates A2UI surface generation to a subagent.
 *
 * The returned tool is ready to bind into a chat model alongside any other tools.
 *
 * @param model Chat model the subagent will invoke for structured A2UI output.
 *   Using the same provider/model as the main agent is fine.
 * @param options Optional behavior overrides.
 */
export function getA2UITools(
  model: A2UISubagentModel,
  options: A2UISubagentToolOptions = {},
) {
  const {
    compositionGuide,
    defaultSurfaceId = "dynamic-surface",
    defaultCatalogId = BASIC_CATALOG_ID,
    toolName = "generate_a2ui",
    toolDescription = "Generate or update a dynamic A2UI surface based on the conversation. " +
      "A secondary LLM designs the UI components and data. " +
      "Use intent='create' (default) when the user requests new visual content " +
      "(cards, forms, lists, dashboards, comparisons, etc.). " +
      "Use intent='update' with target_surface_id to modify a surface you " +
      "previously rendered (e.g. 'change the second card's price', " +
      "'add a Buy button', 'use red instead of blue').",
  } = options;

  return tool(
    async (
      input: GenerateA2UIArgs,
      runtime: ToolRuntime<Record<string, unknown>, unknown>,
    ): Promise<string> => {
      const state = runtime.state as Record<string, unknown>;
      const allMessages = (state.messages as Array<any>) ?? [];
      // Strip current (unbalanced) tool call from history.
      const messages = allMessages.slice(0, -1);

      const intent = input.intent ?? "create";
      const targetSurfaceId = input.target_surface_id;
      const changes = input.changes;
      const isUpdate = intent === "update" && Boolean(targetSurfaceId);

      const prior = isUpdate
        ? findPriorSurface(messages, targetSurfaceId!)
        : undefined;
      if (isUpdate && !prior) {
        return JSON.stringify({
          error:
            `intent='update' requested target_surface_id='${targetSurfaceId}' ` +
            `but no prior render of that surface was found in conversation history`,
        });
      }

      const prompt = buildSubagentPrompt({
        contextPrompt: buildContextPrompt(state),
        compositionGuide,
        editContext: prior
          ? { surfaceId: targetSurfaceId!, prior, changes }
          : undefined,
      });

      if (!model.bindTools) {
        return JSON.stringify({
          error: "Provided model does not support bindTools",
        });
      }

      const modelWithTool = model.bindTools([RENDER_A2UI_TOOL_DEF], {
        tool_choice: {
          type: "function",
          function: { name: "render_a2ui" },
        },
      });

      const response: any = await modelWithTool.invoke([
        new SystemMessage(prompt),
        ...messages,
      ] as any);

      const toolCalls: Array<{ args?: Record<string, unknown> }> =
        response.tool_calls ?? [];
      if (toolCalls.length === 0) {
        return JSON.stringify({ error: "LLM did not call render_a2ui" });
      }

      const args = toolCalls[0].args ?? {};
      const surfaceId = isUpdate
        ? (targetSurfaceId as string)
        : (args.surfaceId as string) || defaultSurfaceId;
      const catalogId = prior?.catalogId || defaultCatalogId;
      const components =
        (args.components as Array<Record<string, unknown>>) || [];
      const data = (args.data as Record<string, unknown>) || {};

      const ops = assembleOps({
        intent: isUpdate ? "update" : "create",
        surfaceId,
        catalogId,
        components,
        data,
      });

      return wrapAsOperationsEnvelope(ops);
    },
    {
      name: toolName,
      description: toolDescription,
      schema: {
        type: "object",
        properties: {
          intent: {
            type: "string",
            enum: ["create", "update"],
            description:
              "'create' to render a new surface; 'update' to modify a surface previously rendered in this conversation. Defaults to 'create'.",
          },
          target_surface_id: {
            type: "string",
            description:
              "Required when intent='update'. The surface id of the prior render to modify.",
          },
          changes: {
            type: "string",
            description:
              "Optional natural-language description of the changes to apply when intent='update'.",
          },
        },
      } as any,
    },
  );
}
