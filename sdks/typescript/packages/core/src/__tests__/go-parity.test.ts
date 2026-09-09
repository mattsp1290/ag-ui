import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import {
  AgentCapabilitiesSchema,
  EventSchemas,
  InputContentSchema,
  MessageSchema,
  RunAgentInputSchema,
  TokenUsageSchema,
  aggregateTokenUsage,
  tokenUsageFromLangChainMetadata,
} from "../index";
import * as core from "../index";

type Case = {
  id: string; kind: string; input: any; expected: any; valid: boolean;
  expected_by_language?: Record<string, any> | null;
  peer_validity?: Record<string, boolean> | null;
};
type RecordValue = { id: string; accepted: boolean; value?: any; error?: string; unsupported?: boolean };
type Envelope = { version: number; route: string; corpus_sha256: string; cases: RecordValue[] };

const dartMode = process.env.AG_UI_DART_PARITY_MODE === "1";
// Nx runs package targets with the package directory as cwd; resolve the
// checkout root explicitly so corpus paths cannot accidentally use a sibling.
const root = join(process.cwd(), "../../../../");
const corpusPath = dartMode && process.env.AG_UI_DART_PARITY_CORPUS
  ? process.env.AG_UI_DART_PARITY_CORPUS
  : join(root, "sdks/community/go/testdata/parity/fixtures.json");
const manifestPath = join(root, "sdks/community/go/testdata/parity/manifest.json");
const corpusBytes = readFileSync(corpusPath);
const corpus = JSON.parse(corpusBytes.toString()) as { version: number; cases: Case[] };
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const digest = createHash("sha256").update(corpusBytes).digest("hex");
const expectedFor = (c: Case) => Object.prototype.hasOwnProperty.call(c.expected_by_language ?? {}, "typescript")
  ? c.expected_by_language!.typescript : c.expected;
const peerValid = (c: Case) => Object.prototype.hasOwnProperty.call(c.peer_validity ?? {}, "typescript")
  ? c.peer_validity!.typescript : c.valid;


