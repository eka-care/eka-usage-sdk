import type { ProducerLike } from "@eka-care/usage-sdk";

export class MockProducer implements ProducerLike {
  public sent: { topic: string; key: string; value: string }[] = [];
  public failNext = false;
  public disconnected = false;

  async send(args: {
    topic: string;
    messages: { key: string; value: string }[];
  }): Promise<unknown> {
    if (this.failNext) {
      this.failNext = false;
      throw new Error("kafka unavailable");
    }
    for (const m of args.messages) {
      this.sent.push({ topic: args.topic, key: m.key, value: m.value });
    }
    return [];
  }

  async disconnect(): Promise<void> {
    this.disconnected = true;
  }
}
