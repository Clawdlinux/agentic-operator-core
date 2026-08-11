import {
  AbsoluteFill,
  Sequence,
  useCurrentFrame,
  useVideoConfig,
  interpolate,
  spring,
} from "remotion";

// Brand colors
const COLORS = {
  bgDark: "#05080f",
  bgCard: "#0f1420",
  navy: "#1E2761",
  blue: "#3B82F6",
  blueLight: "#60A5FA",
  blueAccent: "#CADCFC",
  text: "#e2e8f0",
  textDim: "#94a3b8",
  green: "#4ade80",
};

const FONT_MONO =
  "'JetBrains Mono', 'Menlo', 'Monaco', 'Courier New', monospace";
const FONT_SANS =
  "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif";

// ============================================================================
// SCENE 1: Logo + Title (frames 0–120, 4s)
// ============================================================================
const Scene1Title: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const logoScale = spring({
    frame,
    fps,
    config: { damping: 12, stiffness: 100 },
  });

  const titleOpacity = interpolate(frame, [20, 50], [0, 1], {
    extrapolateRight: "clamp",
  });
  const titleY = interpolate(frame, [20, 50], [30, 0], {
    extrapolateRight: "clamp",
  });

  const taglineOpacity = interpolate(frame, [50, 80], [0, 1], {
    extrapolateRight: "clamp",
  });

  const exitOpacity = interpolate(frame, [100, 120], [1, 0], {
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        background: `radial-gradient(circle at center, ${COLORS.navy} 0%, ${COLORS.bgDark} 70%)`,
        alignItems: "center",
        justifyContent: "center",
        opacity: exitOpacity,
      }}
    >
      {/* Clawdlinux Seal-C mark */}
      <svg
        width={140}
        height={140}
        viewBox="0 0 88 96"
        style={{
          transform: `scale(${logoScale})`,
          marginBottom: 40,
          filter: `drop-shadow(0 0 40px ${COLORS.blue}66)`,
        }}
      >
        <g fill="none" stroke={COLORS.blueLight} strokeLinecap="round">
          <path
            d="M72 18H41C22 18 10 31 10 48S22 78 41 78H72"
            strokeWidth="8"
          />
          <path
            d="M65 31H43C33 31 26 38 26 48S33 65 43 65H65"
            strokeWidth="6"
            opacity="0.72"
          />
          <path
            d="M58 43H46C42 43 40 45 40 48S42 53 46 53H58"
            strokeWidth="4"
            opacity="0.46"
          />
        </g>
        <path d="M69 13L79 18L69 23Z" fill={COLORS.blueLight} />
      </svg>

      <div
        style={{
          opacity: titleOpacity,
          transform: `translateY(${titleY}px)`,
          fontFamily: FONT_SANS,
          fontSize: 96,
          fontWeight: 800,
          color: COLORS.text,
          letterSpacing: 0,
          marginBottom: 20,
        }}
      >
        Clawd<span style={{ color: COLORS.blue }}>linux</span>
      </div>

      <div
        style={{
          opacity: taglineOpacity,
          fontFamily: FONT_MONO,
          fontSize: 28,
          color: COLORS.textDim,
          letterSpacing: 0,
        }}
      >
        Governance for AI agents on Kubernetes
      </div>
    </AbsoluteFill>
  );
};

// ============================================================================
// SCENE 2: The problem (frames 120–270, 5s)
// ============================================================================
const Scene2Problem: React.FC = () => {
  const frame = useCurrentFrame();

  const lines = [
    "❌ Agents leak credentials via prompt injection",
    "❌ No way to enforce per-agent budgets in production",
    "❌ Air-gapped clusters can't run hosted agent platforms",
  ];

  const titleOpacity = interpolate(frame, [0, 20], [0, 1], {
    extrapolateRight: "clamp",
  });
  const exitOpacity = interpolate(frame, [130, 150], [1, 0], {
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        background: COLORS.bgDark,
        padding: 120,
        opacity: exitOpacity,
      }}
    >
      <div
        style={{
          opacity: titleOpacity,
          fontFamily: FONT_SANS,
          fontSize: 64,
          fontWeight: 700,
          color: COLORS.text,
          marginBottom: 80,
          letterSpacing: "-0.02em",
        }}
      >
        Deploying AI agents on K8s is broken.
      </div>

      {lines.map((line, i) => {
        const start = 30 + i * 25;
        const opacity = interpolate(frame, [start, start + 20], [0, 1], {
          extrapolateRight: "clamp",
        });
        const x = interpolate(frame, [start, start + 20], [-30, 0], {
          extrapolateRight: "clamp",
        });
        return (
          <div
            key={i}
            style={{
              opacity,
              transform: `translateX(${x}px)`,
              fontFamily: FONT_MONO,
              fontSize: 36,
              color: COLORS.text,
              marginBottom: 32,
              lineHeight: 1.4,
            }}
          >
            {line}
          </div>
        );
      })}
    </AbsoluteFill>
  );
};

