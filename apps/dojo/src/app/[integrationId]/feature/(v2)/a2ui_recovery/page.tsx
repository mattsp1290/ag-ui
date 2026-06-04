"use client";
import React from "react";
import "@copilotkit/react-core/v2/styles.css";
import "./style.css";
import {
  CopilotChat,
  useConfigureSuggestions,
} from "@copilotkit/react-core/v2";
import { CopilotKit } from "@copilotkit/react-core";
import { dynamicSchemaCatalog } from "@/a2ui-catalog";
// TEMPORARY (OSS-162): register the recovery renderer locally so the failure/retrying UI
// works with the PUBLISHED @copilotkit/react-core. Remove once react-core ships the built-in.
import { createA2UIRecoveryRenderer } from "./recovery-renderer";

export const dynamic = "force-dynamic";

interface PageProps {
  params: Promise<{ integrationId: string }>;
}

// Module-level (stable reference): CopilotKit's renderActivityMessages prop is guarded by
// useStableArrayProp, so this MUST be a constant array, not an inline literal. aimock attempts
// are instant, so reveal the "Retrying…" hint immediately for the demo (prod default delays ~2s).
// (Timing lives on the renderer here, not on `a2ui.recovery` — that config key only exists on
// the unpublished react-core build, and this dojo runs the published package.)
const recoveryRenderers = [
  createA2UIRecoveryRenderer({ showAfterMs: 0, showAfterAttempts: 1 }),
];

function Chat() {
  useConfigureSuggestions({
    suggestions: [
      {
        title: "Recover from an error",
        message: "Compare 3 luxury hotels with ratings and prices.",
      },
      {
        title: "Hard failure",
        message: "Compare 3 broken hotels with ratings and prices.",
      },
    ],
    available: "always",
  });

  return (
    <CopilotChat
      agentId="a2ui_recovery"
      className="h-full rounded-2xl max-w-6xl mx-auto"
    />
  );
}

export default function Page({ params }: PageProps) {
  const { integrationId } = React.use(params);

  return (
    <CopilotKit
      runtimeUrl={`/api/copilotkit/${integrationId}`}
      showDevConsole={false}
      agent="a2ui_recovery"
      // TEMPORARY (OSS-162): see recovery-renderer.tsx. Drop once published react-core
      // ships the built-in createA2UIRecoveryRenderer.
      renderActivityMessages={recoveryRenderers as any}
      a2ui={{
        catalog: dynamicSchemaCatalog,
      }}
    >
      <div className="flex justify-center items-center h-full w-full">
        <div className="h-full w-full md:w-8/10 md:h-8/10 rounded-lg">
          <Chat />
        </div>
      </div>
    </CopilotKit>
  );
}
