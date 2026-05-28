/**
 * @ag-ui/a2ui-toolkit
 *
 * Framework-agnostic building blocks for A2UI subagent tools. Each per-
 * framework adapter (LangGraph, ADK, Mastra, etc.) composes these helpers
 * with its framework-specific glue (tool decorator, runtime accessor, model
 * binding/invoke). Nothing in this package depends on any agent framework.
 */

/** Container key the A2UI middleware looks for in tool results. */
export const A2UI_OPERATIONS_KEY = "a2ui_operations";

/** Default catalog id used when the subagent does not specify one. */
export const BASIC_CATALOG_ID =
  "https://a2ui.org/specification/v0_9/basic_catalog.json";

/** A single A2UI v0.9 server-to-client operation. */
export type A2UIOperation = Record<string, unknown>;

// ---------------------------------------------------------------------------
// Op builders
// ---------------------------------------------------------------------------

export function createSurface(
  surfaceId: string,
  catalogId: string,
): A2UIOperation {
  return {
    version: "v0.9",
    createSurface: { surfaceId, catalogId },
  };
}

export function updateComponents(
  surfaceId: string,
  components: Array<Record<string, unknown>>,
): A2UIOperation {
  return {
    version: "v0.9",
    updateComponents: { surfaceId, components },
  };
}

export function updateDataModel(
  surfaceId: string,
  data: unknown,
  path: string = "/",
): A2UIOperation {
  return {
    version: "v0.9",
    updateDataModel: { surfaceId, path, value: data },
  };
}

// ---------------------------------------------------------------------------
// Inner render_a2ui tool definition
// ---------------------------------------------------------------------------

/**
 * JSON schema for the inner ``render_a2ui`` tool. Framework adapters bind
 * this on the subagent's model with ``tool_choice="render_a2ui"`` so the
 * structured-output call produces ``{surfaceId, components, data}``. The
 * catalog id is owned by the factory, not the subagent — the subagent can't
 * invent a catalog the host hasn't registered.
 */
