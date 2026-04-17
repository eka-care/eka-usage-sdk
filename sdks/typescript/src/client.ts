import * as os from "os";
import {
  ENV_KAFKA_ACKS,
  ENV_KAFKA_BROKERS,
  ENV_KAFKA_COMPRESSION_TYPE,
  ENV_KAFKA_RETRIES,
  SDK_LANGUAGE,
  SDK_VERSION,
  Status,
  USAGE_TOPIC,
} from "./constants";
import { safeStringify, validateUsage } from "./validation";

export interface ProducerLike {
  send(args: {
    topic: string;
    compression?: number;
    acks?: number;
    messages: { key: string; value: string }[];
  }): Promise<unknown>;
  connect?(): Promise<void>;
  disconnect?(): Promise<void>;
}

export type OnError = (err: Error, ctx?: Record<string, unknown>) => void;

export interface EkaClientOptions {
  serviceName: string;
  kafkaBrokers?: string | string[];
  kafkaConfig?: Record<string, unknown>;
  onError?: OnError;
  debug?: boolean;
  producer?: ProducerLike;
  flushIntervalMs?: number;
  maxQueueSize?: number;
  compression?: "lz4" | "gzip" | "snappy" | "zstd" | "none";
  acks?: -1 | 0 | 1;
  retries?: number;
}

type QueuedMessage = {
  topic: string;
  key: string;
  value: string;
};

export class EkaClient {
  readonly serviceName: string;
  private readonly hostname: string;
  private readonly onError?: OnError;
  private readonly debug: boolean;

  private producer: ProducerLike | null;
  private providedProducer: boolean;
  private kafkaBrokers: string[];
  private kafkaConfig: Record<string, unknown>;
  private compressionName: string;
  private acks: number;
  private retries: number;

  private queue: QueuedMessage[] = [];
  private maxQueueSize: number;
  private flushIntervalMs: number;
  private flushTimer: NodeJS.Timeout | null = null;
  private connected = false;
  private closed = false;
  private activeFlush: Promise<void> | null = null;

  constructor(opts: EkaClientOptions) {
    if (!opts.serviceName) throw new Error("serviceName required");

    this.serviceName = opts.serviceName;
    this.hostname = os.hostname();
    this.onError = opts.onError;
    this.debug = !!opts.debug;

    this.providedProducer = !!opts.producer;
    this.producer = opts.producer ?? null;

    if (this.providedProducer) {
      this.kafkaBrokers = [];
      this.connected = true;
    } else {
      const brokersInput = opts.kafkaBrokers ?? process.env[ENV_KAFKA_BROKERS];
      if (!brokersInput) {
        throw new Error(
          `kafka brokers not configured (pass kafkaBrokers or set ${ENV_KAFKA_BROKERS})`,
        );
      }
      this.kafkaBrokers = Array.isArray(brokersInput)
        ? brokersInput
        : brokersInput.split(",").map((s) => s.trim()).filter(Boolean);
    }

    this.kafkaConfig = opts.kafkaConfig || {};
    this.maxQueueSize = opts.maxQueueSize ?? 10000;
    this.flushIntervalMs = opts.flushIntervalMs ?? 200;

    this.compressionName = (
      opts.compression ?? process.env[ENV_KAFKA_COMPRESSION_TYPE] ?? "lz4"
    ).toLowerCase();
    this.acks = opts.acks ?? parseAcks(process.env[ENV_KAFKA_ACKS]) ?? 1;
    const retriesEnv = process.env[ENV_KAFKA_RETRIES];
    this.retries = opts.retries ?? (retriesEnv ? parseInt(retriesEnv, 10) : 5);
  }

  async connect(): Promise<void> {
    if (this.connected) return;
    const { Kafka, CompressionTypes } = await import("kafkajs");
    const compression = resolveCompression(CompressionTypes, this.compressionName);
    const kafka = new Kafka({
      clientId: `eka-usage-sdk-ts/${this.serviceName}`,
      brokers: this.kafkaBrokers,
      retry: { retries: this.retries },
      ...(this.kafkaConfig as object),
    });
    const producer = kafka.producer({ allowAutoTopicCreation: false });
    await producer.connect();
    const acks = this.acks;
    this.producer = {
      send: (args) =>
        producer.send({
          topic: args.topic,
          compression,
          acks,
          messages: args.messages,
        }),
      disconnect: () => producer.disconnect(),
    };
    this.connected = true;
    this.startFlushTimer();
  }

