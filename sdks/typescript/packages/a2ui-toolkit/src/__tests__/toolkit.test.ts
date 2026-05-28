import { describe, it, expect } from "vitest";
import {
  A2UI_OPERATIONS_KEY,
  BASIC_CATALOG_ID,
  DEFAULT_SURFACE_ID,
  RENDER_A2UI_TOOL_DEF,
  assembleOps,
  buildA2UIEnvelope,
  buildContextPrompt,
  buildSubagentPrompt,
  createSurface,
  findPriorSurface,
  prepareA2UIRequest,
  updateComponents,
  updateDataModel,
  wrapAsOperationsEnvelope,
  wrapErrorEnvelope,
} from "../index";

describe("constants", () => {
  it("A2UI_OPERATIONS_KEY is the wire key the middleware looks for", () => {
    expect(A2UI_OPERATIONS_KEY).toBe("a2ui_operations");
  });

  it("BASIC_CATALOG_ID points at the v0.9 basic catalog", () => {
    expect(BASIC_CATALOG_ID).toBe(
      "https://a2ui.org/specification/v0_9/basic_catalog.json",
    );
  });
});

describe("RENDER_A2UI_TOOL_DEF", () => {
  it("is shaped as an OpenAI function-call tool definition", () => {
    expect(RENDER_A2UI_TOOL_DEF.type).toBe("function");
    expect(RENDER_A2UI_TOOL_DEF.function.name).toBe("render_a2ui");
  });

  it("requires surfaceId and components", () => {
    expect(RENDER_A2UI_TOOL_DEF.function.parameters.required).toEqual([
      "surfaceId",
      "components",
    ]);
  });

  it("declares the three expected parameter slots", () => {
    expect(
      Object.keys(RENDER_A2UI_TOOL_DEF.function.parameters.properties),
    ).toEqual(["surfaceId", "components", "data"]);
  });
});

describe("op builders", () => {
  it("createSurface emits a v0.9 createSurface op", () => {
    expect(createSurface("s1", "c1")).toEqual({
      version: "v0.9",
      createSurface: { surfaceId: "s1", catalogId: "c1" },
    });
  });

  it("updateComponents wraps the component array verbatim", () => {
    const comps = [{ id: "root", component: "Row" }];
    expect(updateComponents("s1", comps)).toEqual({
      version: "v0.9",
      updateComponents: { surfaceId: "s1", components: comps },
    });
  });

  it("updateDataModel defaults path to /", () => {
    expect(updateDataModel("s1", { items: [] })).toEqual({
      version: "v0.9",
      updateDataModel: { surfaceId: "s1", path: "/", value: { items: [] } },
    });
  });

  it("updateDataModel honors a custom path", () => {
    expect(updateDataModel("s1", "hello", "/title")).toEqual({
      version: "v0.9",
      updateDataModel: { surfaceId: "s1", path: "/title", value: "hello" },
    });
  });
});

describe("buildContextPrompt", () => {
  it("returns empty when state has no ag-ui slot", () => {
    expect(buildContextPrompt({})).toBe("");
  });

  it("emits described context entries as markdown sections", () => {
    const prompt = buildContextPrompt({
      "ag-ui": {
        context: [{ description: "Style guide", value: "use cards" }],
      },
    });
    expect(prompt).toContain("## Style guide");
    expect(prompt).toContain("use cards");
  });

  it("includes value-only entries without a heading", () => {
    const prompt = buildContextPrompt({
      "ag-ui": { context: [{ value: "free-form note" }] },
    });
    expect(prompt).toContain("free-form note");
    expect(prompt).not.toContain("##");
  });

  it("appends the a2ui component catalog under Available Components", () => {
    const prompt = buildContextPrompt({
      "ag-ui": { a2ui_schema: "<catalog json>" },
    });
    expect(prompt).toContain("## Available Components");
    expect(prompt).toContain("<catalog json>");
  });

  it("ignores entries without description or value", () => {
    const prompt = buildContextPrompt({
      "ag-ui": { context: [{}] },
    });
    expect(prompt).toBe("");
  });
});

