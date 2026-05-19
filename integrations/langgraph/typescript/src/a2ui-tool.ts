/**
 * A2UI subagent tool factory for LangGraph TS agents.
 *
 * Ships a ready-to-bind LangGraph tool that delegates dynamic A2UI surface
 * generation to a secondary LLM call. The author imports the factory, passes
 * their chat model in, and binds the returned tool alongside their other tools.
 * No further A2UI-specific code is required on the author's side.
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

/**
 * Loose type for the subagent model.
 *
 * Typed as `any` (rather than `BaseChatModel`) to tolerate `@langchain/core` version
 * skew between this package and the consumer — e.g. `ChatOpenAI` shipping its own
 * peer-pinned core. The factory only needs `bindTools` + `invoke`, which is checked
 * at runtime.
 */
export type A2UISubagentModel = any;

/** Container key the A2UI middleware looks for in tool results. */
export const A2UI_OPERATIONS_KEY = "a2ui_operations";

/** Default catalog id used when the subagent does not specify one. */
export const BASIC_CATALOG_ID =
  "https://a2ui.org/specification/v0_9/basic_catalog.json";

type A2UIOperation = Record<string, unknown>;

function createSurface(surfaceId: string, catalogId: string): A2UIOperation {
  return {
    version: "v0.9",
    createSurface: { surfaceId, catalogId },
  };
}

function updateComponents(
  surfaceId: string,
  components: Array<Record<string, unknown>>,
): A2UIOperation {
  return {
    version: "v0.9",
    updateComponents: { surfaceId, components },
  };
}

function updateDataModel(
  surfaceId: string,
  data: unknown,
  path: string = "/",
): A2UIOperation {
  return {
    version: "v0.9",
    updateDataModel: { surfaceId, path, value: data },
  };
}

/**
 * Assemble the subagent prompt prefix from AG-UI context + schema in state.
 *
 * The LangGraph AG-UI integration extracts the A2UI component schema into
 * `state["ag-ui"]["a2ui_schema"]` and forwards any other context entries
 * (generation guidelines, design guidelines, etc.) under
 * `state["ag-ui"]["context"]`.
 */
function buildContextPrompt(state: Record<string, unknown>): string {
  const agUi = (state["ag-ui"] as Record<string, unknown> | undefined) ?? {};
  const parts: string[] = [];

  const contextEntries = (agUi.context as Array<Record<string, unknown>> | undefined) ?? [];
  for (const entry of contextEntries) {
    const desc = entry?.description as string | undefined;
    const value = entry?.value as string | undefined;
    if (desc) {
      parts.push(`## ${desc}\n${value ?? ""}\n`);
    } else if (value) {
      parts.push(`${value}\n`);
    }
  }

  const schema = agUi.a2ui_schema as string | undefined;
  if (schema) {
    parts.push(`## Available Components\n${schema}\n`);
  }

  return parts.join("\n");
}

interface PriorSurface {
  components: Array<Record<string, unknown>>;
  data: unknown;
  catalogId?: string;
}

/**
 * Locate the most recent rendered state for `surfaceId` in message history.
 *
 * Walks backwards through messages looking for a tool result whose content
 * is a JSON string containing `a2ui_operations` ops for the given surface.
 * Returns the reconstructed components + data + catalogId, or undefined if
 * no matching surface is found.
 */
function findPriorSurface(
  messages: Array<any>,
  surfaceId: string,
): PriorSurface | undefined {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (!msg) continue;
    const role = msg.type ?? msg.role;
    if (role !== "tool" && role !== "ToolMessage") continue;
    const content = msg.content;
    if (typeof content !== "string") continue;
    let parsed: unknown;
    try {
      parsed = JSON.parse(content);
    } catch {
      continue;
    }
    if (!parsed || typeof parsed !== "object") continue;
    const ops = (parsed as Record<string, unknown>)[A2UI_OPERATIONS_KEY];
    if (!Array.isArray(ops)) continue;

    let components: Array<Record<string, unknown>> | undefined;
    let data: unknown;
    let catalogId: string | undefined;
    let matched = false;

    for (const op of ops) {
      if (!op || typeof op !== "object") continue;
      const opObj = op as Record<string, unknown>;
      const cs = opObj.createSurface as Record<string, unknown> | undefined;
      if (cs && cs.surfaceId === surfaceId) {
        matched = true;
        if (typeof cs.catalogId === "string") catalogId = cs.catalogId;
      }
      const uc = opObj.updateComponents as Record<string, unknown> | undefined;
      if (uc && uc.surfaceId === surfaceId) {
        matched = true;
        if (Array.isArray(uc.components)) {
          components = uc.components as Array<Record<string, unknown>>;
        }
      }
      const ud = opObj.updateDataModel as Record<string, unknown> | undefined;
      if (ud && ud.surfaceId === surfaceId) {
        matched = true;
        data = ud.value;
      }
    }
    if (matched) {
      return { components: components ?? [], data, catalogId };
    }
  }
  return undefined;
}

const RENDER_A2UI_TOOL_DEF = {
  type: "function" as const,
  function: {
    name: "render_a2ui",
    description:
      "Render a dynamic A2UI v0.9 surface. The root component must have id 'root'. " +
      "Use components from the available catalog only.",
    parameters: {
      type: "object",
      properties: {
        surfaceId: {
          type: "string",
          description: "Unique surface identifier.",
        },
        catalogId: {
          type: "string",
          description: "The catalog id for the component catalog.",
        },
        components: {
          type: "array",
          description:
            "A2UI v0.9 component array (flat format). The root component must have id 'root'.",
          items: { type: "object" },
        },
        data: {
          type: "object",
          description:
            "Optional initial data model for the surface (form values, list items, etc.).",
        },
      },
      required: ["surfaceId", "components"],
    },
  },
};

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

      const promptParts: string[] = [buildContextPrompt(state)];
      if (compositionGuide) promptParts.push(compositionGuide);

      let prior: PriorSurface | undefined;
      if (isUpdate) {
        prior = findPriorSurface(messages, targetSurfaceId!);
        if (!prior) {
          return JSON.stringify({
            error:
              `intent='update' requested target_surface_id='${targetSurfaceId}' ` +
              `but no prior render of that surface was found in conversation history`,
          });
        }
        let editBlock =
          `## Editing an existing surface\n` +
          `You are editing surface '${targetSurfaceId}'. Produce the FULL ` +
          `updated components array and data model — not just a diff. Preserve ` +
          `component ids that the user has not asked to change so the renderer ` +
          `can reconcile them. Reuse the same catalogId.\n\n` +
          `### Previous components\n${JSON.stringify(prior.components, null, 2)}\n\n` +
          `### Previous data\n${JSON.stringify(prior.data, null, 2)}\n`;
        if (changes) {
          editBlock += `\n### Requested changes\n${changes}\n`;
        }
        promptParts.push(editBlock);
      }

      const prompt = promptParts.filter((p) => p && p.length > 0).join("\n");

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
      const catalogId =
        prior?.catalogId ||
        (args.catalogId as string) ||
        defaultCatalogId;
      const components =
        (args.components as Array<Record<string, unknown>>) || [];
      const data = (args.data as Record<string, unknown>) || {};

      const ops: A2UIOperation[] = [];
      if (!isUpdate) {
        ops.push(createSurface(surfaceId, catalogId));
      }
      ops.push(updateComponents(surfaceId, components));
      if (data && Object.keys(data).length > 0) {
        ops.push(updateDataModel(surfaceId, data));
      }

      return JSON.stringify({ [A2UI_OPERATIONS_KEY]: ops });
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
