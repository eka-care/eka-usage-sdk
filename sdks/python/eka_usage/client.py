import json
import os
import socket
import threading
import time
from datetime import datetime, timezone
from typing import Any, Callable, Dict, Optional

from .constants import (
    DEFAULT_ACKS,
    DEFAULT_BATCH_SIZE,
    DEFAULT_COMPRESSION_TYPE,
    DEFAULT_LINGER_MS,
    DEFAULT_RETRIES,
    ENV_KAFKA_ACKS,
    ENV_KAFKA_BATCH_SIZE,
    ENV_KAFKA_BROKERS,
    ENV_KAFKA_COMPRESSION_TYPE,
    ENV_KAFKA_LINGER_MS,
    ENV_KAFKA_RETRIES,
    LOGS_TOPIC,
    SDK_LANGUAGE,
    SDK_VERSION,
    USAGE_TOPIC,
)
from .validation import ValidationError, validate_log, validate_usage


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def _env_int(name: str, default: int) -> int:
    v = os.environ.get(name)
    return int(v) if v else default


def _kafka_config_from_env(service_name: str) -> Dict[str, Any]:
    return {
        "linger.ms": _env_int(ENV_KAFKA_LINGER_MS, DEFAULT_LINGER_MS),
        "batch.size": _env_int(ENV_KAFKA_BATCH_SIZE, DEFAULT_BATCH_SIZE),
        "compression.type": os.environ.get(ENV_KAFKA_COMPRESSION_TYPE, DEFAULT_COMPRESSION_TYPE),
        "acks": os.environ.get(ENV_KAFKA_ACKS, DEFAULT_ACKS),
        "retries": _env_int(ENV_KAFKA_RETRIES, DEFAULT_RETRIES),
        "enable.idempotence": False,
        "client.id": f"eka-usage-sdk-py/{service_name}",
    }


class EkaClient:
    def __init__(
        self,
        service_name: str,
        kafka_brokers: Optional[str] = None,
        kafka_config: Optional[Dict[str, Any]] = None,
        on_error: Optional[Callable[[Exception, Optional[Dict[str, Any]]], None]] = None,
        debug: bool = False,
        producer: Any = None,
    ) -> None:
        if not service_name:
            raise ValueError("service_name required")

        self.service_name = service_name
        self.on_error = on_error
        self.debug = debug
        self.hostname = socket.gethostname()

        self._closed = False
        self._lock = threading.Lock()

        if producer is not None:
            self._producer = producer
        else:
            from confluent_kafka import Producer

            brokers = kafka_brokers or os.environ.get(ENV_KAFKA_BROKERS, "")
            if not brokers:
                raise ValueError(
                    f"kafka brokers not configured (pass kafka_brokers or set {ENV_KAFKA_BROKERS})"
                )
            cfg = _kafka_config_from_env(service_name)
            cfg["bootstrap.servers"] = brokers
            if kafka_config:
                cfg.update(kafka_config)
            self._producer = Producer(cfg)

        self._poll_thread = threading.Thread(
            target=self._poll_loop, name="eka-usage-sdk-poll", daemon=True
        )
        self._poll_thread.start()

    def record(
        self,
        workspace_id: str,
        product: str,
        metric_type: str,
        quantity: float = 1.0,
        status: str = "ok",
        metadata: Optional[Dict[str, Any]] = None,
    ) -> None:
        try:
            if not workspace_id:
                raise ValidationError("workspace_id required")
            validate_usage(product, metric_type, quantity, status)
            meta_json = json.dumps(metadata or {}, default=str)
            event = {
                "workspace_id": workspace_id,
                "service_name": self.service_name,
                "product": product,
                "metric_type": metric_type,
                "quantity": float(quantity),
                "status": status,
                "is_billable": 1 if status == "ok" else 0,
                "metadata": meta_json,
                "sdk_language": SDK_LANGUAGE,
                "sdk_version": SDK_VERSION,
                "hostname": self.hostname,
                "ts": _utc_now_iso(),
            }
            self._produce(USAGE_TOPIC, workspace_id, event)
        except Exception as e:
            self._handle_error(e, {"workspace_id": workspace_id, "product": product, "metric_type": metric_type})

    def log(
        self,
        workspace_id: str,
        level: str,
        message: str,
        code: Optional[str] = None,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> None:
        try:
            if not workspace_id:
                raise ValidationError("workspace_id required")
            validate_log(level, message)
            meta_json = json.dumps(metadata or {}, default=str)
            event = {
                "workspace_id": workspace_id,
                "service_name": self.service_name,
                "level": level,
                "message": message,
                "code": code,
                "metadata": meta_json,
                "sdk_language": SDK_LANGUAGE,
                "sdk_version": SDK_VERSION,
                "hostname": self.hostname,
                "ts": _utc_now_iso(),
            }
            self._produce(LOGS_TOPIC, workspace_id, event)
        except Exception as e:
            self._handle_error(e, {"workspace_id": workspace_id, "level": level})

    def _produce(self, topic: str, partition_key: str, event: Dict[str, Any]) -> None:
        payload = json.dumps(event, separators=(",", ":")).encode("utf-8")
        try:
            self._producer.produce(
                topic=topic,
                key=partition_key.encode("utf-8"),
                value=payload,
                on_delivery=self._on_delivery,
            )
        except BufferError as e:
            self._handle_error(e, {"topic": topic, "reason": "local_queue_full"})
        except Exception as e:
            self._handle_error(e, {"topic": topic})

    def _on_delivery(self, err: Any, msg: Any) -> None:
        if err is not None:
            self._handle_error(Exception(str(err)), {"topic": getattr(msg, "topic", lambda: None)()})
        elif self.debug:
            try:
                print(f"[eka-usage-sdk] delivered to {msg.topic()}[{msg.partition()}]@{msg.offset()}")
            except Exception:
                pass

    def _poll_loop(self) -> None:
        while not self._closed:
            try:
                self._producer.poll(0.1)
            except Exception:
                time.sleep(0.1)

    def _handle_error(self, exc: Exception, ctx: Optional[Dict[str, Any]]) -> None:
        if self.debug:
            print(f"[eka-usage-sdk] error: {exc} ctx={ctx}")
        if self.on_error is not None:
            try:
                self.on_error(exc, ctx)
            except Exception:
                pass

    def shutdown(self, timeout: float = 10.0) -> None:
        with self._lock:
            if self._closed:
                return
            self._closed = True
        try:
            self._producer.flush(timeout)
        except Exception as e:
            self._handle_error(e, {"phase": "shutdown_flush"})

    def __enter__(self) -> "EkaClient":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.shutdown()