export const RENDER_A2UI_TOOL_DEF = {
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

// ---------------------------------------------------------------------------
// State helpers
// ---------------------------------------------------------------------------

/**
 * Build the prompt prefix from AG-UI state context entries + the A2UI
 * component catalog. Framework integrations conventionally extract the
 * catalog into ``state["ag-ui"]["a2ui_schema"]`` and forward other context
 * entries (generation guidelines, design guidelines) under
 * ``state["ag-ui"]["context"]``.
 */
export function buildContextPrompt(state: Record<string, unknown>): string {
  const agUi = (state["ag-ui"] as Record<string, unknown> | undefined) ?? {};
  const parts: string[] = [];

  const contextEntries =
    (agUi.context as Array<Record<string, unknown>> | undefined) ?? [];
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

// ---------------------------------------------------------------------------
// Prior surface lookup (used for intent="update")
// ---------------------------------------------------------------------------

export interface PriorSurface {
  components: Array<Record<string, unknown>>;
  data: unknown;
  catalogId?: string;
}

/**
 * Locate the most recent rendered state for ``surfaceId`` in message history.
 *
 * Walks backwards looking for a tool result whose content is a JSON string
 * containing ``a2ui_operations`` for the given surface. Returns the
 * reconstructed ``{components, data, catalogId}``, or ``undefined`` if no
 * matching surface is found.
 */
export function findPriorSurface(
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

// ---------------------------------------------------------------------------
// Prompt assembly
// ---------------------------------------------------------------------------

export interface EditContext {
  surfaceId: string;
  prior: PriorSurface;
  changes?: string;
}

export interface BuildSubagentPromptInput {
  /** Output of ``buildContextPrompt(state)``. */
  contextPrompt: string;
  /** Project-specific composition rules to append. */
  compositionGuide?: string;
  /** When set, instructs the subagent to edit a prior surface in place. */
  editContext?: EditContext;
}

/**
 * Compose the full system prompt the subagent sees: context + catalog
 * (from ``contextPrompt``), optional project-specific composition guide,
 * and optional edit-existing-surface block.
 */
export function buildSubagentPrompt(input: BuildSubagentPromptInput): string {
  const parts: string[] = [];
  if (input.contextPrompt) parts.push(input.contextPrompt);
  if (input.compositionGuide) parts.push(input.compositionGuide);

  if (input.editContext) {
    const { surfaceId, prior, changes } = input.editContext;
    let editBlock =
      `## Editing an existing surface\n` +
      `You are editing surface '${surfaceId}'. Produce the FULL ` +
      `updated components array and data model — not just a diff. Preserve ` +
      `component ids that the user has not asked to change so the renderer ` +
      `can reconcile them. Reuse the same catalogId.\n\n` +
      `### Previous components\n${JSON.stringify(prior.components, null, 2)}\n\n` +
      `### Previous data\n${JSON.stringify(prior.data, null, 2)}\n`;
    if (changes) {
      editBlock += `\n### Requested changes\n${changes}\n`;
    }
    parts.push(editBlock);
  }

  return parts.filter((p) => p && p.length > 0).join("\n");
}

// ---------------------------------------------------------------------------
// Operations envelope
// ---------------------------------------------------------------------------

export interface AssembleOpsInput {
  /** ``"create"`` to render a new surface, ``"update"`` to modify a prior one. */
  intent: "create" | "update";
  surfaceId: string;
  catalogId: string;
  components: Array<Record<string, unknown>>;
  data?: Record<string, unknown>;
}

/**
 * Produce the final A2UI v0.9 operation list for a render result.
 *
 * ``create`` emits ``[createSurface, updateComponents, updateDataModel?]``.
 * ``update`` skips ``createSurface`` so the frontend reconciles the existing
 * surface in place instead of erroring (per v0.9 spec, ``createSurface`` on
 * an existing id is invalid).
 */
export function assembleOps(input: AssembleOpsInput): A2UIOperation[] {
  const ops: A2UIOperation[] = [];
  if (input.intent !== "update") {
    ops.push(createSurface(input.surfaceId, input.catalogId));
  }
  ops.push(updateComponents(input.surfaceId, input.components));
  if (input.data && Object.keys(input.data).length > 0) {
    ops.push(updateDataModel(input.surfaceId, input.data));
  }
  return ops;
}

/**
 * Wrap a list of A2UI operations as the JSON envelope the A2UI middleware
 * looks for in tool results.
 */
export function wrapAsOperationsEnvelope(ops: A2UIOperation[]): string {
  return JSON.stringify({ [A2UI_OPERATIONS_KEY]: ops });
}

/**
 * Wrap an error as the JSON string a subagent tool returns when it can't
 * produce a surface. Keeps the error shape consistent across frameworks.
 */
export function wrapErrorEnvelope(message: string): string {
  return JSON.stringify({ error: message });
}

// ---------------------------------------------------------------------------
// Subagent-tool defaults (shared so every framework adapter advertises the
// same planner-facing surface and behaviour)
// ---------------------------------------------------------------------------

/** Surface id used when the subagent omits ``surfaceId`` on a create. */
export const DEFAULT_SURFACE_ID = "dynamic-surface";

/** Default name the outer A2UI tool is advertised under to the main planner. */
export const GENERATE_A2UI_TOOL_NAME = "generate_a2ui";

/** Default description shown to the main agent's planner. */
export const GENERATE_A2UI_TOOL_DESCRIPTION =
  "Generate or update a dynamic A2UI surface based on the conversation. " +
  "A secondary LLM designs the UI components and data. " +
  "Use intent='create' (default) when the user requests new visual content " +
  "(cards, forms, lists, dashboards, comparisons, etc.). " +
  "Use intent='update' with target_surface_id to modify a surface you " +
  "previously rendered (e.g. 'change the second card's price', " +
  "'add a Buy button', 'use red instead of blue').";

/** Planner-facing descriptions for the outer tool's three arguments. */
export const GENERATE_A2UI_ARG_DESCRIPTIONS = {
  intent:
    "'create' to render a new surface; 'update' to modify a surface previously rendered in this conversation. Defaults to 'create'.",
  target_surface_id:
    "Required when intent='update'. The surface id of the prior render to modify.",
  changes:
    "Optional natural-language description of the changes to apply when intent='update'.",
} as const;

// ---------------------------------------------------------------------------
// High-level orchestration
//
// These two functions hold the entire create/update decision + prompt prep +
// result-assembly logic so every framework adapter is reduced to pure glue
// (tool decorator, state access, model bind+invoke, tool-call read).
// ---------------------------------------------------------------------------

export interface PrepareA2UIRequestInput {
  /** Raw ``intent`` arg from the planner (defaults to ``"create"``). */
  intent?: string;
  /** Raw ``target_surface_id`` arg from the planner. */
  targetSurfaceId?: string;
  /** Raw ``changes`` arg from the planner. */
  changes?: string;
  /** Conversation history with the current (unbalanced) tool call stripped. */
  messages: Array<any>;
  /** The agent's run state (read for context + catalog via buildContextPrompt). */
  state: Record<string, unknown>;
  /** Project-specific composition rules to append to the subagent prompt. */
  compositionGuide?: string;
}

export interface PreparedA2UIRequest {
  /** System prompt to feed the subagent. Empty string when ``error`` is set. */
  prompt: string;
  /** Whether this is an in-place edit of a prior surface. */
  isUpdate: boolean;
  /** The reconstructed prior surface, when editing. */
  prior?: PriorSurface;
  /** Set when the request is invalid (e.g. update with no matching surface). */
  error?: string;
}

/**
 * Resolve the create/update decision, locate any prior surface, and build the
 * subagent system prompt. Returns ``error`` instead of a prompt when the
 * request is invalid (update referencing a surface not in history).
 */
export function prepareA2UIRequest(
  input: PrepareA2UIRequestInput,
): PreparedA2UIRequest {
  const intent = input.intent ?? "create";
  const isUpdate = intent === "update" && Boolean(input.targetSurfaceId);

  const prior = isUpdate
    ? findPriorSurface(input.messages, input.targetSurfaceId!)
    : undefined;

  if (isUpdate && !prior) {
    return {
      prompt: "",
      isUpdate,
      error:
        `intent='update' requested target_surface_id='${input.targetSurfaceId}' ` +
        `but no prior render of that surface was found in conversation history`,
    };
  }

  const prompt = buildSubagentPrompt({
    contextPrompt: buildContextPrompt(input.state),
    compositionGuide: input.compositionGuide,
    editContext: prior
      ? { surfaceId: input.targetSurfaceId!, prior, changes: input.changes }
      : undefined,
  });

  return { prompt, isUpdate, prior };
}

export interface BuildA2UIEnvelopeInput {
  /** The subagent's ``render_a2ui`` structured-output args. */
  args: Record<string, unknown>;
  /** From ``prepareA2UIRequest``. */
  isUpdate: boolean;
  /** The planner's ``target_surface_id`` (used as the surface id on update). */
  targetSurfaceId?: string;
  /** The prior surface from ``prepareA2UIRequest`` (supplies the catalog id on update). */
  prior?: PriorSurface;
  /** Surface id used when the subagent omits one on create. */
  defaultSurfaceId?: string;
  /** Catalog id used when there's no prior surface to inherit one from. */
  defaultCatalogId?: string;
}

/**
 * Turn the subagent's structured output into the final operations envelope.
 *
 * Catalog ownership stays with the host: the subagent never picks a catalog,
 * so the id comes from the prior surface (update) or the configured default
 * (create) — never from the model's args.
 */
export function buildA2UIEnvelope(input: BuildA2UIEnvelopeInput): string {
  const surfaceId = input.isUpdate
    ? (input.targetSurfaceId as string)
    : (input.args.surfaceId as string) ||
      (input.defaultSurfaceId ?? DEFAULT_SURFACE_ID);

  const catalogId =
    input.prior?.catalogId || (input.defaultCatalogId ?? BASIC_CATALOG_ID);

  const components =
    (input.args.components as Array<Record<string, unknown>>) || [];
  const data = (input.args.data as Record<string, unknown>) || {};

  const ops = assembleOps({
    intent: input.isUpdate ? "update" : "create",
    surfaceId,
    catalogId,
    components,
    data,
  });

  return wrapAsOperationsEnvelope(ops);
}
