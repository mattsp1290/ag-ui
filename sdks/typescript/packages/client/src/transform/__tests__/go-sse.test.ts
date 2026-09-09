import { existsSync, readFileSync, realpathSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join, resolve } from "node:path";
import { EventSchemas, RunAgentInputSchema } from "@ag-ui/core";
import { EventEncoder } from "@ag-ui/encoder";
import { firstValueFrom, from, Subject, toArray } from "rxjs";
import { z } from "zod";
import { HttpEvent, HttpEventType } from "../../run/http-request";
import { transformHttpEventStream } from "../http";
import { defaultApplyEvents } from "../../apply/default";
import { HttpAgent } from "../../agent/http";

const repo = resolve(__dirname, "../../../../../../..");
const fixturePath = join(repo, "sdks/community/go/testdata/parity/sse-scenarios.json");
const rawFixture = JSON.parse(readFileSync(fixturePath, "utf8"));
const fixture = z
  .object({
    version: z.literal(1),
    scenarios: z
      .array(
        z
          .object({
            id: z.string().regex(/^[a-z][a-z0-9-]*$/),
            request: RunAgentInputSchema,
            events: z.array(EventSchemas).nonempty(),
          })
          .strict(),
      )
      .nonempty(),
  })
  .strict()
  .parse(rawFixture);
const output = process.env.AG_UI_SSE_PARITY_OUTPUT_DIR;
const requireHere = createRequire(import.meta.url);

async function parseFragmented(bytes: Uint8Array, chunkSize: number) {
  const source = new Subject<HttpEvent>();
  const parsed = firstValueFrom(transformHttpEventStream(source).pipe(toArray()));
  source.next({
    type: HttpEventType.HEADERS,
    status: 200,
    headers: new Headers({ "content-type": "text/event-stream" }),
  });
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    source.next({ type: HttpEventType.DATA, data: bytes.subarray(offset, offset + chunkSize) });
  }
  source.complete();
  return parsed;
}

describe("actual Go and TypeScript SSE scenarios", () => {
  it("uses checkout schemas and encoder with a complete scenario fixture", () => {
    expect(fixture).toEqual(rawFixture);
    expect(
      fixture.scenarios.map(({ id, events }) => ({
        id,
        count: events.length,
        terminal: events.at(-1)?.type,
      })),
    ).toEqual([
      { id: "interrupt", count: 29, terminal: "RUN_FINISHED" },
      { id: "resumed", count: 10, terminal: "RUN_FINISHED" },
      { id: "root-error", count: 4, terminal: "RUN_ERROR" },
    ]);
    expect(new Set(fixture.scenarios.map((s) => s.id)).size).toBe(fixture.scenarios.length);
    console.info("Node SSE interpreter:", process.execPath);
    for (const name of ["core", "encoder"]) {
      console.info(
        `TypeScript SSE ${name} source:`,
        realpathSync(requireHere.resolve(`@ag-ui/${name}`)),
      );
      expect(realpathSync(requireHere.resolve(`@ag-ui/${name}`))).toContain(
        join(repo, `sdks/typescript/packages/${name}/`),
      );
    }
    if (output !== undefined) {
      expect(output).not.toBe("");
      expect(statSync(output).isDirectory()).toBe(true);
    }
  });

  for (const scenario of fixture.scenarios) {
    it(`${scenario.id}: emits real TypeScript encoder bytes`, async () => {
      const encoder = new EventEncoder();
      const wire = scenario.events.map((event) => encoder.encode(event)).join("");
      expect(encoder.getContentType()).toBe("text/event-stream");
      const bytes = new TextEncoder().encode(wire);
      expect(await parseFragmented(bytes, 1)).toEqual(scenario.events);
      if (output !== undefined) writeFileSync(join(output, `typescript-${scenario.id}.sse`), bytes);
    });

    it.skipIf(output === undefined)(
      `${scenario.id}: consumes actual Go SSEWriter bytes`,
      async () => {
        const path = join(output!, `go-${scenario.id}.sse`);
        expect(existsSync(path), "Go producer output is mandatory in cross-language mode").toBe(
          true,
        );
        const bytes = readFileSync(path);
        expect(bytes.length).toBeGreaterThan(0);
        for (const chunkSize of [1, 7, 64, bytes.length]) {
          expect(await parseFragmented(bytes, chunkSize)).toEqual(scenario.events);
        }
        if (scenario.id === "interrupt") {
          const parsed = await parseFragmented(bytes, 7);
          const patchEvents = parsed.filter((event) =>
            ["STATE_SNAPSHOT", "STATE_DELTA", "ACTIVITY_SNAPSHOT", "ACTIVITY_DELTA"].includes(
              event.type,
            ),
          );
          const agent = new HttpAgent({ url: "http://unused.invalid" });
          const mutations = await firstValueFrom(
            defaultApplyEvents(scenario.request, from(patchEvents), agent, []).pipe(toArray()),
          );
          expect(mutations.filter((m) => m.state !== undefined).at(-1)?.state).toEqual({
            tasks: [{ id: "approval-1", ready: false }],
          });
          const activity = mutations
            .filter((m) => m.messages !== undefined)
            .at(-1)
            ?.messages?.find((m) => m.id === "activity-1");
          expect(activity).toMatchObject({
            role: "activity",
            activityType: "progress",
            subagentRunId: "worker",
            content: { phase: "approval", waiting: true },
          });
        }
      },
    );
  }
});