// ============================================================================
// SCENE 3: The solution — manifest typewriter (frames 270–510, 8s)
// ============================================================================
const Scene3Manifest: React.FC = () => {
  const frame = useCurrentFrame();

  const fullManifest = `apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: research-swarm
spec:
  workflowName: research_swarm
  modelStrategy: cost-aware
  collaborationMode: delegation
  budget:
    maxUsd: 5.00
  egress:
    allowedFqdns:
      - api.openai.com
      - arxiv.org
  approval:
    autoApproveThreshold: 0.50`;

  // Typewriter: ~3 chars per frame
  const typeStart = 30;
  const typeSpeed = 4;
  const charsTyped = Math.max(
    0,
    Math.min(fullManifest.length, (frame - typeStart) * typeSpeed),
  );
  const visibleText = fullManifest.slice(0, charsTyped);
  const showCursor = Math.floor(frame / 15) % 2 === 0;

  const headerOpacity = interpolate(frame, [0, 20], [0, 1], {
    extrapolateRight: "clamp",
  });
  const exitOpacity = interpolate(frame, [220, 240], [1, 0], {
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        background: COLORS.bgDark,
        padding: "60px 120px",
        opacity: exitOpacity,
      }}
    >
      <div
        style={{
          opacity: headerOpacity,
          fontFamily: FONT_SANS,
          fontSize: 56,
          fontWeight: 700,
          color: COLORS.text,
          marginBottom: 40,
        }}
      >
        One manifest. <span style={{ color: COLORS.blue }}>Air-gapped by default.</span>
      </div>

      {/* Terminal window */}
      <div
        style={{
          background: COLORS.bgCard,
          borderRadius: 12,
          border: `1px solid ${COLORS.blue}33`,
          padding: 32,
          fontFamily: FONT_MONO,
          fontSize: 26,
          lineHeight: 1.6,
          color: COLORS.text,
          flex: 1,
          boxShadow: `0 20px 80px ${COLORS.blue}22`,
        }}
      >
        {/* Window chrome */}
        <div
          style={{
            display: "flex",
            gap: 8,
            marginBottom: 24,
            paddingBottom: 16,
            borderBottom: `1px solid ${COLORS.textDim}33`,
          }}
        >
          <div
            style={{
              width: 14,
              height: 14,
              borderRadius: "50%",
              background: "#ff5f57",
            }}
          />
          <div
            style={{
              width: 14,
              height: 14,
              borderRadius: "50%",
              background: "#febc2e",
            }}
          />
          <div
            style={{
              width: 14,
              height: 14,
              borderRadius: "50%",
              background: "#28c840",
            }}
          />
          <div
            style={{
              marginLeft: 20,
              color: COLORS.textDim,
              fontSize: 18,
            }}
          >
            agentworkload.yaml
          </div>
        </div>
        <pre
          style={{
            margin: 0,
            whiteSpace: "pre-wrap",
            color: COLORS.text,
          }}
        >
          {visibleText}
          {showCursor && charsTyped < fullManifest.length && (
            <span style={{ color: COLORS.blue }}>▊</span>
          )}
        </pre>
      </div>
    </AbsoluteFill>
  );
};

