import { describe, it, expect } from "vitest";
import {
  A2UI_OPERATIONS_KEY,
  BASIC_CATALOG_ID,
  RENDER_A2UI_TOOL_DEF,
  assembleOps,
  buildContextPrompt,
  buildSubagentPrompt,
  createSurface,
  findPriorSurface,
  updateComponents,
  updateDataModel,
  wrapAsOperationsEnvelope,
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
