"use client";
// TEMPORARY (OSS-162): a local copy of @copilotkit/react-core's createA2UIRecoveryRenderer,
// registered via the dojo page's <CopilotKit renderActivityMessages={...}> prop so the
// retrying/failure UI works against the PUBLISHED @copilotkit/react-core (which does not ship
// the renderer yet). renderActivityMessages is a long-standing public prop; custom renderers
// are merged with the built-ins.
//
// REMOVE this file + the renderActivityMessages prop once @copilotkit/react-core publishes
// createA2UIRecoveryRenderer — then the provider's built-in registration handles it.
import { useEffect, useState } from "react";
import { z } from "zod";

export type A2UIRecoveryRendererOptions = {
  showAfterMs?: number;
  showAfterAttempts?: number;
  debugExposure?: "hidden" | "collapsed" | "verbose";
};

const RecoveryContentSchema = z
  .object({
    status: z.enum(["retrying", "failed", "resolved"]).optional(),
    attempt: z.number().optional(),
    maxAttempts: z.number().optional(),
    error: z.string().optional(),
    errors: z.array(z.any()).optional(),
    attempts: z.array(z.any()).optional(),
  })
  .passthrough();

export function createA2UIRecoveryRenderer(options: A2UIRecoveryRendererOptions = {}) {
  const showAfterMs = options.showAfterMs ?? 2000;
  const showAfterAttempts = options.showAfterAttempts ?? 2;
  const debugExposure = options.debugExposure ?? "collapsed";

  return {
    activityType: "a2ui_recovery",
    content: RecoveryContentSchema,
    render: ({ content }: { content: any }) => {
      const status = content?.status;
      if (status === "failed") {
        return <A2UIRecoveryFailure content={content} debugExposure={debugExposure} />;
      }
      if (status === "retrying") {
        return (
          <A2UIRetryingStatus
            content={content}
            showAfterMs={showAfterMs}
            showAfterAttempts={showAfterAttempts}
            debugExposure={debugExposure}
          />
        );
      }
      return null;
    },
  };
}

function A2UIRetryingStatus({
  content,
  showAfterMs,
  showAfterAttempts,
  debugExposure,
}: {
  content: any;
  showAfterMs: number;
  showAfterAttempts: number;
  debugExposure: "hidden" | "collapsed" | "verbose";
}) {
  const attempt = typeof content?.attempt === "number" ? content.attempt : undefined;
  const immediate = attempt !== undefined && attempt >= showAfterAttempts;
  const [visible, setVisible] = useState(immediate);

  useEffect(() => {
    if (immediate) {
      setVisible(true);
      return;
    }
    const timer = setTimeout(() => setVisible(true), showAfterMs);
    return () => clearTimeout(timer);
  }, [immediate, showAfterMs]);

  if (!visible) return null;

  const errors = Array.isArray(content?.errors) ? content.errors : [];
  return (
    <div className="cpk:flex cpk:flex-col cpk:gap-1 cpk:rounded-lg cpk:bg-gray-50 cpk:px-3 cpk:py-2 cpk:text-xs cpk:text-gray-500">
      <div className="cpk:flex cpk:items-center cpk:gap-2">
        <span
          className="cpk:h-2 cpk:w-2 cpk:rounded-full cpk:bg-gray-300"
          style={{ animation: "cpk-a2ui-pulse 1.5s ease-in-out infinite" }}
        />
        <span>Retrying UI generation…</span>
      </div>
      {debugExposure !== "hidden" && errors.length > 0 && (
        <A2UIDebugDetails
          label="validation issues"
          open={debugExposure === "verbose"}
          payload={{ attempt: content?.attempt, errors }}
        />
      )}
      <style>{`@keyframes cpk-a2ui-pulse {0%,100%{opacity:.4}50%{opacity:1}}`}</style>
    </div>
  );
}

function A2UIRecoveryFailure({
  content,
  debugExposure,
}: {
  content: any;
  debugExposure: "hidden" | "collapsed" | "verbose";
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
