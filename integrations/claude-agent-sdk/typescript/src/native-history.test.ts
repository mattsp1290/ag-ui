import { afterEach, expect, it } from "vitest";
import { mkdtemp, mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { NativeHistory } from "./native-history";
const dirs: string[] = [];
const session = "11111111-1111-4111-8111-111111111111";
async function fixture() {
  const root = await mkdtemp(join(tmpdir(), "agui-native-"));
  dirs.push(root);
  const project = join(root, "project");
  await mkdir(join(project, session, "subagents"), { recursive: true });
  const parent = {
    type: "assistant",
    uuid: "parent-row",
    sessionId: session,
    version: "2.1.306",
    message: {
      role: "assistant",
      content: [
        { type: "tool_use", id: "parent-call", name: "Agent", input: {} },
      ],
    },
  };
  const child = {
    type: "assistant",
    uuid: "child-row",
    sessionId: session,
    agentId: "abc123",
    isSidechain: true,
    version: "2.1.306",
    message: {
      role: "assistant",
      content: [
        {
          type: "tool_use",
          id: "question",
          name: "Read",
          input: { file_path: "/fixture" },
        },
      ],
    },
  };
  await writeFile(
    join(project, session + ".jsonl"),
    JSON.stringify(parent) + "\n",
  );
  const path = join(project, session, "subagents", "agent-abc123.jsonl");
  await writeFile(path, JSON.stringify(child) + "\n");
  return { root, path, history: new NativeHistory(root, session, ["Read"]) };
}
afterEach(async () => {
  await Promise.all(
    dirs.splice(0).map((p) => rm(p, { recursive: true, force: true })),
  );
});
it("reconstructs unanswered questions without event storage and persists actual answers", async () => {
  const { root, path, history } = await fixture();
  const before = await readFile(path, "utf8");
  const first = await history.inspect();
  expect(first?.pending.map((t) => t.id)).toEqual(["question"]);
  expect(
    (await new NativeHistory(root, session, ["Read"]).inspect())?.pending,
  ).toEqual(first?.pending);
  expect(await readFile(path, "utf8")).toBe(before);
  await history.answer(first!, {
    question: { content: "REAL RESULT", is_error: false },
  });
  const restored = await new NativeHistory(root, session, ["Read"]).inspect();
  expect(restored?.pending).toEqual([]);
  expect(await readFile(path, "utf8")).toContain("REAL RESULT");
  await expect(
    history.answer(first!, { question: { content: "duplicate" } }),
  ).rejects.toThrow();
});
it("does not mistake an interrupted result for an answer", async () => {
  const { path, history } = await fixture();
  await writeFile(
    path,
    JSON.stringify({
      type: "user",
      uuid: "interrupted",
      message: {
        role: "user",
        content: [
          {
            type: "tool_result",
            tool_use_id: "question",
            is_error: true,
            content: "[Tool call interrupted: outcome unknown]",
          },
        ],
      },
    }) + "\n",
    { flag: "a" },
  );
  expect((await history.inspect())?.pending.map((t) => t.id)).toEqual([
    "question",
  ]);
});
it("rejects ambiguous children and unsafe session identifiers", async () => {
  const { root, path, history } = await fixture();
  await writeFile(path.replace("abc123", "def456"), await readFile(path));
  await expect(history.inspect()).rejects.toThrow(/one child/);
  expect(() => new NativeHistory(root, "../escape", ["Read"])).toThrow(
    /session/,
  );
});
it("repairs only the original parent result after durable child completion", async () => {
  const { root, path, history } = await fixture();
  const checkpoint = (await history.inspect())!;
  await history.answer(checkpoint, {
    question: { content: "DENIED", is_error: true },
  });
  await expect(history.repair(checkpoint)).rejects.toThrow(/not completed/);
  await writeFile(
    path,
    JSON.stringify({
      type: "assistant",
      uuid: "final-row",
      sessionId: session,
      agentId: "abc123",
      version: "2.1.306",
      message: {
        role: "assistant",
        stop_reason: "end_turn",
        content: [{ type: "text", text: "Child respected the denial" }],
      },
    }) + "\n",
    { flag: "a" },
  );
  await history.repair(checkpoint);
  expect(await history.inspect()).toBeUndefined();
  const parent = await readFile(
    join(root, "project", session + ".jsonl"),
    "utf8",
  );
  expect(parent).toContain("Child respected the denial");
  expect(await readFile(path, "utf8")).toContain('"is_error":true');
});
it("accepts an empty completed child response", async () => {
  const { path, history } = await fixture();
  const checkpoint = (await history.inspect())!;
  await history.answer(checkpoint, { question: { content: "" } });
  await writeFile(
    path,
    JSON.stringify({
      type: "assistant",
      uuid: "empty-final",
      version: "2.1.306",
      message: { role: "assistant", stop_reason: "end_turn", content: [] },
    }) + "\n",
    { flag: "a" },
  );
  expect(await history.completed(checkpoint)).toBe(true);
  await history.repair(checkpoint);
  expect(await history.inspect()).toBeUndefined();
});