function unwrap(schema: any): any {
  let current = schema; const seen = new Set<any>();
  while (current?._def && !seen.has(current)) {
    seen.add(current); const d = current._def;
    const next = d.innerType ?? d.schema ?? d.type ?? d.out;
    if (!["ZodOptional", "ZodNullable", "ZodDefault", "ZodEffects", "ZodCatch", "ZodBranded", "ZodReadonly", "ZodPipeline"].includes(d.typeName) || !next) break;
    current = next;
  }
  return current;
}
function descriptor(schema: any): any {
  const base = unwrap(schema); const result: any = {
    required: !schema.safeParse(undefined).success,
    kind: base._def.typeName, nullable: schema.safeParse(null).success,
  };
  const undef = schema.safeParse(undefined);
  if (undef.success && undef.data !== undefined) result.default = undef.data;
  if (base._def.typeName === "ZodLiteral") result.literal = base._def.value;
  if (base._def.typeName === "ZodEnum") result.values = base._def.values;
  if (base._def.typeName === "ZodNativeEnum") result.values = [...new Set(Object.values(base._def.values))];
  return result;
}
function schemaInventory() {
  const objects: Record<string, any> = {};
  for (const [name, exported] of Object.entries(core).sort(([a], [b]) => a.localeCompare(b))) {
    if (!name.endsWith("Schema") || !(exported as any)?._def) continue;
    const object = unwrap(exported);
    if (object?._def?.typeName !== "ZodObject") continue;
    const shape = typeof object._def.shape === "function" ? object._def.shape() : object.shape;
    objects[name] = Object.fromEntries(Object.entries(shape).map(([k, v]) => [k, descriptor(v)]));
  }
  return objects;
}
function schemaFor(kind: string): any {
  const schemas: Record<string, any> = { event: EventSchemas, message: MessageSchema, request: RunAgentInputSchema,
    content: InputContentSchema, capabilities: AgentCapabilitiesSchema, usage: TokenUsageSchema };
  return schemas[kind];
}
function exception(route: string, c: Case): { accepted?: boolean; value?: any } | undefined {
  const entries = manifest.route_exceptions;
  if (!Array.isArray(entries)) return undefined;
  const found = entries.find((e: any) => e.case_id === c.id && e.route === route);
  return found;
}
function native(c: Case): { accepted: boolean; value?: any; error?: string } {
  try {
    if (c.kind === "aggregate") return { accepted: true, value: aggregateTokenUsage(c.input.entries) };
    if (c.kind === "mapper") return { accepted: true, value: tokenUsageFromLangChainMetadata(c.input.metadata, { provider: c.input.provider, model: c.input.model }) ?? null };
    const parsed = schemaFor(c.kind)?.safeParse(c.input);
    if (!parsed) return { accepted: false, error: `unsupported kind ${c.kind}` };
    return parsed.success ? { accepted: true, value: parsed.data } : { accepted: false, error: parsed.error.issues.map(i => i.message).join("; ") };
  } catch (e) { return { accepted: false, error: String(e) }; }
}
function expectedNative(c: Case, route = "typescript.produced") {
  const result = native(c); const exp = exception(route, c);
  const expected = exp && Object.prototype.hasOwnProperty.call(exp, "value") ? exp.value : expectedFor(c);
  expect(result.accepted, c.id).toBe(exp?.accepted ?? peerValid(c));
  if (result.accepted) expect(result.value, c.id).toEqual(expected);
  return result;
}
function makeProduced(): Envelope {
  return { version: 1, route: "typescript.produced", corpus_sha256: digest, cases: corpus.cases.map(c => {
    const r = expectedNative(c); return r.accepted ? { id: c.id, accepted: true, value: r.value } : { id: c.id, accepted: false, error: r.error || "rejected" };
  }) };
}
function strictEnvelope(raw: string, route: string): Envelope {
  const e = JSON.parse(raw) as Envelope;
  expect(Object.keys(e).sort()).toEqual(["cases", "corpus_sha256", "route", "version"]);
  expect(e.version).toBe(1); expect(e.route).toBe(route); expect(e.corpus_sha256).toBe(digest);
  expect(e.cases).toHaveLength(corpus.cases.length);
  expect(new Set(e.cases.map(x => x.id)).size).toBe(corpus.cases.length);
  expect(e.cases.map(x => x.id).sort()).toEqual(corpus.cases.map(x => x.id).sort());
  for (const r of e.cases) {
    expect(typeof r.accepted).toBe("boolean");
    const keys = Object.keys(r);
    if (r.accepted) {
      expect(keys.sort()).toEqual(["accepted", "id", "value"]);
      expect(r).toHaveProperty("value");
    } else {
      expect(keys.sort()).toSatisfy((k: string[]) => k.every(x => ["accepted", "error", "id", "unsupported"].includes(x)) && k.includes("id") && k.includes("error"));
      expect(typeof r.error).toBe("string"); expect(r.error!.length).toBeGreaterThan(0);
      if (Object.prototype.hasOwnProperty.call(r, "unsupported")) expect(typeof r.unsupported).toBe("boolean");
      expect(r).not.toHaveProperty("value");
    }
  }
  return e;
}
function consumePeer(go: Envelope, resultRoute = "typescript.from-go"): Envelope {
  const byId = new Map(go.cases.map(r => [r.id, r]));
  return { version: 1, route: resultRoute, corpus_sha256: digest, cases: corpus.cases.map(c => {
    const source = byId.get(c.id); expect(source, c.id).toBeTruthy();
    if (!source!.accepted) return { id: c.id, accepted: false, error: `not round-tripped: ${source!.error || "source rejected"}`, unsupported: source!.unsupported };
    try {
      let parsed: any;
      if (c.kind === "aggregate") parsed = Array.isArray(source!.value) ? source!.value.map((v: any) => TokenUsageSchema.parse(v)) : (() => { throw new Error("aggregate result must be an array"); })();
      else if (c.kind === "mapper") parsed = source!.value === null ? null : TokenUsageSchema.parse(source!.value);
      else parsed = schemaFor(c.kind)?.parse(source!.value);
      return { id: c.id, accepted: true, value: parsed };
    } catch (e) { return { id: c.id, accepted: false, error: String(e) }; }
  }) };
}
function assertInventory() {
  const actual = schemaInventory();
  expect(Object.keys(actual).sort()).toEqual(Object.keys(manifest.schema_fields.typescript).sort());
  expect(actual).toEqual(manifest.schema_fields.typescript);
}

