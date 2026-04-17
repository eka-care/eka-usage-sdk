import {
  METRIC_TYPES,
  PRODUCTS,
  Product,
  STATUSES,
} from "./constants";

export class ValidationError extends Error {
  constructor(msg: string) {
    super(msg);
    this.name = "ValidationError";
  }
}

export function validateUsage(
  product: string,
  metricType: string,
  quantity: number,
  status: string,
): void {
  if (!(PRODUCTS as readonly string[]).includes(product)) {
    throw new ValidationError(
      `invalid product '${product}', allowed=${PRODUCTS.join(",")}`,
    );
  }
  const allowed = METRIC_TYPES[product as Product];
  if (!allowed.includes(metricType)) {
    throw new ValidationError(
      `invalid metric_type '${metricType}' for product '${product}', allowed=${allowed.join(",")}`,
    );
  }
  if (!(STATUSES as readonly string[]).includes(status)) {
    throw new ValidationError(
      `invalid status '${status}', allowed=${STATUSES.join(",")}`,
    );
  }
  if (typeof quantity !== "number" || !isFinite(quantity) || quantity < 0) {
    throw new ValidationError(`quantity must be non-negative finite number`);
  }
}

export function safeStringify(obj: unknown): string {
  try {
    return JSON.stringify(obj ?? {}, (_k, v) => {
      if (typeof v === "bigint") return v.toString();
      return v;
    });
  } catch {
    return "{}";
  }
}