// ============================================================================
// SCENE 4: Apply + run (frames 510–630, 4s)
// ============================================================================
const Scene4Apply: React.FC = () => {
  const frame = useCurrentFrame();

  const cmdOpacity = interpolate(frame, [0, 15], [0, 1], {
    extrapolateRight: "clamp",
  });

  const outputs = [
    { text: "agentworkload.agentic.clawdlinux.org/research-swarm created", delay: 25 },
    { text: "✓ Cilium FQDN egress policy applied", delay: 45 },
    { text: "✓ Argo workflow scheduled (3 stages)", delay: 60 },
    { text: "✓ Cost tracker initialized: budget $5.00", delay: 75 },
    { text: "✓ Status: Running (0/3 stages complete)", delay: 90 },
  ];

  const exitOpacity = interpolate(frame, [110, 120], [1, 0], {
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        background: COLORS.bgDark,
        padding: "100px 120px",
        opacity: exitOpacity,
        justifyContent: "center",
      }}
    >
      <div
        style={{
          background: COLORS.bgCard,
          borderRadius: 12,
          border: `1px solid ${COLORS.blue}33`,
          padding: 48,
          fontFamily: FONT_MONO,
          fontSize: 28,
          lineHeight: 1.7,
          color: COLORS.text,
        }}
      >
        <div style={{ opacity: cmdOpacity, marginBottom: 20 }}>
          <span style={{ color: COLORS.blue }}>$</span>{" "}
          <span style={{ color: COLORS.text }}>kubectl apply -f agentworkload.yaml</span>
        </div>
        {outputs.map((o, i) => {
          const op = interpolate(frame, [o.delay, o.delay + 10], [0, 1], {
            extrapolateRight: "clamp",
          });
          const isCheck = o.text.startsWith("✓");
          return (
            <div
              key={i}
              style={{
                opacity: op,
                color: isCheck ? COLORS.green : COLORS.textDim,
                marginBottom: 6,
              }}
            >
              {o.text}
            </div>
          );
        })}
      </div>
    </AbsoluteFill>
  );
};

// ============================================================================
// SCENE 5: CTA (frames 630–720, 3s)
// ============================================================================
const Scene5CTA: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const scale = spring({
    frame,
    fps,
    config: { damping: 12, stiffness: 100 },
  });

  const urlOpacity = interpolate(frame, [20, 40], [0, 1], {
    extrapolateRight: "clamp",
  });

  return (
    <AbsoluteFill
      style={{
        background: `radial-gradient(circle at center, ${COLORS.navy} 0%, ${COLORS.bgDark} 70%)`,
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <div
        style={{
          transform: `scale(${scale})`,
          fontFamily: FONT_SANS,
          fontSize: 84,
          fontWeight: 800,
          color: COLORS.text,
          marginBottom: 24,
          letterSpacing: "-0.02em",
          textAlign: "center",
        }}
      >
        Open source. <span style={{ color: COLORS.blue }}>Apache 2.0.</span>
      </div>
      <div
        style={{
          opacity: urlOpacity,
          fontFamily: FONT_MONO,
          fontSize: 36,
          color: COLORS.blueLight,
          marginTop: 30,
          padding: "16px 32px",
          border: `1px solid ${COLORS.blue}66`,
          borderRadius: 12,
          background: `${COLORS.blue}11`,
        }}
      >
        github.com/Clawdlinux/agentic-operator-core
      </div>
      <div
        style={{
          opacity: urlOpacity,
          fontFamily: FONT_MONO,
          fontSize: 22,
          color: COLORS.textDim,
          marginTop: 24,
        }}
      >
        clawdlinux.org · discord.gg/XtxRdBzZcK
      </div>
    </AbsoluteFill>
  );
};

// ============================================================================
// MAIN COMPOSITION
// ============================================================================
export const MainComposition: React.FC = () => {
  return (
    <AbsoluteFill style={{ background: COLORS.bgDark }}>
      <Sequence from={0} durationInFrames={120}>
        <Scene1Title />
      </Sequence>
      <Sequence from={120} durationInFrames={150}>
        <Scene2Problem />
      </Sequence>
      <Sequence from={270} durationInFrames={240}>
        <Scene3Manifest />
      </Sequence>
      <Sequence from={510} durationInFrames={120}>
        <Scene4Apply />
      </Sequence>
      <Sequence from={630} durationInFrames={90}>
        <Scene5CTA />
      </Sequence>
    </AbsoluteFill>
  );
};
