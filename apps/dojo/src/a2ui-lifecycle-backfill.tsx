"use client";
// TEMPORARY (OSS-162): backfill of the unified A2UI generation-lifecycle renderer.
//
// The middleware now drives the WHOLE lifecycle on ONE `a2ui-surface` activity
// (building → retrying → failed → painted, swapped in place on one messageId), and
// react-core's built-in `a2ui-surface` renderer was updated to render it + the
// `render_a2ui` tool-call skeleton was retired. This dojo runs the PUBLISHED
// @copilotkit/react-core, which still has the OLD surface renderer (no lifecycle)
// and still ships the per-tool-call skeleton. So until react-core republishes:
//   - `createA2UISurfaceLifecycleRenderer` overrides the published `a2ui-surface`
//     renderer (via renderActivityMessages) with the lifecycle-aware one.
//   - `SuppressRenderA2UISkeleton` nulls the published render_a2ui tool-call
//     skeleton (it was the source of the duplicate / lingering skeleton).
//
// REMOVE this file + its usages once react-core publishes the unified renderer.
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { z } from "zod";
import {
  A2UIProvider,
  useA2UIActions,
  useA2UIError,
  A2UIRenderer,
  initializeDefaultCatalog,
  injectStyles,
  DEFAULT_SURFACE_ID,
  viewerTheme,
} from "@copilotkit/a2ui-renderer";
import { useCopilotKit, useRenderTool } from "@copilotkit/react-core/v2";

const A2UI_OPERATIONS_KEY = "a2ui_operations";

type DebugExposure = "hidden" | "collapsed" | "verbose";

export type A2UISurfaceLifecycleOptions = {
  catalog?: any;
  theme?: any;
  showAfterMs?: number;
  showAfterAttempts?: number;
  debugExposure?: DebugExposure;
};

const ContentSchema = z
  .object({
    a2ui_operations: z.array(z.any()).optional(),
    status: z.enum(["building", "retrying", "failed"]).optional(),
    attempt: z.number().optional(),
    maxAttempts: z.number().optional(),
    progressTokens: z.number().optional(),
    error: z.string().optional(),
    errors: z.array(z.any()).optional(),
    attempts: z.array(z.any()).optional(),
    debugExposure: z.enum(["hidden", "collapsed", "verbose"]).optional(),
  })
  .passthrough();

let initialized = false;
function ensureInitialized() {
  if (!initialized) {
    initializeDefaultCatalog();
    injectStyles();
    initialized = true;
  }
}

/**
 * Lifecycle-aware `a2ui-surface` renderer: paints when operations are present,
 * else renders the building / retrying / failed pre-paint states. All states ride
 * the one activity messageId, so the painted surface replaces them in place.
 */
export function createA2UISurfaceLifecycleRenderer(
  options: A2UISurfaceLifecycleOptions = {},
) {
  const theme = options.theme ?? viewerTheme;
  const catalog = options.catalog;
  const showAfterMs = options.showAfterMs ?? 2000;
  const showAfterAttempts = options.showAfterAttempts ?? 2;
  const optionDebugExposure = options.debugExposure ?? "collapsed";

  return {
    activityType: "a2ui-surface",
    content: ContentSchema,
    render: ({ content, agent }: { content: any; agent: any }) => {
      ensureInitialized();

      const [operations, setOperations] = useState<any[]>([]);
      const { copilotkit } = useCopilotKit();

      const lastContentRef = useRef<unknown>(null);
      useEffect(() => {
        if (content === lastContentRef.current) return;
        lastContentRef.current = content;
        const incoming = content?.[A2UI_OPERATIONS_KEY];
        setOperations(Array.isArray(incoming) ? incoming : []);
      }, [content]);

      const groupedOperations = useMemo(() => {
        const groups = new Map<string, any[]>();
        for (const operation of operations) {
          const surfaceId = getOperationSurfaceId(operation) ?? DEFAULT_SURFACE_ID;
          if (!groups.has(surfaceId)) groups.set(surfaceId, []);
          groups.get(surfaceId)!.push(operation);
        }
        return groups;
      }, [operations]);

      const hasOps = groupedOperations.size > 0;

      const renderLifecycle = (c: any) => {
        const status = c?.status;
        const debugExposure: DebugExposure = c?.debugExposure ?? optionDebugExposure;
        if (status === "failed") {
          return <A2UIRecoveryFailure content={c} debugExposure={debugExposure} />;
        }
        if (status === "retrying") {
          return (
            <A2UIRetryingState
              content={c}
              showAfterMs={showAfterMs}
              showAfterAttempts={showAfterAttempts}
              debugExposure={debugExposure}
            />
          );
        }
        return <A2UIBuildingState content={c} />;
      };

      // Keep showing the last pre-paint snapshot during the hand-off below.
      const lastLoaderContentRef = useRef<any>(null);
      if (!hasOps) lastLoaderContentRef.current = content;

      // Cross-over (OSS-162): hold the skeleton in-flow while the surface mounts +
      // paints OFFSCREEN, then swap — so the first card replaces the skeleton with
      // no empty gap (the A2UIProvider needs a couple ticks to paint after mount).
      const [surfaceReady, setSurfaceReady] = useState(false);
      useEffect(() => {
        if (!hasOps) {
          setSurfaceReady(false);
          return;
        }
        const t = setTimeout(() => setSurfaceReady(true), 220);
        return () => clearTimeout(t);
      }, [hasOps]);

      if (!hasOps) {
        return renderLifecycle(content);
      }

      const surfaces = (
        <div className="cpk:flex cpk:min-h-0 cpk:flex-1 cpk:flex-col cpk:gap-6 cpk:overflow-auto cpk:py-6">
          {Array.from(groupedOperations.entries()).map(([surfaceId, ops]) => (
            <ReactSurfaceHost
              key={surfaceId}
              surfaceId={surfaceId}
              operations={ops}
              theme={theme}
              agent={agent}
              copilotkit={copilotkit}
              catalog={catalog}
            />
          ))}
        </div>
      );

      if (surfaceReady) return surfaces;

      return (
        <div style={{ position: "relative" }}>
          <div
            aria-hidden
            style={{ position: "absolute", inset: 0, opacity: 0, pointerEvents: "none" }}
          >
            {surfaces}
          </div>
          {renderLifecycle(lastLoaderContentRef.current ?? content)}
        </div>
      );
    },
  };
}

