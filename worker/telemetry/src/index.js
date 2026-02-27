/**
 * gcm telemetry Cloudflare Worker
 *
 * Accepts batched events from the gcm binary and forwards them to PostHog.
 * The PostHog write key never leaves this Worker — it is never in the binary.
 *
 * Request format (from internal/telemetry/http.go):
 *   POST /
 *   Content-Type: application/json
 *   { "events": [{ "name": "cmd_view", "props": { "distinct_id": "uuid", ... } }] }
 *
 * Response: 200 OK | 400 Bad Request | 405 Method Not Allowed
 *
 * Environment bindings required:
 *   POSTHOG_API_KEY  (secret)  — PostHog project API key (phc_...)
 *   KV               (KV namespace) — used for per-IP rate limiting
 */

const POSTHOG_BATCH_URL = "https://us.i.posthog.com/batch/";

// Rate limit: max 30 requests per IP per minute.
// Enforced asynchronously after responding — never blocks the CLI.
const RATE_LIMIT_PER_MINUTE = 30;

export default {
  async fetch(request, env, ctx) {
    if (request.method !== "POST") {
      return new Response("Method Not Allowed", { status: 405 });
    }

    // --- Parse and validate body ---
    let body;
    try {
      body = await request.json();
    } catch {
      return new Response("Bad Request: invalid JSON", { status: 400 });
    }

    if (!Array.isArray(body.events) || body.events.length === 0) {
      return new Response("Bad Request: missing events array", { status: 400 });
    }
    if (body.events.length > 50) {
      return new Response("Bad Request: too many events per batch", {
        status: 400,
      });
    }

    // --- Transform to PostHog batch format ---
    // The Go client merges distinct_id into props (see telemetry.go Record()).
    // We extract it here and move the rest into PostHog's properties field.
    const timestamp = new Date().toISOString();
    const batch = body.events.map((ev) => {
      const { name, props = {} } = ev;
      const { distinct_id, ...properties } = props;

      return {
        type: "capture",
        event: name || "unknown",
        distinct_id: distinct_id || "anonymous",
        timestamp,
        properties: {
          ...properties,
          $process_person_profile: false, // never create person profiles; anonymous events only
        },
      };
    });

    // --- Respond to client immediately ---
    // Rate limiting and PostHog forwarding happen after the response so the
    // CLI never waits on KV reads or PostHog latency.
    const ip = request.headers.get("CF-Connecting-IP") || "unknown";
    ctx.waitUntil(forwardToPostHog(env, ip, batch));

    return new Response("OK", { status: 200 });
  },
};

async function forwardToPostHog(env, ip, batch) {
  // Rate limiting (per IP, per minute) — runs after client already got 200
  const window = Math.floor(Date.now() / 60_000);
  const rlKey = `rl:${ip}:${window}`;

  const current = parseInt((await env.KV.get(rlKey)) || "0", 10);
  if (current >= RATE_LIMIT_PER_MINUTE) {
    return; // silently drop — client already received 200
  }
  await env.KV.put(rlKey, String(current + 1), { expirationTtl: 120 });

  await fetch(POSTHOG_BATCH_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      api_key: env.POSTHOG_API_KEY,
      batch,
    }),
  }).catch(() => {
    // PostHog errors are silently swallowed — telemetry must never affect the CLI
  });
}