describe("findPriorSurface", () => {
  function toolMsg(content: unknown) {
    return { role: "tool", content: JSON.stringify(content) };
  }

  it("returns undefined when the surface is not present", () => {
    const messages = [toolMsg({ [A2UI_OPERATIONS_KEY]: [] })];
    expect(findPriorSurface(messages, "missing")).toBeUndefined();
  });

  it("returns the most recent rendered state when found", () => {
    const messages = [
      toolMsg({
        [A2UI_OPERATIONS_KEY]: [
          createSurface("s1", "cat://x"),
          updateComponents("s1", [{ id: "root", component: "Row" }]),
          updateDataModel("s1", { items: [1, 2] }),
        ],
      }),
    ];
    expect(findPriorSurface(messages, "s1")).toEqual({
      components: [{ id: "root", component: "Row" }],
      data: { items: [1, 2] },
      catalogId: "cat://x",
    });
  });

  it("prefers the latest matching tool result when multiple exist", () => {
    const messages = [
      toolMsg({
        [A2UI_OPERATIONS_KEY]: [
          createSurface("s1", "old-cat"),
          updateComponents("s1", [{ id: "root", component: "Row" }]),
        ],
      }),
      toolMsg({
        [A2UI_OPERATIONS_KEY]: [
          updateComponents("s1", [{ id: "root", component: "Column" }]),
          updateDataModel("s1", { changed: true }),
        ],
      }),
    ];
    const prior = findPriorSurface(messages, "s1");
    expect(prior?.components).toEqual([{ id: "root", component: "Column" }]);
    expect(prior?.data).toEqual({ changed: true });
  });

  it("ignores non-tool messages and unparseable content", () => {
    const messages = [
      { role: "assistant", content: "not a tool" },
      { role: "tool", content: "not json" },
      toolMsg({ unrelated: "payload" }),
    ];
    expect(findPriorSurface(messages, "s1")).toBeUndefined();
  });

  it("accepts ToolMessage's `type` field as well as `role`", () => {
    const messages = [
      {
        type: "tool",
        content: JSON.stringify({
          [A2UI_OPERATIONS_KEY]: [
            createSurface("s1", "c"),
            updateComponents("s1", [{ id: "root", component: "Row" }]),
          ],
        }),
      },
    ];
    expect(findPriorSurface(messages, "s1")?.catalogId).toBe("c");
  });
});

describe("buildSubagentPrompt", () => {
  it("returns the context prompt verbatim when no extras", () => {
    expect(buildSubagentPrompt({ contextPrompt: "ctx" })).toBe("ctx");
  });

  it("appends composition guide after the context prompt", () => {
    const prompt = buildSubagentPrompt({
      contextPrompt: "ctx",
      compositionGuide: "guide",
    });
    expect(prompt).toBe("ctx\nguide");
  });

  it("emits an edit block carrying the prior surface state", () => {
    const prompt = buildSubagentPrompt({
      contextPrompt: "ctx",
      editContext: {
        surfaceId: "s1",
        prior: { components: [{ id: "root", component: "Row" }], data: { x: 1 } },
        changes: "make the title bigger",
      },
    });
    expect(prompt).toContain("Editing an existing surface");
    expect(prompt).toContain("'s1'");
    expect(prompt).toContain('"id": "root"');
    expect(prompt).toContain('"x": 1');
    expect(prompt).toContain("Requested changes");
    expect(prompt).toContain("make the title bigger");
  });

  it("omits the requested-changes section when changes is missing", () => {
    const prompt = buildSubagentPrompt({
      contextPrompt: "ctx",
      editContext: {
        surfaceId: "s1",
        prior: { components: [], data: null },
      },
    });
    expect(prompt).not.toContain("Requested changes");
  });

  it("drops empty parts from the join", () => {
    expect(buildSubagentPrompt({ contextPrompt: "" })).toBe("");
  });
});

describe("assembleOps", () => {
  it("create intent emits createSurface + updateComponents + updateDataModel", () => {
    const ops = assembleOps({
      intent: "create",
      surfaceId: "s1",
      catalogId: "cat://x",
      components: [{ id: "root", component: "Row" }],
      data: { items: ["a"] },
    });
    expect(ops).toHaveLength(3);
    expect(ops[0]).toHaveProperty("createSurface");
    expect(ops[1]).toHaveProperty("updateComponents");
    expect(ops[2]).toHaveProperty("updateDataModel");
  });

  it("update intent skips createSurface so the frontend reconciles in place", () => {
    const ops = assembleOps({
      intent: "update",
      surfaceId: "s1",
      catalogId: "cat://x",
      components: [{ id: "root", component: "Row" }],
      data: { items: ["a"] },
    });
    expect(ops).toHaveLength(2);
    expect(ops[0]).toHaveProperty("updateComponents");
    expect(ops[1]).toHaveProperty("updateDataModel");
  });

  it("omits updateDataModel when no data is provided", () => {
    const ops = assembleOps({
      intent: "create",
      surfaceId: "s1",
      catalogId: "cat://x",
      components: [{ id: "root", component: "Row" }],
    });
    expect(ops).toHaveLength(2);
    expect(ops[0]).toHaveProperty("createSurface");
    expect(ops[1]).toHaveProperty("updateComponents");
  });

  it("omits updateDataModel when data is an empty object", () => {
    const ops = assembleOps({
      intent: "create",
      surfaceId: "s1",
      catalogId: "cat://x",
      components: [{ id: "root", component: "Row" }],
      data: {},
    });
    expect(ops).toHaveLength(2);
  });
});

