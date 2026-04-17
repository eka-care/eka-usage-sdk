import threading
from typing import Any, Callable, Dict, List, Optional


class MockProducer:
    def __init__(self) -> None:
        self.produced: List[Dict[str, Any]] = []
        self._pending: List[Callable[[], None]] = []
        self._lock = threading.Lock()
        self.fail_next = False

    def produce(
        self,
        topic: str,
        key: bytes = b"",
        value: bytes = b"",
        on_delivery: Optional[Callable[[Any, Any], None]] = None,
    ) -> None:
        if self.fail_next:
            self.fail_next = False
            raise BufferError("queue full")
        with self._lock:
            self.produced.append(
                {"topic": topic, "key": key, "value": value}
            )
            if on_delivery is not None:
                class _Msg:
                    def __init__(self, t):
                        self._t = t
                    def topic(self):
                        return self._t
                    def partition(self):
                        return 0
                    def offset(self):
                        return len(self._msg_list) if False else 0
                    _msg_list = []
                self._pending.append(lambda cb=on_delivery, t=topic: cb(None, _Msg(t)))

    def poll(self, timeout: float = 0.0) -> int:
        with self._lock:
            callbacks, self._pending = self._pending, []
        for cb in callbacks:
            try:
                cb()
            except Exception:
                pass
        return len(callbacks)

    def flush(self, timeout: float = 10.0) -> int:
        self.poll(0)
        return 0
