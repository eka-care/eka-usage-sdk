import { EkaClient } from "../src/client";
import { MockProducer } from "./mockProducer";

function makeClient(onError?: (e: Error, ctx?: Record<string, unknown>) => void) {
  const mp = new MockProducer();
  const client = new EkaClient({
    serviceName: "svc_a",
    kafkaBrokers: "mock",
    producer: mp,
    onError,
  });
  return { client, mp };
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe("EkaClient.record", () => {
  test("valid event", async () => {
    const { client, mp } = makeClient();
    client.record("ws_1", "ekascribe", "transcription_minute", 12.5);
    await client.shutdown();
    expect(mp.sent).toHaveLength(1);
    const evt = JSON.parse(mp.sent[0].value);
    expect(evt.workspace_id).toBe("ws_1");
    expect(evt.product).toBe("ekascribe");
    expect(evt.metric_type).toBe("transcription_minute");
    expect(evt.quantity).toBe(12.5);
    expect(evt.is_billable).toBe(1);
    expect(evt.sdk_language).toBe("typescript");
    expect(mp.sent[0].topic).toBe("eka.usage.events");
    expect(mp.sent[0].key).toBe("ws_1");
  });

  test("invalid product", async () => {
    const errs: Error[] = [];
    const { client, mp } = makeClient((e) => errs.push(e));
    client.record("ws_1", "bogus", "api_call");
    await client.shutdown();
    expect(mp.sent).toHaveLength(0);
    expect(errs).toHaveLength(1);
    expect(errs[0].message).toMatch(/invalid product/);
  });

  test("invalid metric_type", async () => {
    const errs: Error[] = [];
    const { client, mp } = makeClient((e) => errs.push(e));
    client.record("ws_1", "ekascribe", "not_a_metric");
    await client.shutdown();
    expect(errs).toHaveLength(1);
  });

  test("missing workspaceId", async () => {
    const errs: Error[] = [];
    const { client, mp } = makeClient((e) => errs.push(e));
    client.record("", "api", "api_call");
    await client.shutdown();
    expect(mp.sent).toHaveLength(0);
    expect(errs).toHaveLength(1);
    expect(errs[0].message).toMatch(/workspaceId/);
  });

  test("non-serializable metadata falls back safely", async () => {
    const { client, mp } = makeClient();
    const cyc: Record<string, unknown> = {};
    (cyc as any).self = cyc;
    client.record("ws_1", "api", "api_call", 1, "ok", undefined, cyc);
    await client.shutdown();
    expect(mp.sent).toHaveLength(1);
    const evt = JSON.parse(mp.sent[0].value);
    expect(typeof evt.metadata).toBe("string");
  });

  test("error status sets is_billable=0", async () => {
    const { client, mp } = makeClient();
    client.record("ws_1", "api", "api_error", 1, "error");
    await client.shutdown();
    const evt = JSON.parse(mp.sent[0].value);
    expect(evt.is_billable).toBe(0);
  });

  test("different workspaces partitioned separately", async () => {
    const { client, mp } = makeClient();
    client.record("ws_a", "api", "api_call");
    client.record("ws_b", "api", "api_call");
    await client.shutdown();
    expect(mp.sent.map((m) => m.key)).toEqual(["ws_a", "ws_b"]);
  });

  test("unit_cost sent when provided", async () => {
    const { client, mp } = makeClient();
    client.record("ws_1", "ekascribe", "transcription_minute", 5.0, "ok", 0.12);
    await client.shutdown();
    const evt = JSON.parse(mp.sent[0].value);
    expect(evt.unit_cost).toBe(0.12);
  });

  test("unit_cost defaults to null", async () => {
    const { client, mp } = makeClient();
    client.record("ws_1", "api", "api_call");
    await client.shutdown();
    const evt = JSON.parse(mp.sent[0].value);
    expect(evt.unit_cost).toBeNull();
  });

  test("non-blocking", async () => {
    const { client } = makeClient();
    const start = process.hrtime.bigint();
    client.record("ws_1", "api", "api_call");
    const elapsedMs = Number(process.hrtime.bigint() - start) / 1e6;
    await client.shutdown();
    expect(elapsedMs).toBeLessThan(1.0);
  });
});

describe("EkaClient.shutdown", () => {
  test("flushes pending", async () => {
    const { client, mp } = makeClient();
    for (let i = 0; i < 50; i++) client.record("ws_1", "api", "api_call");
    await client.shutdown();
    expect(mp.sent).toHaveLength(50);
  });

  test("idempotent", async () => {
    const { client } = makeClient();
    await client.shutdown();
    await client.shutdown();
  });
});

describe("EkaClient errors", () => {
  test("producer send failure calls onError", async () => {
    const errs: Error[] = [];
    const { client, mp } = makeClient((e) => errs.push(e));
    mp.failNext = true;
    client.record("ws_1", "api", "api_call");
    await sleep(50);
    await client.shutdown();
    expect(errs.length).toBeGreaterThanOrEqual(1);
  });
});

describe("EkaClient config", () => {
  const originalEnv = { ...process.env };

  afterEach(() => {
    process.env = { ...originalEnv };
  });

  test("brokers required when none passed and env unset", () => {
    delete process.env.EKA_KAFKA_BROKERS;
    expect(
      () => new EkaClient({ serviceName: "svc" }),
    ).toThrow(/kafka brokers not configured/);
  });

  test("brokers from env", () => {
    process.env.EKA_KAFKA_BROKERS = "env-broker:9092";
    const c = new EkaClient({ serviceName: "svc" });
    expect((c as any).kafkaBrokers).toEqual(["env-broker:9092"]);
  });

  test("constructor arg overrides env brokers", () => {
    process.env.EKA_KAFKA_BROKERS = "env-broker:9092";
    const c = new EkaClient({
      serviceName: "svc",
      kafkaBrokers: "arg-broker:9092",
    });
    expect((c as any).kafkaBrokers).toEqual(["arg-broker:9092"]);
  });

  test("broker validation skipped when producer provided", () => {
    delete process.env.EKA_KAFKA_BROKERS;
    const c = new EkaClient({
      serviceName: "svc",
      producer: new MockProducer(),
    });
    expect((c as any).kafkaBrokers).toEqual([]);
  });

  test("kafka tuning from env", () => {
    process.env.EKA_KAFKA_BROKERS = "b:9092";
    process.env.EKA_KAFKA_COMPRESSION_TYPE = "zstd";
    process.env.EKA_KAFKA_ACKS = "all";
    process.env.EKA_KAFKA_RETRIES = "10";
    const c = new EkaClient({
      serviceName: "svc",
      producer: new MockProducer(),
    });
    expect((c as any).compressionName).toBe("zstd");
    expect((c as any).acks).toBe(-1);
    expect((c as any).retries).toBe(10);
  });
});
