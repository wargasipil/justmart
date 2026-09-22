import type { AnyMessage, JsonValue, Message, MethodInfo, PartialMessage, ServiceType } from "@bufbuild/protobuf";
import { Code } from "@connectrpc/connect";
import { delay, http, HttpResponse, type HttpHandler } from "msw";

// Typed MSW handlers for ConnectRPC — how a page story gets data with no backend.
//
// The app's transport speaks the Connect protocol with JSON bodies: every unary
// RPC is `POST /api/<package>.<Service>/<Method>`. So a handler here is keyed by
// the SAME generated service descriptor the app's client uses, and a response is
// built through the generated message class. That is the point of routing
// fixtures through `new O(partial).toJson()` rather than hand-writing JSON: a
// renamed proto field is a type error in the story, not a silently blank cell.
//
//   parameters: {
//     msw: mockApi(
//       mockRpc(ProductService, "getProduct", { product: fixture }),
//       mockRpc(BatchService, "listBatches", (req) => ({ batches: [], total: 0 })),
//     ),
//   }

type InOf<M> = M extends MethodInfo<infer I, AnyMessage> ? I : never;
// The nested `O extends Message<O>` re-states MethodInfo's own constraint: TS
// doesn't carry a type parameter's constraint onto an `infer`red type, and
// PartialMessage requires it.
type PartialOut<M> =
  M extends MethodInfo<AnyMessage, infer O> ? (O extends Message<O> ? PartialMessage<O> : never) : never;

type Answer<M> = PartialOut<M> | StoryRpcError;
type Responder<M> = Answer<M> | ((req: InOf<M>) => Answer<M> | Promise<Answer<M>>);

type MockOpts = {
  /** Hold the response back — a number of ms, or "infinite" for a permanent loading state. */
  delay?: number | "infinite";
};

function rpcPath(service: ServiceType, method: string): string {
  const info = service.methods[method] as MethodInfo<AnyMessage, AnyMessage>;
  return `/api/${service.typeName}/${info.name}`;
}

/** Answer one RPC with a fixture (or a function of the decoded request). */
export function mockRpc<S extends ServiceType, K extends keyof S["methods"] & string>(
  service: S,
  method: K,
  respond: Responder<S["methods"][K]>,
  opts: MockOpts = {},
): HttpHandler {
  const info = service.methods[method] as MethodInfo<AnyMessage, AnyMessage>;
  return http.post(rpcPath(service, method), async ({ request }) => {
    if (opts.delay !== undefined) await delay(opts.delay);
    const req = info.I.fromJson((await request.json()) as JsonValue, { ignoreUnknownFields: true });
    const partial =
      typeof respond === "function"
        ? await (respond as (r: AnyMessage) => unknown)(req)
        : respond;
    if (partial instanceof StoryRpcError) return errorResponse(partial.code, partial.message);
    const out = new info.O(partial as PartialMessage<AnyMessage>) as Message;
    return HttpResponse.json(out.toJson() as object);
  });
}

// Connect's wire name for a code is the snake_case of the enum member
// (Code.NotFound → "not_found"); the client reads the code from this body, so
// the HTTP status only needs to be a non-200.
function codeName(code: Code): string {
  return Code[code].replace(/[A-Z]/g, (c, i: number) => (i ? "_" : "") + c.toLowerCase());
}

const HTTP_STATUS: Partial<Record<Code, number>> = {
  [Code.InvalidArgument]: 400,
  [Code.Unauthenticated]: 401,
  [Code.PermissionDenied]: 403,
  [Code.NotFound]: 404,
  [Code.AlreadyExists]: 409,
  [Code.FailedPrecondition]: 412,
  [Code.Unimplemented]: 501,
  [Code.Unavailable]: 503,
};

function errorResponse(code: Code, message: string) {
  return HttpResponse.json({ code: codeName(code), message }, { status: HTTP_STATUS[code] ?? 500 });
}

/** What `rpcError` returns; `mockRpc` turns it into a Connect error body. */
class StoryRpcError {
  constructor(
    readonly code: Code,
    readonly message: string,
  ) {}
}

/**
 * Fail the CALL a `mockRpc` responder is answering, instead of answering it.
 *
 * `mockRpcError` fails an RPC unconditionally, which is right for a page-level
 * state ("ListProducts is down"). A form is the other case: the interesting
 * state is that SOME inputs are refused, and the panel renders the refusal on
 * the offending field. So a responder decides per request:
 *
 * ```ts
 * mockRpc(SettingsService, "setTunnelSettings", (req) =>
 *   looksLikeAToken(req.token) ? { configured: true } : rpcError(Code.InvalidArgument, "settings.tunnel_token_invalid"),
 * )
 * ```
 *
 * `message` is the backend's stable token (see lib/serverErrors.ts), not prose
 * — that is what `useServerFormErrors` maps onto a field, so the story shows
 * the same translated message the shop sees.
 */
export function rpcError(code: Code, message = ""): StoryRpcError {
  return new StoryRpcError(code, message);
}

/** Fail one RPC with a Connect error (e.g. a NotFound for a missing record). */
export function mockRpcError<S extends ServiceType>(
  service: S,
  method: keyof S["methods"] & string,
  code: Code,
  message = "",
): HttpHandler {
  return http.post(rpcPath(service, method), () => errorResponse(code, message));
}

/**
 * The story's handlers plus a closing catch-all for every other `/api` call.
 *
 * Without it, an RPC the story forgot to mock passes through to the vite `/api`
 * proxy — against `make run` it would quietly render dev-DB rows inside a
 * "fixture" story, and without a backend it fails as an opaque proxy error.
 * Answering `Unimplemented` with the procedure name instead makes a missing mock
 * a named failure in the console and network panel — and, for a role story, an
 * assertion: an RPC the page must NOT fire for that role (a manager-only read
 * on the cashier view) is simply left unmocked, so firing it shows up. It's per-story (not a global strategy) so the
 * three `needs-backend` component stories still reach the real server.
 */
export function mockApi(...handlers: HttpHandler[]): HttpHandler[] {
  return [
    ...handlers,
    http.post("/api/*", ({ request }) =>
      HttpResponse.json(
        {
          code: codeName(Code.Unimplemented),
          message: `no story mock for ${new URL(request.url).pathname}`,
        },
        { status: 501 },
      ),
    ),
  ];
}