/** Nulls the published `render_a2ui` tool-call skeleton (surface activity owns loading now). */
export function SuppressRenderA2UISkeleton(): null {
  useRenderTool(
    {
      name: "render_a2ui",
      parameters: z.any(),
      render: () => <></>,
    },
    [],
  );
  return null;
}

// --- Paint path (mirrors react-core's ReactSurfaceHost) ----------------------

function ReactSurfaceHost({
  surfaceId,
  operations,
  theme,
  agent,
  copilotkit,
  catalog,
}: {
  surfaceId: string;
  operations: any[];
  theme: any;
  agent: any;
  copilotkit: any;
  catalog?: any;
}) {
  const handleAction = useCallback(
    async (message: any) => {
      if (!agent) return;
      try {
        copilotkit.setProperties({ ...copilotkit.properties, a2uiAction: message });
        await copilotkit.runAgent({ agent });
      } finally {
        if (copilotkit.properties) {
          const { a2uiAction, ...rest } = copilotkit.properties;
          copilotkit.setProperties(rest);
        }
      }
    },
    [agent, copilotkit],
  );

  return (
    <div className="cpk:flex cpk:w-full cpk:flex-none cpk:flex-col cpk:gap-4">
      <A2UIProvider onAction={handleAction} theme={theme} catalog={catalog}>
        <SurfaceMessageProcessor surfaceId={surfaceId} operations={operations} />
        <A2UISurfaceOrError surfaceId={surfaceId} />
      </A2UIProvider>
    </div>
  );
}

function A2UISurfaceOrError({ surfaceId }: { surfaceId: string }) {
  const error = useA2UIError();
  if (error) {
    return (
      <div className="cpk:rounded-lg cpk:border cpk:border-red-200 cpk:bg-red-50 cpk:p-3 cpk:text-sm cpk:text-red-700">
        A2UI render error: {error}
      </div>
    );
  }
  return <A2UIRenderer surfaceId={surfaceId} className="cpk:flex cpk:flex-1" />;
}

function SurfaceMessageProcessor({
  surfaceId,
  operations,
}: {
  surfaceId: string;
  operations: any[];
}) {
  const { processMessages, getSurface } = useA2UIActions();
  const lastHashRef = useRef<string>("");
  useEffect(() => {
    const hash = JSON.stringify(operations);
    if (hash === lastHashRef.current) return;
    lastHashRef.current = hash;
    const existing = getSurface(surfaceId);
    const ops = existing
      ? operations.filter((op) => !op?.createSurface)
      : operations;
    processMessages(ops);
  }, [processMessages, getSurface, surfaceId, operations]);
  return null;
}

