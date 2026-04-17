import json
import time

import pytest

from eka_usage import EkaClient
from tests.mock_producer import MockProducer


def make_client(**kwargs):
    mp = MockProducer()
    client = EkaClient(
        service_name="svc_a",
        producer=mp,
        **kwargs,
    )
    return client, mp


def test_record_valid_event():
    client, mp = make_client()
    client.record("ws_1", "ekascribe", "transcription_minute", quantity=12.5)
    client.shutdown()
    assert len(mp.produced) == 1
    evt = json.loads(mp.produced[0]["value"].decode())
    assert evt["workspace_id"] == "ws_1"
    assert evt["product"] == "ekascribe"
    assert evt["metric_type"] == "transcription_minute"
    assert evt["quantity"] == 12.5
    assert evt["is_billable"] == 1
    assert evt["sdk_language"] == "python"
    assert "ts" in evt and evt["ts"].endswith("Z")
    assert mp.produced[0]["topic"] == "eka.usage.events"
    assert mp.produced[0]["key"] == b"ws_1"


def test_record_invalid_product():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    client.record("ws_1", "bogus_product", "api_call")
    client.shutdown()
    assert len(mp.produced) == 0
    assert len(errors) == 1
    assert "invalid product" in str(errors[0][0])


def test_record_invalid_metric_type():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    client.record("ws_1", "ekascribe", "not_a_metric")
    client.shutdown()
    assert len(mp.produced) == 0
    assert len(errors) == 1


def test_record_missing_workspace_id():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    client.record("", "api", "api_call")
    client.shutdown()
    assert len(mp.produced) == 0
    assert len(errors) == 1
    assert "workspace_id" in str(errors[0][0])


def test_record_non_serializable():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))

    class Weird:
        pass

    client.record("ws_1", "api", "api_call", metadata={"obj": Weird()})
    client.shutdown()
    assert len(mp.produced) == 1
    evt = json.loads(mp.produced[0]["value"].decode())
    assert isinstance(evt["metadata"], str)


def test_record_error_status_sets_is_billable_zero():
    client, mp = make_client()
    client.record("ws_1", "api", "api_error", status="error")
    client.shutdown()
    evt = json.loads(mp.produced[0]["value"].decode())
    assert evt["is_billable"] == 0
    assert evt["status"] == "error"


def test_record_different_workspaces_partitioned_separately():
    client, mp = make_client()
    client.record("ws_a", "api", "api_call")
    client.record("ws_b", "api", "api_call")
    client.shutdown()
    assert [m["key"] for m in mp.produced] == [b"ws_a", b"ws_b"]


def test_log_valid_event():
    client, mp = make_client()
    client.log("ws_1", "error", "db timeout", code="DB_TIMEOUT", metadata={"query_ms": 5200})
    client.shutdown()
    assert len(mp.produced) == 1
    evt = json.loads(mp.produced[0]["value"].decode())
    assert evt["workspace_id"] == "ws_1"
    assert evt["level"] == "error"
    assert evt["message"] == "db timeout"
    assert evt["code"] == "DB_TIMEOUT"
    assert mp.produced[0]["topic"] == "eka.service.logs"
    assert mp.produced[0]["key"] == b"ws_1"


def test_log_empty_message():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    client.log("ws_1", "error", "")
    client.shutdown()
    assert len(mp.produced) == 0
    assert len(errors) == 1


def test_log_invalid_level():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    client.log("ws_1", "info", "hi")
    client.shutdown()
    assert len(errors) == 1


def test_log_missing_workspace_id():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    client.log("", "error", "boom")
    client.shutdown()
    assert len(mp.produced) == 0
    assert len(errors) == 1


def test_shutdown_flushes_pending():
    client, mp = make_client()
    for i in range(50):
        client.record("ws_1", "api", "api_call")
    client.shutdown()
    assert len(mp.produced) == 50


def test_shutdown_idempotent():
    client, mp = make_client()
    client.shutdown()
    client.shutdown()