  private startFlushTimer(): void {
    if (this.flushTimer !== null) return;
    this.flushTimer = setInterval(
      () => void this.flushOnce(),
      this.flushIntervalMs,
    );
    if (this.flushTimer.unref) this.flushTimer.unref();
  }

  record(
    workspaceId: string,
    product: string,
    metricType: string,
    quantity: number = 1.0,
    status: Status = "ok",
    unitCost?: number | null,
    metadata: Record<string, unknown> = {},
  ): void {
    try {
      if (!workspaceId) throw new Error("workspaceId required");
      validateUsage(product, metricType, quantity, status);
      const event = {
        workspace_id: workspaceId,
        service_name: this.serviceName,
        product,
        metric_type: metricType,
        quantity: Number(quantity),
        unit_cost: unitCost != null ? Number(unitCost) : null,
        status,
        is_billable: status === "ok" ? 1 : 0,
        metadata: safeStringify(metadata),
        sdk_language: SDK_LANGUAGE,
        sdk_version: SDK_VERSION,
        hostname: this.hostname,
        ts: nowIso(),
      };
      this.enqueue(USAGE_TOPIC, workspaceId, event);
    } catch (e) {
      this.handleError(e, { workspaceId, product, metricType });
    }
  }

  private enqueue(
    topic: string,
    partitionKey: string,
    event: Record<string, unknown>,
  ): void {
    if (this.closed) {
      this.handleError(new Error("client closed"), { topic });
      return;
    }
    if (this.queue.length >= this.maxQueueSize) {
      this.handleError(new Error("queue full"), {
        topic,
        reason: "local_queue_full",
      });
      return;
    }
    this.queue.push({
      topic,
      key: partitionKey,
      value: JSON.stringify(event),
    });
    if (this.providedProducer) void this.flushOnce();
  }

  private flushOnce(): Promise<void> {
    if (this.activeFlush) return this.activeFlush;
    if (this.queue.length === 0 || !this.producer) return Promise.resolve();
    const batch = this.queue;
    this.queue = [];
    const producer = this.producer;
    this.activeFlush = (async () => {
      try {
        const byTopic = new Map<string, { key: string; value: string }[]>();
        for (const m of batch) {
          const arr = byTopic.get(m.topic) ?? [];
          arr.push({ key: m.key, value: m.value });
          byTopic.set(m.topic, arr);
        }
        for (const [topic, messages] of byTopic) {
          await producer.send({ topic, messages });
          if (this.debug) {
            console.log(
              `[eka-usage-sdk] sent ${messages.length} to ${topic}`,
            );
          }
        }
      } catch (e) {
        this.handleError(e, { phase: "flush" });
      } finally {
        this.activeFlush = null;
      }
    })();
    return this.activeFlush;
  }

  private handleError(err: unknown, ctx?: Record<string, unknown>): void {
    const e = err instanceof Error ? err : new Error(String(err));
    if (this.debug) console.error("[eka-usage-sdk]", e.message, ctx);
    if (this.onError) {
      try {
        this.onError(e, ctx);
      } catch {
        /* swallow */
      }
    }
  }

  async shutdown(): Promise<void> {
    if (this.closed) return;
    this.closed = true;
    if (this.flushTimer) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }
    if (this.activeFlush) await this.activeFlush;
    await this.flushOnce();
    if (!this.providedProducer && this.producer?.disconnect) {
      try {
        await this.producer.disconnect();
      } catch (e) {
        this.handleError(e, { phase: "disconnect" });
      }
    }
  }
}

function nowIso(): string {
  return new Date().toISOString();
}

function parseAcks(v: string | undefined): number | undefined {
  if (v === undefined || v === "") return undefined;
  if (v === "all") return -1;
  const n = parseInt(v, 10);
  return Number.isNaN(n) ? undefined : n;
}

function resolveCompression(
  ct: { GZIP: number; Snappy: number; LZ4: number; ZSTD: number; None: number },
  name: string,
): number {
  switch (name) {
    case "gzip":
      return ct.GZIP;
    case "snappy":
      return ct.Snappy;
    case "zstd":
      return ct.ZSTD;
    case "none":
      return ct.None;
    case "lz4":
    default:
      return ct.LZ4;
  }
}
