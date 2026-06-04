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

// aimock attempts are instant, so reveal the "Retrying…" hint immediately / after the
// first retry for the demo (the production default delays it ~2s).
const recoveryRenderer = createA2UIRecoveryRenderer({
  showAfterMs: 0,
  showAfterAttempts: 1,
});

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
      renderActivityMessages={[recoveryRenderer] as any}
      a2ui={{
        catalog: dynamicSchemaCatalog,
        recovery: { showAfterMs: 0, showAfterAttempts: 1 },
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