describe("Go parity oracle", () => {
  it("matches the live exported schema inventory", () => assertInventory());
  it("covers the pinned native corpus", () => {
    expect(corpus.version).toBe(1);
    if (dartMode) {
      expect(corpus.cases.length).toBeGreaterThan(0);
      expect(new Set(corpus.cases.map(c => c.id)).size).toBe(corpus.cases.length);
    } else {
      expect(corpus.cases).toHaveLength(88);
      expect(corpus.cases.map(c => c.id)).toEqual(manifest.case_ids);
    }
    for (const c of corpus.cases) {
      if (c.kind === "aggregate" || c.kind === "mapper") expectedNative(c);
      else { const r = native(c); expect(r.accepted, c.id).toBe(peerValid(c)); if (r.accepted) expect(r.value, c.id).toEqual(expectedFor(c)); }
    }
  });
  it("runs the requested artifact phase", () => {
    const output = dartMode ? process.env.AG_UI_DART_PARITY_OUTPUT_DIR : process.env.AG_UI_PARITY_OUTPUT_DIR;
    const phase = dartMode ? process.env.AG_UI_DART_PARITY_PHASE : process.env.AG_UI_PARITY_PHASE;
    if (output === undefined && phase === undefined) return;
    expect(output, "AG_UI_PARITY_OUTPUT_DIR is required when a phase is set").toBeTruthy();
    expect(["produce", "consume", "verify"]).toContain(phase);
    if (phase === "produce") { writeFileSync(join(output!, "typescript.produced.json"), JSON.stringify(makeProduced()) + "\n"); return; }
    const sourceRoute = dartMode ? "dart.encoder" : "go.encoder";
    const resultRoute = dartMode ? "typescript.from-dart" : "typescript.from-go";
    const sourceFile = dartMode ? "dart.encoder.json" : "go.encoder.json";
    const peer = strictEnvelope(readFileSync(join(output!, sourceFile), "utf8"), sourceRoute);
    const consumed = consumePeer(peer, resultRoute);
    if (phase === "consume") {
      writeFileSync(join(output!, `${resultRoute}.json`), JSON.stringify(consumed) + "\n");
      if (dartMode) {
        const goPeer = strictEnvelope(readFileSync(join(output!, "go.encoder.json"), "utf8"), "go.encoder");
        writeFileSync(join(output!, "typescript.from-go.json"), JSON.stringify(consumePeer(goPeer)) + "\n");
      }
      return;
    }
    const stored: Record<string, Envelope> = {};
    for (const route of manifest.generated_artifacts.required_routes) stored[route] = strictEnvelope(readFileSync(join(output!, manifest.generated_artifacts.files[route]), "utf8"), route);
    if (dartMode) {
      stored["dart.encoder"] = strictEnvelope(readFileSync(join(output!, "dart.encoder.json"), "utf8"), "dart.encoder");
      stored["typescript.from-dart"] = strictEnvelope(readFileSync(join(output!, "typescript.from-dart.json"), "utf8"), "typescript.from-dart");
    }
    expect(makeProduced()).toEqual(stored["typescript.produced"]);
    expect(consumed).toEqual(stored[resultRoute]);
    if (dartMode) {
      const goPeer = strictEnvelope(readFileSync(join(output!, "go.encoder.json"), "utf8"), "go.encoder");
      expect(consumePeer(goPeer)).toEqual(stored["typescript.from-go"]);
    }
  });
});