function getOperationSurfaceId(operation: any): string | null {
  if (!operation || typeof operation !== "object") return null;
  if (typeof operation.surfaceId === "string") return operation.surfaceId;
  return (
    operation?.createSurface?.surfaceId ??
    operation?.updateComponents?.surfaceId ??
    operation?.updateDataModel?.surfaceId ??
    operation?.deleteSurface?.surfaceId ??
    null
  );
}

// --- Lifecycle states (mirror react-core's A2UIRecoveryStates) ----------------

function A2UIBuildingState({ content }: { content: any }) {
  const tokens =
    typeof content?.progressTokens === "number" ? content.progressTokens : undefined;
  return <A2UIGeneratingSkeleton label="Building interface" tokens={tokens} />;
}

function A2UIRetryingState({
  content,
  showAfterMs,
  showAfterAttempts,
  debugExposure,
}: {
  content: any;
  showAfterMs: number;
  showAfterAttempts: number;
  debugExposure: DebugExposure;
}) {
  const attempt = typeof content?.attempt === "number" ? content.attempt : undefined;
  const maxAttempts =
    typeof content?.maxAttempts === "number" ? content.maxAttempts : undefined;
  const immediate = attempt !== undefined && attempt >= showAfterAttempts;
  const [revealed, setRevealed] = useState(immediate);

  useEffect(() => {
    if (immediate) {
      setRevealed(true);
      return;
    }
    const timer = setTimeout(() => setRevealed(true), showAfterMs);
    return () => clearTimeout(timer);
  }, [immediate, showAfterMs]);

  const tokens =
    typeof content?.progressTokens === "number" ? content.progressTokens : undefined;

  if (!revealed) {
    return <A2UIGeneratingSkeleton label="Building interface" tokens={tokens} />;
  }

  const label =
    attempt !== undefined && maxAttempts !== undefined
      ? `Retrying generation… (${attempt}/${maxAttempts} attempts)`
      : "Retrying generation…";
  const errors = Array.isArray(content?.errors) ? content.errors : [];

  return (
    <A2UIGeneratingSkeleton label={label} tokens={tokens}>
      {debugExposure !== "hidden" && errors.length > 0 && (
        <A2UIDebugDetails
          label="validation issues"
          open={debugExposure === "verbose"}
          payload={{ attempt: content?.attempt, errors }}
        />
      )}
    </A2UIGeneratingSkeleton>
  );
}

function A2UIRecoveryFailure({
  content,
  debugExposure,
}: {
  content: any;
  debugExposure: DebugExposure;
}) {
  return (
    <div className="cpk:rounded-lg cpk:border cpk:border-amber-200 cpk:bg-amber-50 cpk:p-3 cpk:text-sm cpk:text-amber-800">
      <div className="cpk:font-medium">Couldn't generate the UI</div>
      <div className="cpk:mt-1 cpk:text-xs cpk:text-amber-700">
        Something went wrong rendering this. You can keep chatting and try again.
      </div>
      {debugExposure !== "hidden" && (
        <A2UIDebugDetails
          label="developer details"
          open={debugExposure === "verbose"}
          payload={{ error: content?.error, attempts: content?.attempts }}
        />
      )}
    </div>
  );
}