def test_record_non_blocking():
    client, mp = make_client()
    start = time.perf_counter()
    client.record("ws_1", "api", "api_call")
    elapsed_ms = (time.perf_counter() - start) * 1000
    client.shutdown()
    assert elapsed_ms < 1.0, f"record() took {elapsed_ms:.3f}ms, must be <1ms"


def test_context_manager():
    mp = MockProducer()
    with EkaClient("svc", producer=mp) as client:
        client.record("ws", "api", "api_call")
    assert len(mp.produced) == 1


def test_buffer_error_calls_on_error():
    errors = []
    client, mp = make_client(on_error=lambda e, ctx: errors.append((e, ctx)))
    mp.fail_next = True
    client.record("ws_1", "api", "api_call")
    client.shutdown()
    assert len(errors) == 1
    assert errors[0][1]["reason"] == "local_queue_full"


def test_brokers_required_when_no_producer(monkeypatch):
    monkeypatch.delenv("EKA_KAFKA_BROKERS", raising=False)
    with pytest.raises(ValueError, match="kafka brokers not configured"):
        EkaClient(service_name="svc")


def test_brokers_from_env(monkeypatch):
    monkeypatch.setenv("EKA_KAFKA_BROKERS", "env-broker:9092")
    captured = {}

    class FakeProducer:
        def __init__(self, cfg):
            captured["cfg"] = cfg
        def poll(self, *a, **kw): return 0
        def flush(self, *a, **kw): return 0

    import eka_usage.client as client_mod
    monkeypatch.setattr(client_mod, "__name__", client_mod.__name__)

    import confluent_kafka
    monkeypatch.setattr(confluent_kafka, "Producer", FakeProducer)

    c = EkaClient(service_name="svc")
    assert captured["cfg"]["bootstrap.servers"] == "env-broker:9092"
    c.shutdown()


def test_kafka_config_from_env(monkeypatch):
    monkeypatch.setenv("EKA_KAFKA_BROKERS", "b:9092")
    monkeypatch.setenv("EKA_KAFKA_LINGER_MS", "200")
    monkeypatch.setenv("EKA_KAFKA_BATCH_SIZE", "131072")
    monkeypatch.setenv("EKA_KAFKA_COMPRESSION_TYPE", "zstd")
    monkeypatch.setenv("EKA_KAFKA_ACKS", "all")
    monkeypatch.setenv("EKA_KAFKA_RETRIES", "10")
    captured = {}

    class FakeProducer:
        def __init__(self, cfg):
            captured["cfg"] = cfg
        def poll(self, *a, **kw): return 0
        def flush(self, *a, **kw): return 0

    import confluent_kafka
    monkeypatch.setattr(confluent_kafka, "Producer", FakeProducer)

    c = EkaClient(service_name="svc")
    cfg = captured["cfg"]
    assert cfg["linger.ms"] == 200
    assert cfg["batch.size"] == 131072
    assert cfg["compression.type"] == "zstd"
    assert cfg["acks"] == "all"
    assert cfg["retries"] == 10
    c.shutdown()


def test_explicit_args_override_env(monkeypatch):
    monkeypatch.setenv("EKA_KAFKA_BROKERS", "env-broker:9092")
    monkeypatch.setenv("EKA_KAFKA_LINGER_MS", "200")
    captured = {}

    class FakeProducer:
        def __init__(self, cfg):
            captured["cfg"] = cfg
        def poll(self, *a, **kw): return 0
        def flush(self, *a, **kw): return 0

    import confluent_kafka
    monkeypatch.setattr(confluent_kafka, "Producer", FakeProducer)

    c = EkaClient(
        service_name="svc",
        kafka_brokers="arg-broker:9092",
        kafka_config={"linger.ms": 999},
    )
    assert captured["cfg"]["bootstrap.servers"] == "arg-broker:9092"
    assert captured["cfg"]["linger.ms"] == 999
    c.shutdown()