describe("wrapAsOperationsEnvelope", () => {
  it("serializes ops under the A2UI_OPERATIONS_KEY", () => {
    const ops = [createSurface("s1", "c")];
    const envelope = JSON.parse(wrapAsOperationsEnvelope(ops));
    expect(envelope).toEqual({ [A2UI_OPERATIONS_KEY]: ops });
  });

  it("handles an empty ops list", () => {
    expect(JSON.parse(wrapAsOperationsEnvelope([]))).toEqual({
      [A2UI_OPERATIONS_KEY]: [],
    });
  });
});

describe("wrapErrorEnvelope", () => {
  it("wraps a message under the error key", () => {
    expect(JSON.parse(wrapErrorEnvelope("boom"))).toEqual({ error: "boom" });
  });
});

// A prior surface encoded the way it appears in conversation history.
function priorSurfaceMessage(surfaceId: string) {
  return {
    type: "tool",
    content: wrapAsOperationsEnvelope([
      createSurface(surfaceId, "cat://x"),
      updateComponents(surfaceId, [{ id: "root", component: "Row" }]),
      updateDataModel(surfaceId, { items: [1, 2] }),
    ]),
  };
}

describe("prepareA2UIRequest", () => {
  it("create: builds a prompt, no prior, not an update", () => {
    const prep = prepareA2UIRequest({
      intent: "create",
      messages: [],
      state: { "ag-ui": { context: [{ value: "ctx" }] } },
      compositionGuide: "guide",
    });
    expect(prep.error).toBeUndefined();
    expect(prep.isUpdate).toBe(false);
    expect(prep.prior).toBeUndefined();
    expect(prep.prompt).toContain("ctx");
    expect(prep.prompt).toContain("guide");
  });

  it("defaults a missing intent to create", () => {
    const prep = prepareA2UIRequest({ messages: [], state: {} });
    expect(prep.isUpdate).toBe(false);
    expect(prep.error).toBeUndefined();
  });

  it("update with a matching prior surface: edit prompt + prior populated", () => {
    const prep = prepareA2UIRequest({
      intent: "update",
      targetSurfaceId: "s1",
      changes: "make it red",
      messages: [priorSurfaceMessage("s1")],
      state: {},
    });
    expect(prep.error).toBeUndefined();
    expect(prep.isUpdate).toBe(true);
    expect(prep.prior?.catalogId).toBe("cat://x");
    expect(prep.prompt).toContain("Editing an existing surface");
    expect(prep.prompt).toContain("make it red");
  });

  it("update with no matching prior: returns an error, no prompt", () => {
    const prep = prepareA2UIRequest({
      intent: "update",
      targetSurfaceId: "missing",
      messages: [priorSurfaceMessage("s1")],
      state: {},
    });
    expect(prep.prompt).toBe("");
    expect(prep.error).toContain("missing");
    expect(prep.error).toContain("no prior render");
  });
});

describe("buildA2UIEnvelope", () => {
  it("create: createSurface uses the configured default catalog, not the args", () => {
    const env = JSON.parse(
      buildA2UIEnvelope({
        args: { surfaceId: "from-args", components: [{ id: "root", component: "Row" }], data: { items: [1] } },
        isUpdate: false,
        defaultCatalogId: "cat://configured",
      }),
    );
    const ops = env[A2UI_OPERATIONS_KEY];
    expect(ops[0].createSurface).toEqual({ surfaceId: "from-args", catalogId: "cat://configured" });
    expect(ops[1].updateComponents.components).toEqual([{ id: "root", component: "Row" }]);
    expect(ops[2].updateDataModel.value).toEqual({ items: [1] });
  });

  it("create: falls back to DEFAULT_SURFACE_ID when args omit surfaceId", () => {
    const env = JSON.parse(
      buildA2UIEnvelope({ args: { components: [] }, isUpdate: false }),
    );
    expect(env[A2UI_OPERATIONS_KEY][0].createSurface.surfaceId).toBe(DEFAULT_SURFACE_ID);
  });

  it("update: skips createSurface, keeps target id + prior catalog", () => {
    const env = JSON.parse(
      buildA2UIEnvelope({
        args: { surfaceId: "ignored", components: [{ id: "root", component: "Column" }] },
        isUpdate: true,
        targetSurfaceId: "s1",
        prior: { components: [], data: null, catalogId: "cat://prior" },
      }),
    );
    const ops = env[A2UI_OPERATIONS_KEY];
    expect(ops.some((o: any) => o.createSurface)).toBe(false);
    expect(ops[0].updateComponents.surfaceId).toBe("s1");
  });
});