function A2UIGeneratingSkeleton({
  label,
  tokens,
  children,
}: {
  label: string;
  tokens?: number;
  children?: React.ReactNode;
}) {
  const phase =
    tokens == null ? 3 : tokens < 50 ? 0 : tokens < 200 ? 1 : tokens < 400 ? 2 : 3;

  return (
    <div style={{ margin: "12px 0", maxWidth: 320 }}>
      <div
        style={{
          position: "relative",
          overflow: "hidden",
          borderRadius: 12,
          border: "1px solid rgba(228,228,231,0.8)",
          backgroundColor: "#fff",
          boxShadow: "0 1px 2px rgba(0,0,0,0.04)",
          padding: "16px 18px 14px",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
          <div style={{ display: "flex", gap: 4 }}>
            <Dot />
            <Dot />
            <Dot />
          </div>
          <Bar w={64} h={6} bg="#e4e4e7" opacity={phase >= 1 ? 1 : 0.4} transition="opacity 0.5s" />
        </div>
        <div style={{ display: "grid", gap: 7 }}>
          <Row show={phase >= 0}>
            <Bar w={36} h={7} bg="rgba(147,197,253,0.7)" anim={0} />
            <Bar w={80} h={7} bg="rgba(219,234,254,0.8)" anim={0.2} />
          </Row>
          <Row show={phase >= 0} delay={0.1}>
            <Spacer />
            <Dot />
            <Bar w={100} h={7} bg="rgba(24,24,27,0.2)" anim={0.3} />
          </Row>
          <Row show={phase >= 1} delay={0.15}>
            <Spacer />
            <Bar w={48} h={7} bg="rgba(24,24,27,0.15)" anim={0.1} />
            <Bar w={40} h={7} bg="rgba(153,246,228,0.6)" anim={0.5} />
            <Bar w={56} h={7} bg="rgba(147,197,253,0.6)" anim={0.3} />
          </Row>
          <Row show={phase >= 1} delay={0.2}>
            <Spacer />
            <Dot />
            <Bar w={60} h={7} bg="rgba(24,24,27,0.15)" anim={0.4} />
          </Row>
          <Row show={phase >= 2} delay={0.25}>
            <Bar w={40} h={7} bg="rgba(153,246,228,0.5)" anim={0.2} />
            <Dot />
            <Bar w={48} h={7} bg="rgba(24,24,27,0.15)" anim={0.6} />
            <Bar w={64} h={7} bg="rgba(147,197,253,0.5)" anim={0.1} />
          </Row>
          <Row show={phase >= 2} delay={0.3}>
            <Bar w={36} h={7} bg="rgba(147,197,253,0.6)" anim={0.5} />
            <Bar w={36} h={7} bg="rgba(24,24,27,0.12)" anim={0.7} />
          </Row>
          <Row show={phase >= 3} delay={0.35}>
            <Dot />
            <Bar w={44} h={7} bg="rgba(24,24,27,0.18)" anim={0.3} />
            <Dot />
            <Bar w={56} h={7} bg="rgba(153,246,228,0.5)" anim={0.8} />
            <Bar w={48} h={7} bg="rgba(147,197,253,0.5)" anim={0.4} />
          </Row>
        </div>
        <div
          style={{
            pointerEvents: "none",
            position: "absolute",
            inset: 0,
            background:
              "linear-gradient(105deg, transparent 0%, transparent 40%, rgba(255,255,255,0.6) 50%, transparent 60%, transparent 100%)",
            backgroundSize: "250% 100%",
            animation: "cpk-a2ui-sweep 3s ease-in-out infinite",
          }}
        />
      </div>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 8,
          marginTop: 8,
        }}
      >
        <span style={{ fontSize: 12, color: "#a1a1aa", letterSpacing: "0.025em" }}>{label}</span>
        {typeof tokens === "number" && tokens > 0 && (
          <span style={{ fontSize: 11, color: "#d4d4d8", fontVariantNumeric: "tabular-nums" }}>
            ~{tokens.toLocaleString()} tokens
          </span>
        )}
      </div>
      {children}
      <style>{`
        @keyframes cpk-a2ui-fade { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        @keyframes cpk-a2ui-sweep { 0% { background-position: 250% 0; } 100% { background-position: -250% 0; } }
      `}</style>
    </div>
  );
}

function A2UIDebugDetails({
  label,
  open,
  payload,
}: {
  label: string;
  open: boolean;
  payload: unknown;
}) {
  return (
    <details open={open} className="cpk:mt-2 cpk:text-xs">
      <summary className="cpk:cursor-pointer cpk:text-gray-500">{label}</summary>
      <pre
        className="cpk:mt-1 cpk:overflow-auto cpk:rounded cpk:bg-gray-100 cpk:p-2 cpk:text-gray-700"
        style={{ fontSize: 11 }}
      >
        {JSON.stringify(payload, null, 2)}
      </pre>
    </details>
  );
}

function Dot() {
  return (
    <div
      style={{ width: 7, height: 7, borderRadius: "50%", backgroundColor: "#d4d4d8", flexShrink: 0 }}
    />
  );
}
function Spacer() {
  return <div style={{ width: 12 }} />;
}
function Bar({
  w,
  h,
  bg,
  anim,
  opacity,
  transition,
}: {
  w: number;
  h: number;
  bg: string;
  anim?: number;
  opacity?: number;
  transition?: string;
}) {
  return (
    <div
      style={{
        width: w,
        height: h,
        borderRadius: 9999,
        backgroundColor: bg,
        ...(anim !== undefined
          ? { animation: `cpk-a2ui-fade 2.4s ease-in-out ${anim}s infinite` }
          : {}),
        ...(opacity !== undefined ? { opacity } : {}),
        ...(transition ? { transition } : {}),
      }}
    />
  );
}
function Row({
  children,
  show,
  delay = 0,
}: {
  children: React.ReactNode;
  show: boolean;
  delay?: number;
}) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 6,
        opacity: show ? 1 : 0,
        transition: `opacity 0.4s ${delay}s`,
      }}
    >
      {children}
    </div>
  );
}
