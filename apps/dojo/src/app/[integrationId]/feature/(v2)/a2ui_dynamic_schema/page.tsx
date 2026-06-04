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
// TEMPORARY (OSS-162): override the published a2ui-surface renderer with the unified
// lifecycle one + suppress the published render_a2ui tool-call skeleton (the source
// of the duplicate / lingering skeleton). Remove once react-core republishes.
import {
  createA2UISurfaceLifecycleRenderer,
  SuppressRenderA2UISkeleton,
} from "@/a2ui-lifecycle-backfill";

export const dynamic = "force-dynamic";

interface PageProps {
  params: Promise<{ integrationId: string }>;
}

// Stable reference (renderActivityMessages is guarded by useStableArrayProp).
const lifecycleRenderers = [
  createA2UISurfaceLifecycleRenderer({ catalog: dynamicSchemaCatalog }),
];

function Chat() {
  useConfigureSuggestions({
    suggestions: [
      {
        title: "Hotel comparison",
        message:
          "Compare 3 luxury hotels in different cities with ratings and prices.",
      },
      {
        title: "Product comparison",
        message:
          "Compare 3 wireless headphones with prices, ratings, and descriptions.",
      },
      {
        title: "Team roster",
        message:
          "Show a team of 4 people with their roles, departments, and contact info.",
      },
    ],
    available: "always",
  });

  return (
    <CopilotChat
      agentId="a2ui_dynamic_schema"
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
      agent="a2ui_dynamic_schema"
      // TEMPORARY (OSS-162): see a2ui-lifecycle-backfill.tsx. Drop once published
      // react-core ships the unified a2ui-surface lifecycle renderer.
      renderActivityMessages={lifecycleRenderers as any}
      a2ui={{ catalog: dynamicSchemaCatalog }}
    >
      {/* TEMPORARY (OSS-162): null the published render_a2ui tool-call skeleton. */}
      <SuppressRenderA2UISkeleton />
      <div className="flex justify-center items-center h-full w-full">
        <div className="h-full w-full md:w-8/10 md:h-8/10 rounded-lg">
          <Chat />
        </div>
      </div>
    </CopilotKit>
  );
}
